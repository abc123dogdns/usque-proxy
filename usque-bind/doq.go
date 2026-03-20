package usquebind

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
)

// doqProxy resolves DNS queries over QUIC (RFC 9250).
type doqProxy struct {
	addr      string
	protector VpnProtector
	cache     *lruCache
	refreshing sync.Map

	mu      sync.Mutex
	conn    *quic.Conn
	udpConn *net.UDPConn

	makeConn func() (*quic.Conn, *net.UDPConn, error)
}

func newDoqProxy(addr string, protector VpnProtector) *doqProxy {
	// Default port 853 per RFC 9250
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
		port = "853"
	}
	if port == "" {
		port = "853"
	}

	d := &doqProxy{
		addr:      net.JoinHostPort(host, port),
		protector: protector,
		cache:     newLRUCache(1024),
	}

	d.makeConn = func() (*quic.Conn, *net.UDPConn, error) {
		udpAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(host, port))
		if err != nil {
			return nil, nil, fmt.Errorf("resolve DoQ addr: %w", err)
		}

		localAddr := &net.UDPAddr{IP: net.IPv4zero, Port: 0}
		if udpAddr.IP.To4() == nil {
			localAddr = &net.UDPAddr{IP: net.IPv6zero, Port: 0}
		}

		udpConn, err := net.ListenUDP("udp", localAddr)
		if err != nil {
			return nil, nil, fmt.Errorf("DoQ UDP socket: %w", err)
		}

		// Protect socket from VPN routing
		rawConn, err := udpConn.SyscallConn()
		if err != nil {
			udpConn.Close()
			return nil, nil, fmt.Errorf("DoQ raw conn: %w", err)
		}
		var protectErr error
		rawConn.Control(func(fd uintptr) {
			if !protector.ProtectFd(int(fd)) {
				protectErr = errors.New("VPN protect() failed for DoQ socket")
			}
		})
		if protectErr != nil {
			udpConn.Close()
			return nil, nil, protectErr
		}

		tlsCfg := &tls.Config{
			NextProtos:         []string{"doq"},
			MinVersion:         tls.VersionTLS13,
			ClientSessionCache: globalTLSSessionCache,
			ServerName:         host,
		}
		quicCfg := &quic.Config{
			MaxIdleTimeout:  90 * time.Second,
			KeepAlivePeriod: 30 * time.Second,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		conn, err := quic.Dial(ctx, udpConn, udpAddr, tlsCfg, quicCfg)
		if err != nil {
			udpConn.Close()
			return nil, nil, fmt.Errorf("DoQ QUIC dial: %w", err)
		}

		return conn, udpConn, nil
	}

	return d
}

// getConn returns the current QUIC connection, reconnecting if needed.
func (d *doqProxy) getConn() (*quic.Conn, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.conn != nil {
		select {
		case <-d.conn.Context().Done():
			// Connection dead, reconnect
			d.conn = nil
			if d.udpConn != nil {
				d.udpConn.Close()
				d.udpConn = nil
			}
		default:
			return d.conn, nil
		}
	}

	conn, udpConn, err := d.makeConn()
	if err != nil {
		return nil, err
	}
	d.conn = conn
	d.udpConn = udpConn
	return conn, nil
}

// fetchFromServer sends a DNS query over QUIC per RFC 9250.
func (d *doqProxy) fetchFromServer(query []byte) ([]byte, error) {
	padded := padQuery(query)
	resp, err := d.doFetch(padded)
	if err != nil {
		// Retry once on stream error (reconnect + retry)
		if isDoqRetryable(err) {
			d.resetConn()
			resp, err = d.doFetch(padded)
		}
	}
	return resp, err
}

func (d *doqProxy) doFetch(query []byte) ([]byte, error) {
	conn, err := d.getConn()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		return nil, fmt.Errorf("DoQ open stream: %w", err)
	}

	// Write 2-byte length prefix + query (RFC 9250 §4.2)
	lenBuf := make([]byte, 2)
	binary.BigEndian.PutUint16(lenBuf, uint16(len(query)))
	if _, err := stream.Write(lenBuf); err != nil {
		stream.CancelRead(0)
		stream.CancelWrite(0)
		return nil, fmt.Errorf("DoQ write len: %w", err)
	}
	if _, err := stream.Write(query); err != nil {
		stream.CancelRead(0)
		stream.CancelWrite(0)
		return nil, fmt.Errorf("DoQ write query: %w", err)
	}
	// Send FIN to indicate we're done writing
	stream.Close()

	// Read 2-byte length prefix + response
	respLenBuf := make([]byte, 2)
	if _, err := io.ReadFull(stream, respLenBuf); err != nil {
		return nil, fmt.Errorf("DoQ read resp len: %w", err)
	}
	respLen := binary.BigEndian.Uint16(respLenBuf)
	if respLen < 12 || respLen > 4096 {
		return nil, fmt.Errorf("DoQ invalid response length: %d", respLen)
	}

	resp := make([]byte, respLen)
	if _, err := io.ReadFull(stream, resp); err != nil {
		return nil, fmt.Errorf("DoQ read resp: %w", err)
	}

	return resp, nil
}

// resolve sends a DNS query via DoQ with caching (same pattern as dohProxy.resolve).
func (d *doqProxy) resolve(query []byte) ([]byte, error) {
	if len(query) < 12 {
		return nil, errors.New("DNS query too short")
	}

	// Save original transaction ID and zero it for cache key
	origID := [2]byte{query[0], query[1]}
	query[0], query[1] = 0, 0

	cacheKey := string(query)
	if entry, ok := d.cache.get(cacheKey); ok {
		now := time.Now()
		if now.Before(entry.expiry) {
			resp := make([]byte, len(entry.response))
			copy(resp, entry.response)
			resp[0], resp[1] = origID[0], origID[1]
			query[0], query[1] = origID[0], origID[1]
			// Proactive refresh at 75% TTL
			if entry.originalTTL > 0 {
				refreshAt := entry.expiry.Add(-entry.originalTTL / 4)
				if now.After(refreshAt) {
					if _, loaded := d.refreshing.LoadOrStore(cacheKey, struct{}{}); !loaded {
						queryCopy := make([]byte, len(query))
						copy(queryCopy, query)
						queryCopy[0], queryCopy[1] = 0, 0
						go d.backgroundRefresh(cacheKey, queryCopy)
					}
				}
			}
			return resp, nil
		}
		if now.Before(entry.staleDeadline) {
			resp := make([]byte, len(entry.response))
			copy(resp, entry.response)
			resp[0], resp[1] = origID[0], origID[1]
			query[0], query[1] = origID[0], origID[1]
			if _, loaded := d.refreshing.LoadOrStore(cacheKey, struct{}{}); !loaded {
				queryCopy := make([]byte, len(query))
				copy(queryCopy, query)
				queryCopy[0], queryCopy[1] = 0, 0
				go d.backgroundRefresh(cacheKey, queryCopy)
			}
			return resp, nil
		}
		d.cache.delete(cacheKey)
	}

	body, err := d.fetchFromServer(query)
	if err != nil {
		query[0], query[1] = origID[0], origID[1]
		return nil, err
	}

	cacheCopy := make([]byte, len(body))
	copy(cacheCopy, body)
	cacheCopy[0], cacheCopy[1] = 0, 0
	ttl := extractMinTTL(body)
	now := time.Now()
	d.cache.put(cacheKey, &cacheEntry{
		response:      cacheCopy,
		expiry:        now.Add(ttl),
		staleDeadline: now.Add(ttl + staleGracePeriod),
		originalTTL:   ttl,
	})

	body[0], body[1] = origID[0], origID[1]
	query[0], query[1] = origID[0], origID[1]
	return body, nil
}

func (d *doqProxy) backgroundRefresh(cacheKey string, query []byte) {
	defer d.refreshing.Delete(cacheKey)

	body, err := d.fetchFromServer(query)
	if err != nil {
		log.Printf("DoQ background refresh error: %v", err)
		return
	}

	cacheCopy := make([]byte, len(body))
	copy(cacheCopy, body)
	cacheCopy[0], cacheCopy[1] = 0, 0
	ttl := extractMinTTL(body)
	now := time.Now()
	d.cache.put(cacheKey, &cacheEntry{
		response:      cacheCopy,
		expiry:        now.Add(ttl),
		staleDeadline: now.Add(ttl + staleGracePeriod),
		originalTTL:   ttl,
	})
}

// resetConn closes the QUIC connection and UDP socket.
func (d *doqProxy) resetConn() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.conn != nil {
		d.conn.CloseWithError(0, "reset")
		d.conn = nil
	}
	if d.udpConn != nil {
		d.udpConn.Close()
		d.udpConn = nil
	}
}

// warmConnection pre-establishes the QUIC connection.
func (d *doqProxy) warmConnection() {
	go func() {
		_, _ = d.fetchFromServer(warmupQuery)
	}()
}

// isDoqRetryable returns true for transient QUIC stream errors worth retrying.
func isDoqRetryable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "stream") ||
		strings.Contains(msg, "connection") ||
		strings.Contains(msg, "timeout") ||
		errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrUnexpectedEOF)
}

// newDoqDnsInterceptor creates a dnsInterceptor that resolves all DNS via DoQ.
func newDoqDnsInterceptor(ctx context.Context, doqAddr string, protector VpnProtector) *dnsInterceptor {
	if doqAddr == "" {
		return nil
	}

	doq := newDoqProxy(doqAddr, protector)
	doq.warmConnection()

	d := &dnsInterceptor{
		resolver: doq.resolve,
		reqCh:    make(chan dnsRequest, 256),
		resetFunc: func() {
			doq.resetConn()
			doq.warmConnection()
		},
	}

	const numWorkers = 4
	for i := 0; i < numWorkers; i++ {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case req, ok := <-d.reqCh:
					if !ok {
						return
					}
					d.handleInterceptedDNS(req)
				}
			}
		}()
	}

	return d
}
