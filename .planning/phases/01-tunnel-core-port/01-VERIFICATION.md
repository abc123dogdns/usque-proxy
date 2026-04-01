---
phase: 01-tunnel-core-port
verified: 2026-04-01T12:00:00Z
status: passed
score: 7/7
re_verification:
  previous_status: gaps_found
  previous_score: 2/7
  gaps_closed:
    - "When MASQUE session dies server-side, forwardUp or forwardDown detects connectip.CloseError and sends to errChan"
    - "Non-fatal packet write/read errors on Connect-IP are logged but do not tear down the tunnel"
    - "After tunnel death, reconnect uses constant 1-second delay with no exponential backoff"
    - "QUIC keepalive period is 30 seconds"
    - "Lifetime rotation timer and maxConnLifetime are removed from the select loop"
  gaps_remaining: []
  regressions: []
---

# Phase 01: Tunnel Core Port Verification Report

**Phase Goal:** Tunnel connections detect death immediately through forwarding loop errors and reconnect within seconds
**Verified:** 2026-04-01T12:00:00Z
**Status:** passed
**Re-verification:** Yes -- after gap closure (previous verification found 5/7 truths failed due to bad merge)

## Goal Achievement

### Observable Truths (Plan 01)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | When MASQUE session dies server-side, forwardUp or forwardDown detects connectip.CloseError and sends to errChan | VERIFIED | bind.go lines 829, 839, 856 contain `errors.As(err, new(*connectip.CloseError))` -- 3 occurrences total |
| 2 | Non-fatal packet write/read errors on Connect-IP are logged but do not tear down the tunnel | VERIFIED | bind.go lines 833, 843, 860 use `log.Printf(... "continuing...")` and `continue` for non-CloseError cases |
| 3 | TUN device read/write failures always trigger reconnection | VERIFIED | bind.go line 791 sends device.ReadPacket errors directly to errChan; line 869 sends device.WritePacket errors directly to errChan -- no CloseError check on TUN errors |
| 4 | After tunnel death, reconnect uses constant 1-second delay with no exponential backoff | VERIFIED | bind.go line 458 declares `reconnectDelay = 1 * time.Second`; lines 520, 531, 559, 567, 620 all use `reconnectDelay`; zero matches for minBackoff, maxBackoff, networkGraceMax, networkGraceAttempts |
| 5 | QUIC keepalive period is 30 seconds | VERIFIED | bind.go line 540: `KeepAlivePeriod: 30 * time.Second` |
| 6 | Lifetime rotation timer and maxConnLifetime are removed from the select loop | VERIFIED | Zero matches for lifetimeTimer, maxConnLifetime, func nextBackoff, func livenessCheck in bind.go |
| 7 | Each reconnect cycle closes ipConn, udpConn, tr and waits for goroutines before retry | VERIFIED | bind.go line 608 `cleanup(ipConn, udpConn, tr)`, line 609 `wg.Wait()`, lines 613-614 `dns.resetConnections()` |

**Plan 01 Score:** 7/7 truths verified

### Observable Truths (Plan 02 -- regression check)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | GetStats() no longer returns delivery_ratio, rx_stall_sec, etc. | VERIFIED | Zero matches for these keys in bind.go |
| 2 | Kotlin TunnelStats has only 8 fields | VERIFIED | VpnViewModel.kt lines 30-39: txBytes, rxBytes, connected, hasNetwork, connectCount, lastError, uptimeSec, connectedSinceMs |
| 3 | DebugScreen cleaned of obsolete rows | VERIFIED | Zero matches for TX packets, RX packets, Delivery ratio, RX stall, Lifetime rotations in DebugScreen.kt |
| 4 | nextBackoff function removed | VERIFIED | Zero matches for `func nextBackoff` |
| 5 | livenessCheck function removed | VERIFIED | Zero matches for `func livenessCheck` |
| 6 | Obsolete atomics removed | VERIFIED | Zero matches for txPackets, rxPackets, lastDeliveryRatio, lifetimeRotations in bind.go |

**Plan 02 Score:** 6/6 truths verified (no regressions)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `usque-bind/bind.go` | Rewritten forwardUp, forwardDown, maintainTunnel with CloseError classification and constant reconnect delay | VERIFIED | All Plan 01 changes present; Plan 02 cleanup also intact |
| `app/.../VpnViewModel.kt` | Cleaned TunnelStats data class (8 fields) | VERIFIED | Lines 30-39 confirm 8 fields, no obsolete fields |
| `app/.../DebugScreen.kt` | Cleaned debug screen | VERIFIED | No obsolete rows found |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| forwardUp | errChan | CloseError check before sending | WIRED | Lines 829-831: CloseError -> errChan; lines 833-834: non-fatal -> log+continue |
| forwardDown | errChan | CloseError check before sending | WIRED | Lines 856-858: CloseError -> errChan; lines 860-861: non-fatal -> log+continue |
| maintainTunnel | reconnect delay | constant reconnectDelay | WIRED | Line 458 declares constant; line 620 uses it in post-select sleep |
| GetStats() | refreshStats() | JSON keys must match | WIRED | Verified in previous verification, no regression |

### Additional Verifications

| Check | Result |
|-------|--------|
| `wg.Add(2)` (not 3 -- livenessCheck not spawned) | Line 589: `wg.Add(2)` confirmed |
| No connCtx/connCancel in maintainTunnel | Zero matches confirmed |
| Network-triggered reconnect uses 200ms | Line 618: `200*time.Millisecond` confirmed |
| waitForNetwork still present | Line 557 calls waitForNetwork when no network |
| dns.resetConnections() after cleanup | Lines 613-614 confirmed |
| No math/rand import | Zero matches confirmed |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-----------|-------------|--------|----------|
| TUNL-01 | 01-01 | Forwarding goroutines detect CloseError and trigger immediate reconnection | SATISFIED | 3 CloseError checks in forwardUp/forwardDown (lines 829, 839, 856) |
| TUNL-02 | 01-01 | Reconnection uses constant 1-second delay instead of exponential backoff | SATISFIED | `reconnectDelay = 1 * time.Second` (line 458); no backoff variables |
| TUNL-03 | 01-01, 01-02 | Each reconnect cycle cleanly tears down all resources | SATISFIED | cleanup() + wg.Wait() (lines 608-609); no leaked state from removed mechanisms |
| TUNL-04 | 01-01 | Tunnel loop retries infinitely with no maximum attempt limit | SATISFIED | `for` loop with no max counter; only exits on ctx.Done() |
| TUNL-05 | 01-01 | Non-fatal forwarding errors logged but don't trigger reconnection | SATISFIED | log.Printf + continue pattern on lines 833, 843, 860 |
| CMPT-05 | 01-01 | QUIC keepalive period set to 30 seconds | SATISFIED | Line 540: `KeepAlivePeriod: 30 * time.Second` |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (none) | - | - | - | No anti-patterns found |

### Behavioral Spot-Checks

Step 7b: SKIPPED (Android native library -- requires gomobile build + device to test; no runnable entry points on host)

### Human Verification Required

### 1. Tunnel Death Detection Under Real Conditions

**Test:** Connect VPN, wait 2-4 hours, verify tunnel reconnects automatically when Cloudflare drops the MASQUE session
**Expected:** Tunnel detects death via CloseError within seconds and reconnects with ~1 second delay
**Why human:** Requires real Android device with active VPN connection over extended period

### 2. Non-Fatal Error Resilience

**Test:** Use the VPN under poor network conditions (e.g., switching between WiFi and cellular)
**Expected:** Individual packet errors are logged but do not cause unnecessary reconnects; only true connection death triggers reconnect
**Why human:** Requires real network conditions that produce transient packet errors

### Gaps Summary

No gaps. All 5 previously-failed truths from the initial verification have been resolved. The bad merge that dropped Plan 01 changes has been corrected -- CloseError classification, constant reconnect delay, 30s keepalive, lifetime timer removal, and livenessCheck removal are all present in the codebase. Plan 02 cleanup changes show no regressions. All 6 requirement IDs (TUNL-01 through TUNL-05 and CMPT-05) are satisfied.

---

_Verified: 2026-04-01T12:00:00Z_
_Verifier: Claude (gsd-verifier)_
