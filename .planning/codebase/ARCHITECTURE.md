# Architecture

**Analysis Date:** 2026-04-01

## Pattern Overview

**Overall:** Multi-language Android VPN application with a polyglot native core

**Key Characteristics:**
- Android app (Kotlin/Compose) acts as the UI and OS integration shell
- Core VPN tunnel logic lives in a Go library (`usque-bind`) compiled to an AAR via `gomobile`
- A standalone Rust CLI (`usque-rs`) provides a desktop/Linux reference implementation of the same MASQUE tunnel, not shipped in the Android app
- The Go AAR (`usquebind.aar`) is the single bridge between the Android layer and Cloudflare WARP's MASQUE (CONNECT-IP over QUIC/HTTP3) protocol
- Android ViewModel holds all UI state as `StateFlow`; the service emits events via `SharedFlow` instead of polling

## Layers

**UI Layer:**
- Purpose: Render app screens, collect user input, subscribe to state flows
- Location: `app/src/main/java/com/nhubaotruong/usqueproxy/ui/`
- Contains: Composable screens (`screen/`), reusable components (`component/`), navigation (`nav/`), Material3 theme (`theme/`)
- Depends on: ViewModel
- Used by: `MainActivity`

**ViewModel Layer:**
- Purpose: Translate user actions into service intents and JNI calls; expose state as `StateFlow`
- Location: `app/src/main/java/com/nhubaotruong/usqueproxy/ui/viewmodel/VpnViewModel.kt`
- Contains: `VpnViewModel` (single ViewModel), `VpnState` enum, `TunnelStats` data class
- Depends on: `UsqueVpnService` static state/events, `usquebind.Usquebind` JNI (for stats/register), `VpnPreferences`
- Used by: All composable screens via `AppNavigation`

**Data/Preferences Layer:**
- Purpose: Persist user settings and VPN configuration via DataStore; provide typed read model
- Location: `app/src/main/java/com/nhubaotruong/usqueproxy/data/`
- Contains: `VpnPreferences.kt` (DataStore wrapper), `VpnPrefs` data class, `AppRepository.kt`, `Office365Endpoints.kt`, enums (`SplitMode`, `DnsMode`, `ProfileType`, `ThemeMode`)
- Depends on: Jetpack DataStore Preferences
- Used by: `VpnViewModel`, `UsqueVpnService`, `BootReceiver`

**VPN Service Layer:**
- Purpose: Android OS VPN integration — owns the TUN fd, lifecycle, keepalive, watchdog, network callbacks
- Location: `app/src/main/java/com/nhubaotruong/usqueproxy/vpn/UsqueVpnService.kt`
- Contains: `UsqueVpnService` (extends `VpnService`), `VpnServiceEvent` sealed interface, static `isRunning`/`lastError`/`events`
- Depends on: `usquebind.Usquebind` (Go JNI), `usquebind.VpnProtector`, `VpnPreferences`
- Used by: `VpnViewModel` (starts/stops via `Intent`), system via `BootReceiver`/`VpnTileService`/`AlarmManager`

**Go Tunnel Library (usque-bind):**
- Purpose: Implement the full MASQUE tunnel: QUIC/HTTP3 session, CONNECT-IP proxy, DNS interception, keepalive, reconnect loop, stats
- Location: `usque-bind/` (Go package `usquebind`)
- Key files: `bind.go` (main tunnel + JNI API), `doh.go` (DNS-over-HTTPS + HTTP/3 probe), `doq.go` (DNS-over-QUIC)
- Depends on: `github.com/Diniboy1123/usque` (upstream MASQUE library), `github.com/Diniboy1123/connect-ip-go`, `quic-go`, `golang.org/x/mobile`
- Used by: Compiled to `app/libs/usquebind.aar` via `gomobile bind`; consumed from Kotlin via JNI class `usquebind.Usquebind`

**Rust CLI (usque-rs):**
- Purpose: Standalone MASQUE client for Linux/desktop — equivalent functionality to `usque-bind` but for native TUN devices
- Location: `usque-rs/` (binary crate)
- Key files: `src/main.rs`, `src/tunnel.rs`, `src/register.rs`, `src/tun_device.rs`, `src/config.rs`, `src/tls.rs`, `src/packet.rs`, `src/icmp.rs`
- Depends on: `quiche` (BoringSSL QUIC), `tokio`, `mio`, `ring`, `rcgen`, `tun`, `rtnetlink`
- Used by: Not consumed by Android app; independent binary

**Reference Android (usque-android submodule):**
- Purpose: Upstream `usque` Go library's own Android demo/binding — used as reference, not the production app
- Location: `usque-android/` (git submodule)

## Data Flow

**VPN Connect Flow:**

1. User taps Connect in `MainScreen.kt` → `onRequestVpnPermission()` in `MainActivity`
2. `MainActivity` calls `VpnService.prepare()` to check Android VPN permission
3. On grant, `VpnViewModel.connect()` calls `ContextCompat.startForegroundService(ctx, Intent(UsqueVpnService))`
4. `UsqueVpnService.onStartCommand()` builds a `VpnService.Builder`, opens a TUN fd (`vpnInterface`)
5. Service reads config JSON from `VpnPreferences.activeConfigJson` (DataStore), applies split-tunnel, DNS, and route settings
6. Service launches a coroutine calling `Usquebind.startTunnel(configJson, tunFd, protector)` (blocking JNI)
7. Go `StartTunnel()` in `bind.go` dups the fd, starts `maintainTunnel()` which enters a reconnect loop
8. `maintainTunnel()` resolves endpoints, establishes QUIC session to `consumer-masque.cloudflareclient.com:443`, sends HTTP/3 CONNECT-IP request
9. On success, Go emits packets between the TUN device (`FdAdapter`) and the MASQUE proxy indefinitely
10. Service emits `VpnServiceEvent.Started` via `SharedFlow`; `VpnViewModel` sets `_vpnState = CONNECTED`

**Registration Flow:**

1. User enters license key or JWT in `SettingsScreen.kt` → `VpnViewModel.register(license)` or `registerWithJwt(jwt)`
2. ViewModel calls `Usquebind.register(license)` or `Usquebind.registerWithJWT(jwt)` on `Dispatchers.IO`
3. Go `Register()`/`RegisterWithJWT()` in `bind.go` calls `api.Register()` against `https://api.cloudflareclient.com`, generates EC P-256 key pair, enrolls public key
4. Returns serialized config JSON to Kotlin; saved via `VpnPreferences.saveWarpConfig()` / `saveZtConfig()` to DataStore

**Keepalive / Watchdog Loop:**

1. `ScheduledExecutorService` fires every 2 minutes → calls `Usquebind.keepalive()` → Go checks packet age vs 125s idle threshold
2. `AlarmManager` fires every 8 minutes as a Doze fallback, debounced by 60s against executor
3. `UsqueVpnService.watchdogRunnable` runs on `Handler(Looper.getMainLooper())` every 60s — reads `Usquebind.getStats()` JSON, detects one-way rx stalls, surfaces errors with grace period

**State Updates:**

1. `UsqueVpnService` emits `VpnServiceEvent` to `companion object._events: MutableSharedFlow`
2. `VpnViewModel.init` collects the SharedFlow; updates `_vpnState`, `_tunnelError`, `_connectedSince`
3. `AppNavigation` runs a `LaunchedEffect` polling `viewModel.refreshState()` every 5s (reads only volatile `isRunning` — no JNI)
4. `MainScreen` runs a separate `LaunchedEffect` polling `viewModel.refreshStats()` every 10s when visible (calls `Usquebind.getStats()` JNI)

## Key Abstractions

**`VpnProtector` interface:**
- Purpose: Allows Go to call Android `VpnService.protect(fd)` so outbound QUIC sockets bypass the TUN device (no routing loop)
- Examples: Implemented inline in `UsqueVpnService.kt` as a lambda passed to `Usquebind.startTunnel()`
- Pattern: Defined in Go as `type VpnProtector interface { ProtectFd(fd int) bool }`, exposed via gomobile

**`FdAdapter` struct (Go):**
- Purpose: Wraps an `os.File` (Android TUN fd) to satisfy `api.TunnelDevice` interface — `ReadPacket()`/`WritePacket()`
- Examples: `usque-bind/bind.go` lines 97–108

**`VpnServiceEvent` sealed interface:**
- Purpose: Typed events from service to ViewModel without polling
- Examples: `UsqueVpnService.kt` inner sealed interface with `Connecting`, `Started`, `Stopped`, `Disconnecting`, `Error(message)` objects
- Pattern: `companion object._events: MutableSharedFlow<VpnServiceEvent>` with replay=1

**`VpnPrefs` data class:**
- Purpose: Immutable snapshot of all user preferences, emitted as a `Flow` from DataStore
- Examples: `app/src/main/java/com/nhubaotruong/usqueproxy/data/VpnPreferences.kt`
- Pattern: Computed properties `activeConfigJson` and `isActiveRegistered` delegate to the active `ProfileType`

**`tunnelConfig` struct (Go):**
- Purpose: Extends `config.Config` with Android-specific overrides (custom SNI, ConnectURI, DoH/DoQ URLs, system DNS list, Private DNS flag)
- Examples: `usque-bind/bind.go` lines 65–73

## Entry Points

**`MainActivity`:**
- Location: `app/src/main/java/com/nhubaotruong/usqueproxy/MainActivity.kt`
- Triggers: App launch, `ACTION_CONNECT_VPN` intent from tile/boot
- Responsibilities: Hosts single `VpnViewModel`, requests VPN permission, sets Compose content tree, triggers auto-connect on startup

**`UsqueVpnService`:**
- Location: `app/src/main/java/com/nhubaotruong/usqueproxy/vpn/UsqueVpnService.kt`
- Triggers: `startForegroundService` from ViewModel/BootReceiver/TileService; `ACTION_STOP`/`ACTION_RESTART` intents; `ACTION_KEEPALIVE_ALARM` from AlarmManager
- Responsibilities: TUN fd lifecycle, VPN builder configuration, coroutine launch of Go tunnel, keepalive scheduling, watchdog, network callbacks, notification management

**`BootReceiver`:**
- Location: `app/src/main/java/com/nhubaotruong/usqueproxy/receiver/BootReceiver.kt`
- Triggers: `ACTION_BOOT_COMPLETED`
- Responsibilities: Reads DataStore prefs; starts `UsqueVpnService` if `autoConnect` enabled and VPN permission is still granted

**`VpnTileService`:**
- Location: `app/src/main/java/com/nhubaotruong/usqueproxy/tile/VpnTileService.kt`
- Triggers: Quick Settings tile interaction
- Responsibilities: Toggle VPN on/off; open app with `ACTION_CONNECT_VPN` if permission is missing

**`usque-rs/src/main.rs` (Rust CLI):**
- Location: `usque-rs/src/main.rs`
- Triggers: CLI invocation (`usque-rs register` / `usque-rs nativetun`)
- Responsibilities: Parse CLI args via `clap`, dispatch to `register::register()` or `tunnel::maintain_tunnel()`

## Error Handling

**Strategy:** Errors from Go are surfaced to Kotlin as exceptions (JNI) or via stats JSON. Transient errors during reconnect are suppressed with a grace period before surfacing to the UI.

**Patterns:**
- Go `StartTunnel()` returns a Go `error`; gomobile converts this to a Java `Exception` thrown from `Usquebind.startTunnel()`
- `UsqueVpnService` catches the exception in `tunnelJob`'s `finally` block; stores in `companion object.lastError`
- Watchdog reads `last_error` field from `Usquebind.getStats()` JSON; only surfaces to UI after `ERROR_GRACE_TICKS` (3) consecutive error ticks
- `VpnViewModel` exposes `tunnelError: StateFlow<String?>` which composables observe; cleared by `clearTunnelError()`
- Rust CLI uses `anyhow::Result` for all fallible operations; errors propagate to `main()` and print to stderr

## Cross-Cutting Concerns

**Logging:**
- Kotlin: `android.util.Log` with `TAG = "UsqueVpnService"`
- Go: standard `log.Printf` / `log.Println`; output captured in Android logcat via gomobile runtime

**Validation:**
- Config JSON is validated at tunnel start time in Go (`json.Unmarshal` into `tunnelConfig`); invalid JSON returns an error before any network activity
- DataStore migrations are handled inline in `VpnPreferences.prefsFlow` map block (legacy key fallback)

**Authentication:**
- Two profile types: `WARP` (license key or anonymous) and `ZERO_TRUST` (JWT from team domain)
- Credentials stored only in Android DataStore as config JSON blob (base64 EC private key inside)
- QUIC TLS uses per-connection client certificates generated by Go (`rcgen` in Rust, `crypto/x509` in Go)

**Battery / Doze:**
- App requests battery optimization exemption at startup (`MainActivity.checkBatteryOptimization()`)
- Dual keepalive: 2-min `ScheduledExecutorService` (reliable with exemption) + 8-min `AlarmManager` (fires in Doze maintenance windows)
- `PARTIAL_WAKE_LOCK` acquired for each keepalive/reconnect attempt, released in `finally` block

---

*Architecture analysis: 2026-04-01*
