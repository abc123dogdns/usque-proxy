# Testing Patterns

**Analysis Date:** 2026-04-01

## Test Framework

**Runner:**
- JUnit 4 (`org.junit.Test`, `org.junit.Assert.*`)
- Config: No separate `junit.xml` or test config files — standard Android test source sets

**Assertion Library:**
- JUnit 4 `Assert.*` (`assertEquals`, etc.)

**Android Test Runner:**
- `androidx.test.runner.AndroidJUnitRunner` — configured via `testInstrumentationRunner` in all `build.gradle.kts` files

**Run Commands:**
```bash
# Run unit tests (JVM)
./gradlew test

# Run instrumented tests (device/emulator)
./gradlew connectedAndroidTest

# Run specific module tests
./gradlew :app:test
./gradlew :android:app:test
./gradlew :android:networkmonitor:test
```

---

## Test File Organization

**Location:** Tests are in separate source sets, NOT co-located with source files

**Structure:**
```
app/
  src/
    test/java/com/nhubaotruong/usqueproxy/          # JVM unit tests
    androidTest/java/com/nhubaotruong/usqueproxy/   # Instrumented tests

android/app/
  src/
    test/java/com/zaneschepke/wireguardautotunnel/          # JVM unit tests
    androidTest/java/com/zaneschepke/wireguardautotunnel/   # Instrumented tests

android/networkmonitor/
  src/
    test/java/com/zaneschepke/networkmonitor/
    androidTest/java/com/zaneschepke/networkmonitor/

android/logcatter/
  src/
    test/java/com/zaneschepke/
    androidTest/java/com/zaneschepke/
```

**Naming:**
- Example/placeholder files: `ExampleUnitTest.kt`, `ExampleInstrumentedTest.kt`
- Functional tests named after feature: `MigrationTest.kt`

---

## Test Structure

**Suite Organization:**

JVM unit test (placeholder pattern — all test modules):
```kotlin
class ExampleUnitTest {
    @Test
    fun addition_isCorrect() {
        assertEquals(4, 2 + 2)
    }
}
```

Instrumented test (Room migration pattern — `android/app/src/androidTest/`):
```kotlin
@RunWith(AndroidJUnit4::class)
class MigrationTest {
    private val dbName = "migration-test"

    @get:Rule
    val helper: MigrationTestHelper =
        MigrationTestHelper(InstrumentationRegistry.getInstrumentation(), AppDatabase::class.java)

    @Test
    @Throws(IOException::class)
    fun migrate6To7() {
        helper.createDatabase(dbName, 6).apply {
            close()
        }
        helper.runMigrationsAndValidate(dbName, 7, true)
    }
}
```

**Patterns:**
- `@RunWith(AndroidJUnit4::class)` on all instrumented test classes
- `@get:Rule` for JUnit Rules (e.g., `MigrationTestHelper`)
- `@Throws(IOException::class)` on tests that perform file/DB operations
- No `@Before`/`@After` setup patterns observed in existing tests

---

## Mocking

**Framework:** None detected — no mockito, mockk, or turbine dependencies in any `build.gradle.kts`

**Current approach:** Tests are minimal placeholders; no mocking infrastructure exists

**What to Mock (when tests are added):**
- `TunnelRepository`, `GeneralSettingRepository`, and other domain repository interfaces — they are defined as interfaces in `domain/repository/` making them mockable
- `NetworkMonitor` interface (`android/networkmonitor/`) — interface-based design supports mocking
- `TunnelBackend` interface — `android/app/src/main/java/.../core/tunnel/backend/TunnelBackend.kt`
- `ServiceManager` — constructor injection pattern makes it substitutable

**What NOT to Mock:**
- `TunnelConfig`, `VpnPrefs`, `TunnelStats`, `AutoTunnelSettings` — plain data classes with no side effects
- `StringValue` — pure sealed class with no dependencies

---

## Fixtures and Factories

**Test Data:** No dedicated fixture or factory classes exist

**Current pattern:** Direct instantiation with default values:
```kotlin
// Data classes all have defaults — instantiate directly in tests
val prefs = VpnPrefs()          // all defaults
val settings = GeneralSettings() // all defaults
val stats = TunnelStats()        // all defaults
```

**Location:** No dedicated `testFixtures/` directory — inline creation only

---

## Coverage

**Requirements:** None enforced — no coverage thresholds configured in any `build.gradle.kts`

**View Coverage:**
```bash
./gradlew test jacocoTestReport
# No jacocoTestReport task configured; would need to be added
```

---

## Test Types

**Unit Tests (JVM):**
- Location: `src/test/java/`
- Scope: Currently only placeholder `ExampleUnitTest` in all four modules
- No real business logic is unit tested

**Integration Tests (Instrumented):**
- Location: `src/androidTest/java/`
- Scope: One real test exists: `MigrationTest` at `android/app/src/androidTest/java/com/zaneschepke/wireguardautotunnel/MigrationTest.kt`
- Tests Room database schema migration from version 6 to 7 using `MigrationTestHelper`
- Uses schema files stored in `android/app/schemas/com.zaneschepke.wireguardautotunnel.data.AppDatabase/`

**E2E Tests:** Not used — no Espresso or Compose UI test rules invoked beyond boilerplate

---

## Testing Dependencies

**app/build.gradle.kts:**
```kotlin
testImplementation(libs.junit)
androidTestImplementation(libs.androidx.junit)
androidTestImplementation(libs.androidx.espresso.core)
androidTestImplementation(platform(libs.androidx.compose.bom))
androidTestImplementation(libs.androidx.compose.ui.test.junit4)
```

**android/app/build.gradle.kts:**
```kotlin
testImplementation(libs.junit)
testImplementation(libs.androidx.junit)
androidTestImplementation(libs.androidx.junit)
androidTestImplementation(libs.androidx.espresso.core)
androidTestImplementation(platform(libs.androidx.compose.bom))
androidTestImplementation(libs.androidx.compose.ui.test)
androidTestImplementation(libs.androidx.room.testing)   // Enables MigrationTestHelper
```

---

## Common Patterns

**Async Testing:** Not established — no `runTest`, `TestCoroutineDispatcher`, or turbine usage observed

**Error Testing:** Not established — no error path tests exist

**State Flow Testing:** Not established — no `StateFlow` tests despite heavy use of flows in production code

---

## Key Testing Gaps

The codebase has minimal test coverage. All four modules have only placeholder unit tests. The single real test (`MigrationTest`) covers only DB schema migration.

**High-value areas with no tests:**
- `VpnViewModel` (`app/src/main/java/.../ui/viewmodel/VpnViewModel.kt`) — state transitions, `connect()`/`disconnect()` logic
- `VpnPreferences` (`app/src/main/java/.../data/VpnPreferences.kt`) — migration logic from legacy keys
- `TunnelManager` (`android/app/src/main/java/.../core/tunnel/TunnelManager.kt`) — mode switching, restore, reboot handling
- `TunnelMonitorHandler` (`android/app/src/main/java/.../core/tunnel/handler/TunnelMonitoringHandler.kt`) — ping/health logic
- Domain repository interfaces — all lack backing test doubles

---

*Testing analysis: 2026-04-01*
