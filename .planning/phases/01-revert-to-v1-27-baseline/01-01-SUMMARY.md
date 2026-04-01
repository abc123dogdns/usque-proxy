---
phase: 1
plan: 1
title: Revert all source files to v1.27 baseline
status: complete
started: 2026-04-01
completed: 2026-04-01
---

# Summary: 01-01 Revert all source files to v1.27 baseline

## What Was Done

Restored all Go and Kotlin source files to their exact v1.27 tag state using `git checkout v1.27`. Removed 2 files added after v1.27 (DebugScreen.kt, Project.xml).

## Key Files

### Modified (reverted to v1.27)
- `usque-bind/bind.go` — Go tunnel core
- `usque-bind/doh.go` — DNS-over-HTTPS
- `usque-bind/doq.go` — DNS-over-QUIC
- `app/src/main/java/com/nhubaotruong/usqueproxy/vpn/UsqueVpnService.kt` — VPN service
- `app/src/main/java/com/nhubaotruong/usqueproxy/ui/viewmodel/VpnViewModel.kt` — ViewModel
- `app/src/main/java/com/nhubaotruong/usqueproxy/MainActivity.kt` — Main activity
- `app/src/main/java/com/nhubaotruong/usqueproxy/ui/nav/AppNavigation.kt` — Navigation
- `app/build.gradle.kts` — Build config
- `app/src/main/AndroidManifest.xml` — Manifest

### Removed
- `app/src/main/java/com/nhubaotruong/usqueproxy/ui/screen/DebugScreen.kt`
- `.idea/codeStyles/Project.xml`

## Verification

- `git diff v1.27 -- usque-bind/ app/` produces zero output — byte-identical to v1.27
- 11 files changed: 213 insertions, 963 deletions (net removal of v1.0 changes)

## Self-Check: PASSED

All source files verified byte-identical to v1.27 tag.
