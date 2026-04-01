# Codebase Concerns

**Analysis Date:** 2026-04-01

## Tech Debt

**Silent Auto-Tunnel Failure on Start:**
- Issue: When auto-tunnel fails to start a VPN (`AutoTunnelEvent.Start`), the error is swallowed with only a Timber log. There is no user notification or retry logic.
- Files: `android/app/src/main/java/com/zaneschepke/wireguardautotunnel/core/service/autotunnel/AutoTunnelService.kt:368-371`
- Impact: Users see no feedback when auto-tunnel silently fails to bring up a VPN. The tunnel stays down with no indication.
- Fix approach: Emit a notification to the user or post a side effect to the global snackbar on failure. The `TODO notify or retry` comment marks this location.

**Location Permission Polling Instead of Reactive Check:**
- Issue: The location permissions job uses a state-flow observer rather than a system callback, requiring the user to re-open the app or trigger a network event to dismiss stale permission notifications.
- Files: `android/app/src/main/java/com/zaneschepke/wireguardautotunnel/core/service/autotunnel/AutoTunnelService.kt:275-276`
- Impact: Users who grant location permission while the app is backgrounded may still see the warning notification until the next network/settings change.
- Fix approach: Use `ActivityCompat.requestPermissions` callback or a `PermissionChecker` broadcast to proactively clear the notification.

**`handleLockDownModeInit` Can Crash Before Foreground Service Starts:**
- Issue: `TunnelManager.handleLockDownModeInit()` calls `setBackendMode()` which exercises the VPN backend, but the foreground VPN service may not have started yet (e.g. when triggered via WorkManager).
- Files: `android/app/src/main/java/com/zaneschepke/wireguardautotunnel/core/tunnel/TunnelManager.kt:237-258`
- Impact: Can produce a silent crash or `ServiceNotRunning`/`NotAuthorized` exception when lock-down mode is triggered by the service restore or reboot worker before the foreground service is active.
- Fix approach: Add a guard that waits for the tunnel service to be bound (similar to `ServiceManager.startTunnelService`) before calling `setBackendMode`.

**Quick Settings Tile: Multi-Tunnel Toggle Incomplete:**
- Issue: `TunnelControlTile.onClick()` stores "last active tunnels" in a global `@Volatile` list on `WireGuardAutoTunnel`. Managing multi-tunnel state via a class-level static list is acknowledged as insufficient in a TODO comment.
- Files: `android/app/src/main/java/com/zaneschepke/wireguardautotunnel/core/service/tile/TunnelControlTile.kt:95-98`, `android/app/src/main/java/com/zaneschepke/wireguardautotunnel/WireGuardAutoTunnel.kt:95-105`
- Impact: Multi-tunnel toggle via the quick settings tile is unreliable; state can be lost across process restarts.
- Fix approach: Persist last-active tunnel IDs in the database (a simple column on the tunnel config or a dedicated table) rather than an in-memory static list.

**Amnezia Compatibility Mode Detection Workaround:**
- Issue: `InterfaceProxy.isAmneziaCompatibilityModeSet()` includes a comment `TODO fix this later when we get amnezia to properly return 0`, indicating that the library does not correctly return zero for unset junk parameters, requiring a workaround that compares magic headers to fixed values instead.
- Files: `android/app/src/main/java/com/zaneschepke/wireguardautotunnel/ui/state/InterfaceProxy.kt:123-131`
- Impact: If the Amnezia library's behaviour changes, the compatibility detection will silently break. `isCompatibleWithStandardWireGuard()` delegates entirely to this check.
- Fix approach: Revisit after the upstream `amnezia-vpn/amnezia-client` library is updated to expose a reliable zero-value for unused parameters.

**Ping Monitor Does Not Cover LOCK_DOWN or PROXY Modes:**
- Issue: `TunnelMonitorHandler.startPingMonitor()` explicitly skips ping monitoring when `appMode == AppMode.LOCK_DOWN || appMode == AppMode.PROXY` with a `TODO for now until we get monitoring for these modes` comment.
- Files: `android/app/src/main/java/com/zaneschepke/wireguardautotunnel/core/tunnel/handler/TunnelMonitoringHandler.kt:177-178`
- Impact: Users in proxy or lockdown mode get no tunnel health visibility; the ping/reconnect flow that rescues stale tunnels is inactive.
- Fix approach: Implement ping monitoring for proxy/userspace tunnel modes and remove the guard.

**`runBlocking` Inside a Room Migration:**
- Issue: `MIGRATION_23_24` calls `runBlocking` to read from DataStore inside a Room migration callback, which runs on the background thread Room provides. This can deadlock on some devices if the thread pool is exhausted.
- Files: `android/app/src/main/java/com/zaneschepke/wireguardautotunnel/data/migrations/Migrations.kt:237`
- Impact: Potential ANR or silent hang during first-run database upgrade on large user bases.
- Fix approach: Extract DataStore values before calling `migrate()` and pass them in via a lambda or constructor parameter (as already done with the `dataStore: DataStore<Preferences>` parameter pattern used in the same migration function).

**Duplicate Service Declarations in AndroidManifest:**
- Issue: `TunnelControlTile` and `AutoTunnelControlTile` are each declared twice in `AndroidManifest.xml` with identical attributes.
- Files: `android/app/src/main/AndroidManifest.xml:117-183`
- Impact: Duplicate entries are merged at build time but indicate the manifest has not been reviewed. May cause confusion during future refactoring or when adding per-service intent-filters.
- Fix approach: Remove the redundant second declaration for each tile service.

**WgQuick and AmQuick Stored as Raw Strings:**
- Issue: The entire WireGuard/AmneziaWG configuration (including private keys and pre-shared keys) is stored as a raw text blob in the `wg_quick`/`am_quick` columns in the Room database. The database is on-device (not backed up since `allowBackup="false"`), but there is no encryption at rest for the key material.
- Files: `android/app/src/main/java/com/zaneschepke/wireguardautotunnel/data/entity/TunnelConfig.kt:12-19`
- Impact: On rooted devices or devices with accessible storage, private keys and pre-shared keys are exposed in the SQLite file without encryption.
- Fix approach: Encrypt tunnel config blobs using the Android Keystore (EncryptedSharedPreferences or a custom cipher wrapper) before persisting to Room.

**`TunnelConfig.pingTarget` is `var` on a `data class`:**
- Issue: `domain/model/TunnelConfig.kt` declares `var pingTarget: String?` inside a `data class`, making one field mutable while all others are `val`. This breaks value-semantic equality expectations for a domain model.
- Files: `android/app/src/main/java/com/zaneschepke/wireguardautotunnel/domain/model/TunnelConfig.kt:25`
- Impact: `data class` copy semantics can behave unexpectedly if `pingTarget` is mutated after construction. Coroutine flows that use the class for equality deduplication (`distinctUntilChanged`) may miss updates.
- Fix approach: Change `var pingTarget` to `val pingTarget`.

## Known Bugs

**`TunnelLifecycleManager.startTunnel` Returns Failure for Non-`Up` Status:**
- Symptoms: Any non-`Up` terminal state from the backend (e.g. `Down` on immediate close) completes the `startupCompleted` deferred with `Result.failure(UnknownError())`, even if the close was intentional.
- Files: `android/app/src/main/java/com/zaneschepke/wireguardautotunnel/core/tunnel/TunnelLifecycleManager.kt:72-77`
- Trigger: Starting a tunnel that immediately transitions to `Down` (e.g. invalid config before exception is thrown via the backend) produces a generic `UnknownError`.
- Workaround: Backend exceptions are caught separately and produce typed errors; the `Down` path only applies to very quick shutdown scenarios.

## Security Considerations

**RemoteControlReceiver Exported Without Signature-Level Protection:**
- Risk: `RemoteControlReceiver` is exported (`android:exported="true"`) and responds to six broadcast actions. It uses a simple string key (`EXTRA_KEY`) sent as a plaintext broadcast extra for authentication.
- Files: `android/app/src/main/AndroidManifest.xml:249-262`, `android/app/src/main/java/com/zaneschepke/wireguardautotunnel/core/broadcast/RemoteControlReceiver.kt:57-58`
- Current mitigation: Remote control is disabled by default; users must opt in and set a key. The `tools:ignore="ExportedReceiver"` suppression is intentional.
- Recommendations: Document the threat model clearly. Consider requiring a `protectionLevel="signature"` or `signatureOrSystem` permission for the broadcast, or migrating to a content-provider/AIDL-based approach so the key is not visible in `dumpsys activity broadcasts`.

**Private Keys Visible in Config Editor UI:**
- Risk: `InterfaceFields.kt` renders private key content in a `ConfigurationTextBox`. While it uses `PasswordVisualTransformation` by default, the key is toggleable to plaintext with a single tap and can be copied to clipboard.
- Files: `android/app/src/main/java/com/zaneschepke/wireguardautotunnel/ui/screens/tunnels/config/components/InterfaceFields.kt:59-84`
- Current mitigation: The field is masked by default. Screen capture protection is not observed at the Activity level.
- Recommendations: Add `WindowManager.LayoutParams.FLAG_SECURE` when the private key is visible, or restrict clipboard copy to system-only (use `ClipboardManager` with `clipDescription` sensitivity flag on API 33+).

**Remote Control Key Stored in Plaintext in Room:**
- Risk: The remote control key is stored in the `remote_key` column of `general_settings` unencrypted.
- Files: `android/app/src/main/java/com/zaneschepke/wireguardautotunnel/data/entity/GeneralSettings.kt`, migration at `Migrations.kt:251`
- Current mitigation: `allowBackup="false"` prevents cloud backup of the database.
- Recommendations: Encrypt the `remote_key` value using Android Keystore before storage, the same as recommended for tunnel private keys.

## Performance Bottlenecks

**WireGuard Stats Polled Every 1 Second Per Active Tunnel:**
- Problem: `TunnelMonitorHandler.startWgStatsPoll` runs a tight 1-second loop calling `getStatistics()` and `updateTunnelStatus()` for each active tunnel. With multiple tunnels or a slow backend, this accumulates on the IO dispatcher.
- Files: `android/app/src/main/java/com/zaneschepke/wireguardautotunnel/core/tunnel/handler/TunnelMonitoringHandler.kt:354-368`, `STATS_DELAY = 1_000L`
- Cause: The 1 s constant was chosen for smooth notification traffic display but creates contention on the shared IO dispatcher.
- Improvement path: Decouple notification refresh rate from the stats collection rate. Use a longer poll interval (e.g. 5 s) for stats and only push to the notification every 1 s using the most recent cached value.

**`BaseTunnelForegroundService` Queries All Tunnels on Every Active-Tunnel-Key Change:**
- Problem: `start()` subscribes to `activeTunnels.distinctByKeys()` and calls `tunnelsRepository.getAll()` on every emit to find config names for the notification.
- Files: `android/app/src/main/java/com/zaneschepke/wireguardautotunnel/core/service/BaseTunnelForegroundService.kt:92-102`
- Cause: `getAll()` is a suspend DB query triggered on each tunnel key-set change.
- Improvement path: Use `tunnelsRepository.userTunnelsFlow` instead of a one-shot `getAll()` to avoid repeated DB queries on active state changes.

**MainActivity Has 583 Lines of Mixed Navigation, Side-Effect Handling, and UI:**
- Problem: `MainActivity.kt` contains the full navigation graph, all VPN permission flows, snackbar handling, app-mode switching, and the backup/restore integration.
- Files: `android/app/src/main/java/com/zaneschepke/wireguardautotunnel/MainActivity.kt`
- Cause: Navigation3 setup and composable wiring accumulated incrementally.
- Improvement path: Extract navigation entry definitions to a separate `NavGraph.kt` composable; move VPN permission logic into a dedicated `VpnPermissionHandler` helper.

## Fragile Areas

**ServiceWorker Restores Auto-Tunnel by Checking In-Memory Service Reference:**
- Files: `android/app/src/main/java/com/zaneschepke/wireguardautotunnel/core/worker/ServiceWorker.kt:48`
- Why fragile: `serviceManager.autoTunnelService.value == null` checks whether `AutoTunnelService` is bound (in-process reference), not whether it is alive in the OS. If the service is alive but the binding was lost (e.g. after a memory trim), the worker will attempt a redundant `startForegroundService` call.
- Safe modification: Also check with a lightweight `ActivityManager.getRunningServices()` call (with caveats for API 26+) or send a keepalive ping to the service before deciding to restart.
- Test coverage: No unit tests cover the `ServiceWorker` logic.

**`TunnelLifecycleManager.stopTunnel` Has a 5-Second Hard Timeout:**
- Files: `android/app/src/main/java/com/zaneschepke/wireguardautotunnel/core/tunnel/TunnelLifecycleManager.kt:107-115`, `STOP_TIMEOUT_MS = 5_000L`
- Why fragile: If the backend takes more than 5 s to stop (e.g. kernel module teardown under load), `forceStopTunnel` is called, which can leave the kernel WireGuard interface in an inconsistent state.
- Safe modification: Increase the timeout for kernel mode or differentiate timeout per `AppMode`.
- Test coverage: No automated tests for the timeout path.

**`TunnelActiveStatePersister` Takes a Snapshot of All Tunnels on Every State Change:**
- Files: `android/app/src/main/java/com/zaneschepke/wireguardautotunnel/core/tunnel/handler/TunnelActiveStateHandler.kt:26`
- Why fragile: `tunnelsRepository.userTunnelsFlow.firstOrNull()` is called inside the `activeTunnels` collector, taking a one-shot snapshot. If the tunnels list has not emitted yet (e.g. on cold start), the `tunnelsById` map is empty and `isActive` state is not persisted.
- Safe modification: Combine `activeTunnels` and `userTunnelsFlow` rather than using `firstOrNull()` to ensure both sources are populated before acting.
- Test coverage: No tests cover this race condition.

**Database Migration Gap: Versions 23→24 and 25→26 Use Manual SQL:**
- Files: `android/app/src/main/java/com/zaneschepke/wireguardautotunnel/data/migrations/Migrations.kt`
- Why fragile: These are large hand-written migrations that reconstruct tables from scratch. Silent failures in data migration are caught with a `try/catch` that inserts default rows, which means data loss is silently accepted rather than surfaced as an error.
- Safe modification: Add assertions after migration to verify that the row counts in new tables match the row counts from old tables before dropping them.
- Test coverage: `MigrationTest.kt` only tests migration 6→7. Migrations 23→24 and 25→26 have no automated test coverage.

## Scaling Limits

**Multiple Active Tunnels:**
- Current capacity: The UI, notification, and tile all handle the multi-tunnel case with a generic "Multiple" label. The `BaseTunnelForegroundService.createTunnelsNotification()` shows no per-tunnel traffic stats when more than one tunnel is active.
- Limit: The notification traffic display only works for a single active tunnel (`singleOrNull()` in `restartStatsUpdaterIfNeeded`). All multi-tunnel users lose per-tunnel traffic visibility in the notification.
- Scaling path: Implement per-tunnel notification groups or an expanded notification style for multi-tunnel.

## Dependencies at Risk

**`roomdatabasebackup` Library:**
- Risk: `de.raphaelebner:roomdatabasebackup` is a third-party, community-maintained library used in `MainActivity.kt` (line 122, `RoomBackup`). The library wraps the Room database file for backup/restore; it bypasses Room's WAL mode cleanly only if called while no database transactions are in flight.
- Impact: Backup taken while the tunnel is active could produce a corrupted or inconsistent backup file.
- Migration plan: Migrate to Android's built-in `BackupAgent` or use Room's own `backup()` API (available in newer Room versions) which handles WAL checkpointing correctly.

**`orbit-mvi` Orbit ViewModel Library:**
- Risk: `SharedAppViewModel` uses `org.orbitmvi:orbit-viewmodel`. This adds an MVI abstraction layer. If the project needs to migrate to Jetpack's `UDF`/`SavedStateHandle` patterns or if the library lags behind Compose lifecycle changes, it creates a refactor burden.
- Impact: All viewmodel state is expressed through `ContainerHost`/`intent`/`reduce` which is unfamiliar to contributors not familiar with Orbit.
- Migration plan: Not urgent; ensure Orbit is kept current with Compose lifecycle updates.

## Missing Critical Features

**No Recovery Notification or Retry on Auto-Tunnel Start Failure:**
- Problem: When `AutoTunnelService` fails to start a tunnel (e.g. VPN permission revoked, DNS hang), no notification is shown and no retry is attempted.
- Blocks: Users in an auto-tunnel scenario may have no VPN protection without knowing it.

**Ping Monitoring Unavailable in PROXY and LOCK_DOWN Modes:**
- Problem: The tunnel health monitoring (ping + log health) is explicitly disabled for proxy and lockdown app modes.
- Blocks: Tunnel restart on failure (`restartOnPingFailure`) cannot function in these modes; users relying on auto-healing tunnels must use VPN mode.

## Test Coverage Gaps

**Core Tunnel Lifecycle Has No Unit Tests:**
- What's not tested: `TunnelLifecycleManager` start/stop/timeout flows, `TunnelManager` mode switching, `TunnelMonitorHandler` ping logic.
- Files: `android/app/src/main/java/com/zaneschepke/wireguardautotunnel/core/tunnel/`
- Risk: Regressions in tunnel start/stop races or monitoring loops are caught only in manual QA.
- Priority: High

**Auto-Tunnel State Machine Has No Unit Tests:**
- What's not tested: `AutoTunnelState.determineAutoTunnelEvent()`, network-change debounce logic, re-evaluation scheduling.
- Files: `android/app/src/main/java/com/zaneschepke/wireguardautotunnel/core/service/autotunnel/AutoTunnelService.kt`, `android/app/src/main/java/com/zaneschepke/wireguardautotunnel/domain/state/AutoTunnelState.kt`
- Risk: Edge cases (e.g. duplicate network events, race between settings change and network change) go untested.
- Priority: High

**Database Migrations 23→24, 25→26, 28→29 Are Not Tested:**
- What's not tested: The three manual migration paths in `Migrations.kt`. Only migration 6→7 has an instrumented test.
- Files: `android/app/src/androidTest/java/com/zaneschepke/wireguardautotunnel/MigrationTest.kt`
- Risk: Silent data loss or schema mismatch for users upgrading from versions 23, 25, or 28 of the database schema.
- Priority: High

**ServiceWorker and Restore Logic Have No Tests:**
- What's not tested: `ServiceWorker.doWork()`, `TunnelManager.handleRestore()`, `TunnelManager.handleReboot()`.
- Files: `android/app/src/main/java/com/zaneschepke/wireguardautotunnel/core/worker/ServiceWorker.kt`, `android/app/src/main/java/com/zaneschepke/wireguardautotunnel/core/tunnel/TunnelManager.kt:267-313`
- Risk: Boot-restore and always-on VPN restore paths can silently fail without tests to catch regressions.
- Priority: Medium

---

*Concerns audit: 2026-04-01*
