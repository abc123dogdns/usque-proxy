# Codebase Structure

**Analysis Date:** 2026-04-01

## Directory Layout

```
usque-proxy/                        # Gradle root project
├── app/                            # Android application module (Kotlin/Compose)
│   ├── build.gradle.kts            # App Gradle config, dependencies, signing
│   ├── libs/                       # Prebuilt AAR (usquebind.aar + sources JAR)
│   ├── proguard-rules.pro
│   └── src/
│       ├── main/
│       │   ├── AndroidManifest.xml
│       │   ├── java/com/nhubaotruong/usqueproxy/
│       │   │   ├── MainActivity.kt             # Single Activity entry point
│       │   │   ├── data/                       # Preferences, models, enums
│       │   │   ├── receiver/                   # BootReceiver
│       │   │   ├── tile/                       # Quick Settings tile
│       │   │   ├── ui/
│       │   │   │   ├── component/              # Shared composables
│       │   │   │   ├── nav/                    # AppNavigation (pager)
│       │   │   │   ├── screen/                 # Full-page composables
│       │   │   │   ├── theme/                  # Material3 theme
│       │   │   │   └── viewmodel/              # VpnViewModel
│       │   │   └── vpn/                        # UsqueVpnService
│       │   └── res/                            # Android resources
│       └── test/                               # Unit tests (stub)
├── usque-bind/                     # Go package — compiled to AAR by gomobile
│   ├── bind.go                     # Main JNI API + tunnel state machine (~900 lines)
│   ├── doh.go                      # DNS-over-HTTPS + HTTP/3 DNS proxy
│   ├── doq.go                      # DNS-over-QUIC proxy
│   ├── tools.go                    # gomobile blank import
│   └── go.mod                      # Go 1.24.2; module = usquebind
├── usque-rs/                       # Rust CLI — standalone MASQUE client for Linux
│   ├── src/
│   │   ├── main.rs                 # CLI entry, clap commands
│   │   ├── tunnel.rs               # MASQUE reconnect loop + stats
│   │   ├── register.rs             # WARP API registration
│   │   ├── tun_device.rs           # TUN fd creation + rtnetlink setup
│   │   ├── config.rs               # Config serde struct
│   │   ├── tls.rs                  # TLS/QUIC helpers
│   │   ├── packet.rs               # IP packet parsing
│   │   └── icmp.rs                 # ICMP handling
│   ├── tests/
│   │   └── tunnel_mtu.rs           # Integration test for MTU
│   └── Cargo.toml
├── android/                        # Secondary Android module (wireguard-auto-tunnel fork)
│   └── app/src/main/java/com/zaneschepke/wireguardautotunnel/
│       ├── core/                   # Services, tunnel backend, workers
│       ├── data/                   # Room DB, DAOs, repositories, DataStore
│       ├── di/                     # Hilt dependency injection modules
│       ├── domain/                 # Domain models, events, states
│       └── ui/                     # Compose screens for auto-tunnel config
├── usque-android/                  # Git submodule — upstream usque reference app (Go)
│   ├── android/usque-vpn/          # Simple Android VPN demo (XML layout, no Compose)
│   ├── api/                        # Go: Cloudflare WARP API client
│   ├── cmd/                        # Go: CLI commands (register, nativetun, socks, portfw…)
│   ├── config/                     # Go: config model
│   ├── internal/                   # Go: internal helpers
│   └── models/                     # Go: API response models
├── build-usque.sh                  # Builds usque-bind → app/libs/usquebind.aar
├── build.gradle.kts                # Root Gradle (plugin versions only)
├── settings.gradle.kts             # Includes only `:app` module
├── gradle.properties               # JVM args, AndroidX flags
└── keystore.properties             # Signing config (not committed with secrets)
```

## Directory Purposes

**`app/src/main/java/com/nhubaotruong/usqueproxy/data/`:**
- Purpose: All persistence and data models for the Android app
- Contains: `VpnPreferences.kt` (DataStore CRUD + Flow), `VpnPrefs` data class, enums (`SplitMode`, `DnsMode`, `ProfileType`, `ThemeMode`), `AppRepository.kt` (installed apps list), `Office365Endpoints.kt` (route exclusion CIDRs)
- Key files: `VpnPreferences.kt`, `VpnPrefs` data class (inside same file)

**`app/src/main/java/com/nhubaotruong/usqueproxy/vpn/`:**
- Purpose: Android VPN service — the most complex file in the project
- Contains: `UsqueVpnService.kt` (1056 lines) — TUN fd creation, VPN builder, keepalive, watchdog, network callbacks, Doze handling, notification
- Key files: `UsqueVpnService.kt`

**`app/src/main/java/com/nhubaotruong/usqueproxy/ui/screen/`:**
- Purpose: Full-page Compose screens, one file per tab
- Contains: `MainScreen.kt` (connect button, status, stats), `SettingsScreen.kt` (profiles, DNS, SNI, split tunnel settings), `SplitTunnelScreen.kt` (app list with include/exclude), `DebugScreen.kt` (raw stats JSON)
- Key files: `SettingsScreen.kt` (358 lines, most settings logic)

**`app/src/main/java/com/nhubaotruong/usqueproxy/ui/viewmodel/`:**
- Purpose: Single ViewModel for the entire app
- Contains: `VpnViewModel.kt`, `VpnState` enum, `TunnelStats` data class
- Key files: `VpnViewModel.kt`

**`app/src/main/java/com/nhubaotruong/usqueproxy/ui/nav/`:**
- Purpose: Navigation host — bottom nav + `HorizontalPager` over 4 tabs
- Key files: `AppNavigation.kt`

**`app/src/main/java/com/nhubaotruong/usqueproxy/ui/component/`:**
- Purpose: Shared composables reused across screens
- Contains: `RestartBanner.kt` (shown when a restart is pending)

**`app/src/main/java/com/nhubaotruong/usqueproxy/receiver/`:**
- Purpose: Android broadcast receivers
- Contains: `BootReceiver.kt` (auto-connect on boot)

**`app/src/main/java/com/nhubaotruong/usqueproxy/tile/`:**
- Purpose: Quick Settings tile
- Contains: `VpnTileService.kt`

**`app/libs/`:**
- Purpose: Prebuilt binary AAR — the output of `build-usque.sh`
- Contains: `usquebind.aar`, `usquebind-sources.jar`
- Generated: Yes (by `gomobile bind`)
- Committed: Yes (binary artifact checked in for build reproducibility without Go toolchain)

**`usque-bind/`:**
- Purpose: Go source that compiles into `usquebind.aar`; all tunnel logic lives here
- Contains: Three Go files — `bind.go` (core), `doh.go` (DoH/DNS proxy), `doq.go` (DoQ/DNS proxy)
- Key files: `bind.go`

**`usque-rs/src/`:**
- Purpose: Rust MASQUE client; not used by Android — a standalone tool for Linux/desktop WARP
- Contains: 8 source files, 1 integration test

**`android/`:**
- Purpose: A bundled fork of `zaneschepke/wireguard-android` for auto-tunnel functionality (separate from the main app)
- Note: Has its own Gradle setup, Hilt DI, Room database — architecturally distinct from `app/`

## Key File Locations

**Entry Points:**
- `app/src/main/java/com/nhubaotruong/usqueproxy/MainActivity.kt`: Android Activity, Compose root
- `app/src/main/java/com/nhubaotruong/usqueproxy/vpn/UsqueVpnService.kt`: Foreground VPN service
- `usque-bind/bind.go`: Go JNI API surface and tunnel state machine
- `usque-rs/src/main.rs`: Rust CLI entry point

**Configuration:**
- `app/build.gradle.kts`: Android build config, dependencies, minSdk=35, targetSdk=36, arm64-v8a only
- `usque-bind/go.mod`: Go 1.24.2, module `usquebind`
- `usque-rs/Cargo.toml`: Rust 2021 edition, `quiche` with BoringSSL
- `build-usque.sh`: gomobile build script (run to regenerate the AAR)
- `gradle.properties`: Gradle JVM args
- `settings.gradle.kts`: Only includes `:app` (the `android/` submodule is not a Gradle include)

**Core Logic:**
- `app/src/main/java/com/nhubaotruong/usqueproxy/data/VpnPreferences.kt`: DataStore persistence for all user prefs
- `app/src/main/java/com/nhubaotruong/usqueproxy/ui/viewmodel/VpnViewModel.kt`: All app state and actions
- `usque-bind/bind.go`: `StartTunnel()`, `StopTunnel()`, `Reconnect()`, `Keepalive()`, `GetStats()`, `Register()`, `Enroll()`
- `usque-bind/doh.go`: DNS-over-HTTPS proxy with HTTP/3 upgrade and LRU cache

**Testing:**
- `app/src/test/java/com/nhubaotruong/usqueproxy/`: JVM unit tests (stub only)
- `app/src/androidTest/`: Instrumented tests (empty)
- `usque-rs/tests/tunnel_mtu.rs`: Rust integration test for MTU calculation

## Naming Conventions

**Files:**
- Kotlin: `PascalCase.kt` matching the primary class name (e.g., `VpnViewModel.kt`, `UsqueVpnService.kt`)
- Go: `snake_case.go` by purpose (e.g., `bind.go`, `doh.go`, `doq.go`)
- Rust: `snake_case.rs` by module (e.g., `tun_device.rs`, `tunnel.rs`)

**Directories:**
- Kotlin packages: lowercase singular nouns (`data/`, `vpn/`, `ui/`, `receiver/`, `tile/`)
- Go: package name matches directory; all Go files in `usque-bind/` share package `usquebind`
- Rust modules: snake_case matching filenames declared in `mod` statements in `main.rs`

**Classes and Functions:**
- Kotlin classes: `PascalCase` (e.g., `VpnViewModel`, `UsqueVpnService`)
- Kotlin functions: `camelCase` (e.g., `refreshStats()`, `maintainTunnel()`)
- Go exported functions: `PascalCase` (required for gomobile — e.g., `StartTunnel`, `GetStats`, `Register`)
- Go unexported: `camelCase` (e.g., `maintainTunnel`, `enrollAndBuildConfig`)
- Rust: `snake_case` for functions and modules, `PascalCase` for types

## Where to Add New Code

**New user preference/setting:**
1. Add key constant in `VpnPreferences.Keys` object: `app/src/main/java/com/nhubaotruong/usqueproxy/data/VpnPreferences.kt`
2. Add field to `VpnPrefs` data class in the same file
3. Add getter/setter to `VpnPreferences` class
4. Expose `StateFlow` in `VpnViewModel.kt` if needed for reactive UI
5. Wire to a UI control in the appropriate screen file under `app/.../ui/screen/`
6. If the setting affects tunnel behavior, pass it in the config JSON built in `UsqueVpnService.kt` and handle in `usque-bind/bind.go`'s `tunnelConfig`

**New composable screen:**
- Add as `app/src/main/java/com/nhubaotruong/usqueproxy/ui/screen/MyNewScreen.kt`
- Register as a tab page in `app/src/main/java/com/nhubaotruong/usqueproxy/ui/nav/AppNavigation.kt` (add to `navItems` and `HorizontalPager` `when` block)

**New shared composable component:**
- Place in `app/src/main/java/com/nhubaotruong/usqueproxy/ui/component/`

**New tunnel feature (Go side):**
- Add to `usque-bind/bind.go` (if it needs Android VPN integration / fd access)
- Or as a new file in `usque-bind/` if self-contained (e.g., a new DNS mode would follow the pattern of `doh.go` / `doq.go`)
- Export the function as `PascalCase` for gomobile to make it available to Kotlin
- Rebuild the AAR with `./build-usque.sh` and commit the updated `app/libs/usquebind.aar`

**New Rust module:**
- Add `src/my_module.rs` and declare `mod my_module;` in `usque-rs/src/main.rs`

## Special Directories

**`app/libs/`:**
- Purpose: Prebuilt `usquebind.aar` — the gomobile output containing all Go tunnel code
- Generated: Yes, by running `./build-usque.sh` from project root
- Committed: Yes — binary checked in so Android builds work without Go/gomobile toolchain

**`usque-android/`:**
- Purpose: Upstream `github.com/Diniboy1123/usque` repo as a git submodule (reference implementation)
- Generated: No (git submodule)
- Committed: Submodule pointer committed; content fetched separately

**`.sisyphus/`:**
- Purpose: AI agent planning notes and evidence (internal tooling)
- Generated: No
- Committed: No (in `.gitignore`)

**`.planning/`:**
- Purpose: GSD codebase analysis documents
- Generated: Yes (by Claude)
- Committed: Depends on project convention

---

*Structure analysis: 2026-04-01*
