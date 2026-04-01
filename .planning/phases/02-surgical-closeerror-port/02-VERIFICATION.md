---
phase: 02-surgical-closeerror-port
verified: 2026-04-01T11:00:00Z
status: passed
score: 7/7 must-haves verified
---

# Phase 2: Surgical CloseError Port Verification Report

**Phase Goal:** Forwarding loops detect MASQUE session death via CloseError and trigger immediate reconnection with constant delay
**Verified:** 2026-04-01T11:00:00Z
**Status:** passed
**Re-verification:** No -- initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | forwardUp sends fatal error on CloseError from ipConn.WritePacket | VERIFIED | bind.go:723 `errors.As(err, new(*connectip.CloseError))` sends to errChan and returns |
| 2 | forwardUp sends fatal error on CloseError from ICMP device.WritePacket | VERIFIED | bind.go:732 same pattern, message "connection closed while writing ICMP to TUN device" |
| 3 | forwardDown sends fatal error on CloseError from ipConn.ReadPacket | VERIFIED | bind.go:748 same pattern, message "connection closed while reading from IP connection" |
| 4 | Non-CloseError errors in forwardUp and forwardDown are logged and continued past | VERIFIED | 3 `log.Printf` calls with "continuing..." at lines 727, 736, 752; all followed by `continue` |
| 5 | TUN device read/write errors remain unconditionally fatal | VERIFIED | bind.go:694 (forwardUp ReadPacket) and bind.go:760 (forwardDown WritePacket) both send directly to errChan with no CloseError check |
| 6 | Reconnection uses constant 1-second delay, never exponential backoff | VERIFIED | bind.go:373 `reconnectDelay = 1 * time.Second`; 5 call sites all use `sleepCtx(ctx, reconnectDelay)` at lines 436, 447, 475, 483, 524 |
| 7 | nextBackoff function and all adaptive/backoff code removed | VERIFIED | grep for `nextBackoff`, `networkHint`, `NetworkType`, `networkGraceAttempts`, `isNetworkReconnect`, `networkTriggered`, `mrand`, `minBackoff`, `maxBackoff` all return 0 matches |

**Score:** 7/7 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `usque-bind/bind.go` | CloseError-classified forwarding loops, constant reconnect delay, fixed QUIC config | VERIFIED | 835 lines; contains all 3 CloseError checks, constant 1s delay, fixed QUIC config (30s/120s/1280/PMTU-disabled), unconditional DNS reset |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| forwardUp | errChan | CloseError on ipConn.WritePacket | WIRED | Line 723-725: `errors.As` check, `errChan <- fmt.Errorf(...)`, `return` |
| forwardUp | errChan | CloseError on ICMP device.WritePacket | WIRED | Line 732-734: same pattern |
| forwardDown | errChan | CloseError on ipConn.ReadPacket | WIRED | Line 748-750: same pattern |
| maintainTunnel | sleepCtx | constant reconnectDelay | WIRED | All 5 sleep sites use `reconnectDelay` constant (lines 436, 447, 475, 483, 524) |
| maintainTunnel | dns.resetConnections | unconditional after every tunnel loss | WIRED | Line 520-522: `if dns != nil { dns.resetConnections() }` -- no `isNetworkReconnect` guard |

### Data-Flow Trace (Level 4)

Not applicable -- this phase modifies error handling and reconnect logic in Go goroutines, not data-rendering components.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Go code compiles | `GOTOOLCHAIN=go1.24.2 go vet ./...` in usque-bind/ | Clean exit, no output | PASS |
| No dead code references | grep for 12 removed identifiers | 0 matches | PASS |
| Exactly 3 CloseError checks | `grep -c 'errors.As(err, new(*connectip.CloseError))'` | 3 | PASS |
| Exactly 3 non-fatal log lines | `grep -c 'continuing...'` | 3 | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| CERR-01 | 02-01 | forwardUp detects CloseError on ipConn.WritePacket | SATISFIED | bind.go:723-725 |
| CERR-02 | 02-01 | forwardUp detects CloseError on ICMP device.WritePacket | SATISFIED | bind.go:732-734 |
| CERR-03 | 02-01 | forwardDown detects CloseError on ipConn.ReadPacket | SATISFIED | bind.go:748-750 |
| CERR-04 | 02-01, 02-02 | Non-CloseError errors logged but do not trigger reconnection | SATISFIED | Lines 727, 736, 752: log + continue |
| RDLY-01 | 02-02 | Constant 1-second reconnect delay replaces exponential backoff | SATISFIED | bind.go:373 constant; 0 matches for nextBackoff/minBackoff/maxBackoff |

All 5 requirement IDs accounted for. No orphaned requirements.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (none) | - | - | - | - |

No TODOs, FIXMEs, placeholders, empty implementations, or stub patterns found in bind.go.

### Human Verification Required

### 1. On-Device Tunnel Reliability

**Test:** Build AAR (`bash build-usque.sh`), install on device, connect VPN, leave running for 2+ hours
**Expected:** When server expires the MASQUE session, tunnel detects death within seconds and reconnects with 1s delay (visible in logcat as "connection closed while..." followed by reconnection)
**Why human:** Requires physical Android device, Cloudflare WARP account, and multi-hour observation of tunnel behavior

### 2. Transient Error Resilience

**Test:** During active VPN session, observe logcat for "continuing..." messages
**Expected:** Transient packet errors appear as log lines but do not cause disconnection/reconnection
**Why human:** Transient errors occur at Cloudflare's discretion; cannot be triggered programmatically without a test harness

### 3. AAR Build

**Test:** Run `bash build-usque.sh` to produce `app/libs/usquebind.aar`
**Expected:** AAR builds successfully with gomobile
**Why human:** Requires gomobile + Android NDK installed locally (not available in CI environment)

### Gaps Summary

No gaps found. All 7 observable truths verified, all 5 requirements satisfied, all artifacts substantive and wired, `go vet` passes clean, and all dead code confirmed removed. The phase goal -- forwarding loops detect MASQUE session death via CloseError and trigger immediate reconnection with constant delay -- is achieved.

### Commit Verification

| Commit | Message | Verified |
|--------|---------|----------|
| e9ea829 | feat(02-01): add CloseError classification to forwardUp and forwardDown | Exists in git log |
| 0584e9a | feat(02-02): replace backoff with constant delay, fix QUIC config, unconditional DNS reset | Exists in git log |

---

_Verified: 2026-04-01T11:00:00Z_
_Verifier: Claude (gsd-verifier)_
