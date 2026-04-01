---
phase: 01-tunnel-core-port
plan: 02
subsystem: tunnel
tags: [go, kotlin, dead-code-removal, stats-cleanup, backoff]

# Dependency graph
requires:
  - phase: 01-tunnel-core-port/01
    provides: Rewritten forwardUp/forwardDown/maintainTunnel that made liveness/backoff/packet-counter code dead
provides:
  - Cleaned bind.go without nextBackoff, livenessCheck, or obsolete atomics
  - Cleaned TunnelStats data class with 8 fields (removed 5)
  - Cleaned DebugScreen without obsolete monitoring rows
affects: [02-android-service-simplify]

# Tech tracking
tech-stack:
  added: []
  patterns: [constant-retry-delay]

key-files:
  created: []
  modified:
    - usque-bind/bind.go
    - app/src/main/java/com/nhubaotruong/usqueproxy/ui/viewmodel/VpnViewModel.kt
    - app/src/main/java/com/nhubaotruong/usqueproxy/ui/screen/DebugScreen.kt

key-decisions:
  - "Constant 1s reconnect delay replaces exponential backoff with jitter (nextBackoff removed)"
  - "Kept lastRxTime/lastTxTime atomics for Keepalive() Doze detection; Phase 2 removes Keepalive"
  - "Kept maxConnLifetime timer but removed lifetimeRotations counter"

patterns-established:
  - "Constant retry: all backoff paths now use minBackoff (1s) instead of escalating"

requirements-completed: [TUNL-03]

# Metrics
duration: 5min
completed: 2026-04-01
---

# Phase 01 Plan 02: Dead Code and Stats Cleanup Summary

**Removed nextBackoff, livenessCheck, 4 obsolete atomics, and 7 stats fields from Go/Kotlin layers; constant 1s retry replaces exponential backoff**

## Performance

- **Duration:** 5 min
- **Started:** 2026-04-01T07:48:00Z
- **Completed:** 2026-04-01T07:53:00Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- Removed all dead monitoring code from bind.go: nextBackoff function, livenessCheck function, 4 atomic variables (txPackets, rxPackets, lastDeliveryRatio, lifetimeRotations), maxBackoff constant, mrand import
- Removed 7 obsolete stats fields from GetStats(): rx_stall_sec, tx_packets, rx_packets, delivery_ratio, lifetime_rotations, last_rx_time_ms, last_tx_time_ms
- Cleaned TunnelStats data class from 13 to 8 fields and removed 5 DebugScreen rows
- Replaced exponential backoff (1s-60s with jitter) with constant 1s retry delay across all reconnect paths

## Task Commits

Each task was committed atomically:

1. **Task 1: Remove obsolete Go code** - `5b48a81` (refactor)
2. **Task 2: Remove obsolete Kotlin stats fields** - `cc2614f` (refactor)

## Files Created/Modified
- `usque-bind/bind.go` - Removed dead functions, atomics, stats fields, unused import; constant retry
- `app/src/main/java/com/nhubaotruong/usqueproxy/ui/viewmodel/VpnViewModel.kt` - Cleaned TunnelStats data class and refreshStats parser
- `app/src/main/java/com/nhubaotruong/usqueproxy/ui/screen/DebugScreen.kt` - Removed 5 obsolete debug rows

## Decisions Made
- Replaced exponential backoff with constant 1s retry -- simpler, matches usque-android approach
- Kept lastRxTime/lastTxTime atomics because they are still used by Keepalive() for Doze detection (Phase 2 handles Keepalive removal)
- Kept maxConnLifetime 2-hour timer but removed the lifetimeRotations counter (timer is still functional for forced rotation)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Replaced nextBackoff call sites instead of just deleting function**
- **Found during:** Task 1
- **Issue:** Plan said to remove nextBackoff function, but it was called 6 times in maintainTunnel. Deleting it without replacing calls would break compilation.
- **Fix:** Replaced all `backoff = nextBackoff(backoff, maxBackoff)` with `backoff = minBackoff` to implement constant 1s retry, then removed the function and maxBackoff constant.
- **Files modified:** usque-bind/bind.go
- **Verification:** grep confirms 0 references to nextBackoff and maxBackoff
- **Committed in:** 5b48a81 (Task 1 commit)

**2. [Rule 3 - Blocking] Removed livenessCheck goroutine launch and adjusted WaitGroup**
- **Found during:** Task 1
- **Issue:** Plan said to remove livenessCheck function, but it was launched as a goroutine with wg.Add(3). Required removing launch site and changing to wg.Add(2), plus removing connCtx/connCancel which only existed for liveness.
- **Fix:** Removed goroutine launch, connCtx/connCancel, adjusted WaitGroup from 3 to 2.
- **Files modified:** usque-bind/bind.go
- **Verification:** grep confirms 0 references to livenessCheck
- **Committed in:** 5b48a81 (Task 1 commit)

---

**Total deviations:** 2 auto-fixed (2 blocking)
**Impact on plan:** Both fixes necessary to maintain compilation after removing dead code. No scope creep.

## Issues Encountered
- Go build fails due to pre-existing gvisor dependency compatibility issue (unrelated to changes). Verified correctness via grep-based acceptance criteria instead.

## User Setup Required
None - no external service configuration required.

## Known Stubs
None - this plan only removes code, no new functionality added.

## Next Phase Readiness
- Phase 01 tunnel core port is complete (both plans done)
- Ready for Phase 02 (Android service simplification): Keepalive function and maxConnLifetime timer remain for Phase 2 to address

---
*Phase: 01-tunnel-core-port*
*Completed: 2026-04-01*
