# External Integrations

**Analysis Date:** 2026-04-01

## APIs & External Services

**Cloudflare WARP / Zero Trust (core tunnel):**
- Service: Cloudflare WARP consumer and Zero Trust MASQUE endpoints
- Protocol: MASQUE over HTTP/3 over QUIC (RFC 9484 CONNECT-IP)
- Default SNI: `consumer-masque.cloudflareclient.com` (WARP), `zt-masque.cloudflareclient.com` (Zero Trust)
- Default Connect URI: `https://cloudflareaccess.com`
- SDK/Client: `github.com/Diniboy1123/usque v1.4.2` (Go), bridged to Android via `usquebind.aar`
- Auth: Device registration credentials stored as JSON in `VpnPreferences` (`warp_config_json` / `zt_config_json` DataStore keys)
- Config path: `usque-bind/bind.go` (`tunnelConfig`, `StartTunnel`)

**Microsoft Office 365 Endpoint API (optional split-tunnel bypass):**
- Service: `https://endpoints.office.com/endpoints/worldwide`
- Purpose: Fetch current O365 IP ranges to exclude from VPN routing
- Implementation: `app/src/main/java/com/nhubaotruong/usqueproxy/data/Office365Endpoints.kt`
- Client: Raw `HttpURLConnection` (no SDK)
- Auth: None (public API; fixed `clientrequestid` UUID hardcoded in source)
- Caching: 24-hour file cache at `context.filesDir/office365_ips.json`
- Behavior: Falls back to stale cache on network failure; never throws

## Data Storage

**Databases:**
- None (no SQLite, Room, or remote database)

**Preferences / Key-Value Store:**
- AndroidX DataStore Preferences (`datastore-preferences 1.2.1`)
- Store name: `vpn_prefs` (file on device internal storage)
- Implementation: `app/src/main/java/com/nhubaotruong/usqueproxy/data/VpnPreferences.kt`
- Keys: split mode, included/excluded app lists, DNS mode, DoH/DoQ URLs, WARP config JSON, Zero Trust config JSON, registration state, custom SNI, connect URI, auto-connect, theme mode
- Accessed reactively via `Flow<VpnPrefs>` collected in `VpnViewModel`

**File Storage:**
- Local filesystem only
- O365 IP cache: `context.filesDir/office365_ips.json`
- TLS session caches: in-memory (`tls.NewLRUClientSessionCache` in Go layer)

**Caching:**
- In-memory LRU DNS cache in Go layer (`doh.go`, `doq.go`, `cachedResolver`, capacity 1024 entries)
- TLS session cache: `globalTLSSessionCache` (32 entries) shared across DoH clients; `quicSessionCache` (8 entries) for tunnel QUIC connections

## Authentication & Identity

**Auth Provider: Cloudflare WARP API (self-registration)**
- Implementation: Device registers with Cloudflare WARP API (via `github.com/Diniboy1123/usque/api`) to obtain credentials
- Result stored as JSON blob in DataStore (`warp_config_json` or `zt_config_json`)
- Two profile types: `WARP` (consumer) and `ZERO_TRUST` (organizational)
- Registration driven from `VpnViewModel` (`app/src/main/java/com/nhubaotruong/usqueproxy/ui/viewmodel/VpnViewModel.kt`)
- Credentials include ECDSA keys and certificates generated client-side using `crypto/ecdsa` (Go)

**No third-party auth SDK** (no Firebase Auth, Google Sign-In, etc.)

## Monitoring & Observability

**Error Tracking:**
- None (no Crashlytics, Sentry, etc.)

**Logs:**
- Android: `android.util.Log` with tag `UsqueVpnService`, `Office365Endpoints`, etc.
- Go layer: standard `log` package; errors surfaced to Android via `Usquebind.GetLastError()` and the `lastError` atomic
- Tunnel statistics exposed via atomic counters polled by `VpnViewModel`: tx/rx bytes, tx/rx packets, connect count, delivery ratio, lifetime rotations, uptime

## CI/CD & Deployment

**Hosting:**
- GitHub Releases (APK artifacts)

**CI Pipeline:**
- GitHub Actions (`release.yml` in `.github/workflows/`)
- Trigger: push tags matching `v*` or manual `workflow_dispatch`
- Runner: `ubuntu-latest`
- Steps:
  1. Set up JDK 17 (Temurin), Go 1.24.2, Android SDK
  2. Install SDK packages: `platforms;android-36`, `platforms;android-31`, `build-tools;36.0.0`, `ndk;28.0.13004108`
  3. Install gomobile/gobind
  4. Run `build-usque.sh` to produce AAR
  5. Decode keystore from `KEYSTORE_BASE64` secret
  6. Build release APK (`./gradlew assembleRelease`)
  7. Upload APK to GitHub Release via `softprops/action-gh-release@v2`
- Dependabot: weekly updates for Gradle, Go modules, and GitHub Actions (`.github/dependabot.yml`)

## Webhooks & Callbacks

**Incoming:**
- None

**Outgoing:**
- None (VPN tunnel traffic is not webhook-based)

## DNS Resolvers (Configurable External Services)

The app supports four DNS modes, each routing DNS queries differently:

- `SYSTEM` - Uses device DNS, passthrough via VPN tunnel
- `CLOUDFLARE` - Uses Cloudflare DNS-over-HTTPS (`1.1.1.1/dns-query`) via Go DoH client
- `CUSTOM_DOH` - User-specified DoH URL; handled by `dohProxy` in `usque-bind/doh.go`
- `CUSTOM_DOQ` - User-specified DoQ address (default port 853); handled by `doqProxy` in `usque-bind/doq.go`

DNS interceptor runs 4 worker goroutines with a 256-request channel buffer. Configuration stored as `doh_url` / `doq_url` DataStore keys.

## Environment Configuration

**Required secrets (CI only):**
- `KEYSTORE_BASE64` - Base64-encoded keystore JKS for release signing
- `KEYSTORE_PASSWORD` - Keystore password
- `KEY_ALIAS` - Key alias within keystore
- `KEY_PASSWORD` - Key password

**Local development secrets:**
- `keystore.properties` (root of project, git-ignored) - Same four keystore fields

**No runtime secrets or API keys are required** - all Cloudflare communication uses device-registered credentials stored in DataStore.

---

*Integration audit: 2026-04-01*
