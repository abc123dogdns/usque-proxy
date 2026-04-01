# Coding Conventions

**Analysis Date:** 2026-04-01

## Project Structure

This repo contains two distinct Kotlin Android apps:

- **`app/`** — `com.nhubaotruong.usqueproxy` — the Usque WARP/ZeroTrust VPN app (simpler, no DI framework)
- **`android/app/`** — `com.zaneschepke.wireguardautotunnel` — the WireGuard Auto-Tunnel app (full Koin DI, Orbit MVI, Room)

Conventions below apply to both unless noted.

---

## Naming Patterns

**Files:**
- PascalCase for all Kotlin class files: `TunnelManager.kt`, `VpnViewModel.kt`, `AppRepository.kt`
- Extension function files named after type: `Extensions.kt`, `ContextExtensions.kt`, `TunnelExtensions.kt`, `StringExtensions.kt`
- Interface names without `I` prefix: `TunnelRepository`, `TunnelProvider`, `TunnelBackend`
- Sealed class hierarchy names reflect parent: `BackendCoreException`, `LocalSideEffect`, `BackendMessage`
- Module files suffixed with `Module`: `AppModule.kt`, `DatabaseModule.kt`, `TunnelModule.kt`

**Functions:**
- camelCase for all functions
- Boolean-returning functions use `is`/`has` prefix: `isRunning`, `hasVpnPermission`, `isActive`
- Suspend functions for IO-bound work: `getInstalledApps()`, `startTunnel()`, `stopTunnel()`
- Handler/manager factory functions use descriptive verbs: `handleRestore()`, `handleReboot()`, `handleLockDownModeInit()`

**Variables:**
- camelCase for properties and local vars
- Private backing MutableStateFlow prefixed with `_`: `_vpnState`, `_activeTunnels`, `_stats`, `_connectedSince`
- Public StateFlow exposed without prefix: `vpnState`, `activeTunnels`, `stats`
- Constants in `companion object` or top-level: `RESTART_TUNNEL_DELAY`, `STATE_POLL_INTERVAL`, `STATS_POLL_INTERVAL`

**Types:**
- Enums in SCREAMING_SNAKE_CASE values: `VpnState.DISCONNECTED`, `SplitMode.ALL`, `AppMode.LOCK_DOWN`
- Data classes with all-default constructors for state: `VpnPrefs()`, `TunnelStats()`, `GlobalAppUiState()`
- Sealed classes for event/side-effect hierarchies: `LocalSideEffect`, `BackendCoreException`, `BackendMessage`
- `typealias` used for readability: `TunnelName = String?`, `QuickConfig = String`

**Packages:**
- `domain/` — interfaces, models, enums, repository contracts, events/exceptions
- `data/` — Room entities, DAOs, migrations, AppDatabase
- `core/` — services, tunnel backends, notifications, shortcuts
- `ui/` — screens, components, viewmodels, navigation, themes, state
- `di/` — Koin module definitions (android/app only)
- `util/` — pure utilities, extension functions, constants

---

## Code Style

**Formatting:**
- No `.editorconfig` or `.ktlint` config files detected — formatting is enforced by Android Studio/IDE defaults
- Trailing commas used on multi-line parameter lists and collections
- 4-space indentation
- Max line length approximately 100 chars (inferred from code patterns)
- Kotlin's standard library style throughout

**Annotations:**
- `@OptIn` annotations placed at function or class level when using experimental APIs
- Common opt-ins: `@OptIn(ExperimentalCoroutinesApi::class)`, `@OptIn(ExperimentalAtomicApi::class)`, `@OptIn(ExperimentalMaterial3Api::class)`
- `@Composable` on all Compose functions, `private` on screen-internal composables

---

## Import Organization

**Order (Android Studio default):**
1. Android/framework imports (`android.*`, `androidx.*`)
2. Third-party library imports (`com.wireguard.*`, `org.koin.*`, `timber.*`, `io.ktor.*`)
3. Internal project imports (`com.zaneschepke.*`, `com.nhubaotruong.*`)
4. Kotlin standard library (`kotlin.*`, `kotlinx.*`)

**Wildcard imports:**
- Avoided in main source; `import com.zaneschepke.wireguardautotunnel.viewmodel.*` seen in DI modules only

---

## Error Handling

**Patterns (android/app — WireGuard Auto-Tunnel):**
- Custom sealed exception hierarchy: `BackendCoreException` with typed subclasses (`NotAuthorized`, `DnsFailure`, `InvalidConfig`, etc.)
- Each exception carries a `stringRes: Int` for UI display via `toStringValue()` returning `StringValue.StringResource`
- Errors propagated via `SharedFlow<Pair<String?, BackendCoreException>>` — never thrown across coroutine boundaries
- `runCatching { }` used for fallible operations with `.onFailure { e -> Timber.e(e, ...) }` chaining:
  ```kotlin
  runCatching { stopTunnel(tunnel.id) }
      .onFailure { e -> Timber.e(e, "Failed to stop tunnel ${tunnel.id} during restart") }
  ```
- `tryEmit` used for non-suspending event emission on `MutableSharedFlow`
- `localErrorEvents.emit(null to NotAuthorized())` pattern for broadcasting errors to UI

**Patterns (app — Usque VPN):**
- `try/catch(e: Exception)` with `_registerError.value = e.message` in ViewModel
- `runCatching { }` used in Compose `LaunchedEffect` loops: `runCatching { viewModel.refreshStats() }`
- `runCatching { SplitMode.valueOf(...) }.getOrDefault(SplitMode.ALL)` for safe enum parsing

---

## Logging

**Framework:** Timber (`timber.log.Timber`) — android/app only

**Patterns:**
- `Timber.d(...)` for lifecycle/state changes
- `Timber.w(...)` for recoverable unexpected states
- `Timber.e(exception, message)` for failures
- Log messages include the entity ID: `"Shutting down tunnel monitoring job for tunnelId: $id"`
- Usque VPN app (`app/`) uses no logging — no Timber dependency

---

## State Management

**android/app (WireGuard Auto-Tunnel):**
- Orbit MVI pattern via `ContainerHost<State, SideEffect>` in ViewModels
- `intent { ... }` blocks for actions; `reduce { state.copy(...) }` for state updates
- `postSideEffect(...)` for one-shot UI events
- `StateFlow` with `SharingStarted.WhileSubscribed(5_000L)` for lifecycle-aware exposure

**app (Usque VPN):**
- Plain `AndroidViewModel` with manual `MutableStateFlow` properties
- Private `_name` backing + public `name: StateFlow` read-only exposure
- Event-driven updates via `UsqueVpnService.events` collected in ViewModel `init`

---

## Coroutines

- `withContext(ioDispatcher)` wraps all blocking/IO work
- `supervisorScope` used for parallel launches where one failure should not cancel siblings
- Injected `CoroutineDispatcher` (never hardcoded `Dispatchers.IO` in android/app — qualifiers used)
- Exception: `app/` uses `Dispatchers.IO` directly (no DI)
- `applicationScope` for long-running coroutines outside ViewModel lifecycle
- `ensureActive()` called inside long loops to respect cancellation

---

## Function Design

**Size:** Functions generally under 40 lines; complex logic delegated to handler classes
**Parameters:** Named parameters used at call sites for multi-parameter constructors
**Return Values:**
- Suspend functions returning `Result<Unit>` for tunnel operations
- `Unit` returns for fire-and-forget
- Nullable returns (`TunnelConfig?`, `TunnelStatistics?`) instead of exceptions for "not found"

---

## Module Design

**android/app:**
- Interfaces defined in `domain/repository/` — implementations in `data/repository/`
- Koin DI wires implementations to interfaces: `singleOf(::WireGuardNotification) bind NotificationManager::class`
- Handler classes (e.g., `TunnelMonitorHandler`, `DynamicDnsHandler`) receive all dependencies via constructor — no service locator pattern inside classes

**app (Usque VPN):**
- No DI framework; direct instantiation in ViewModel: `private val prefs = VpnPreferences(application)`
- `companion object` used for constants and static factory fields (`ACTION_STOP`, `ACTION_RESTART`, `events` SharedFlow)

---

## Comments

**When to Comment:**
- Inline comments on non-obvious behavior: `// Keep current state — avoid UI flicker during brief disconnect`
- KDoc on public interfaces and key API methods
- `TODO` comments for known limitations: `// TODO this can crash if we haven't started foreground service yet`
- Migration notes in DataStore prefs: `// legacy, for migration`

**KDoc/TSDoc:**
- Single-line KDoc `/** Called from composable LaunchedEffect — checks volatile booleans, no JNI. */` used on ViewModel public functions

---

*Convention analysis: 2026-04-01*
