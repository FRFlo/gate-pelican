# Decisions — HackedServer Detection Port

## 2026-03-11 Session Bootstrap

### Config Format: TOML from submodule (no YAML mirror)
Read directly from `HackedServer/hackedserver-core/src/main/resources/*.toml`.
Reload = re-read from same submodule paths.

### Bedrock Detection: Brand fallback only
No Geyser/Floodgate Java API. Use brand string ("Geyser") only via PlayerClientBrandEvent.

### Commands: reload/check/list only
No /detection inv (GUI-only, Spigot-specific).

### Test Strategy: TDD
Write tests first, then implement. Use go test -race for concurrency checks.

### Plugin Name: detection
Package: `plugins/detection/`
Plugin Name: "Detection"

## 2026-03-11 Task 4 — Path Resolver Design Choices

### baseDir injection (not runtime.Getwd)
- `ResolveSubmodulePaths` takes `baseDir` as parameter instead of calling `os.Getwd()` internally.
- Rationale: makes tests hermetic — they can pass a temp dir for missing-submodule scenario without mocking.
- Downstream consumers (startup guard in T15) will pass the actual repo root discovered at startup.

### filepath.FromSlash on the submodule constant
- `submoduleResourcesDir` uses forward-slash notation; `filepath.FromSlash` converts for Windows.
- Ensures cross-platform correctness.

### No os.IsExist — only os.IsNotExist + generic fallthrough
- Missing file → explicit remediation error.
- Other OS errors (permissions, etc.) → wrapped error with path context.

## 2026-03-11 Task 2 — Config Structs + TOML Mapping

### ActionDef.DelayTicks: pointer vs zero value
`*int64` chosen for `delay_ticks` so that the TOML key's absence (nil) is semantically distinct from an explicit `delay_ticks = 0`. The actions pipeline (Task 11) will check `if def.DelayTicks == nil { use global }` rather than comparing against -1 as a sentinel.

### Generic + Actions TOML: indirect decode
Rather than implementing `toml.Unmarshaler`, we decode into `map[string]interface{}` and then re-marshal each subtable. This avoids reflection hacks and stays idiomatic with `go-toml/v2`. Performance is irrelevant here (loaded once at startup).

### No filepath.Join in config_toml.go
`tomlPath()` uses string concatenation with "/" to keep things simple. The OS-aware filepath join lives in Task 4 (`config_toml_paths.go`). Tests pass the base dir using the same convention.

## 2026-03-11 Task 3 — Player State Model

### Single RWMutex over per-field locks
Chose a single `sync.RWMutex` on `DetectedPlayer` rather than per-field mutexes or per-map synchronized blocks (Java's approach). Rationale: simpler lock ordering (none needed), correct volatile semantics without `atomic`, no extra allocations per map, and the player is never on a hot concurrent path (only touched during login/detection).

### forgeClientType as *ForgeClientType (pointer)
Stores as `*ForgeClientType` to distinguish "not yet detected" (nil) from a detected value. Java used `null`. The exported `ForgeClientType() (ForgeClientType, bool)` API surfaces this via a boolean ok pattern rather than returning the zero value.

### Type declarations in player.go (not types.go)
Kept ForgeClientType, LunarModInfo, ForgeModInfo in player.go to keep the state model self-contained. This is the single file all future tasks will import to manipulate player state, so having the types co-located with the state reduces navigation overhead.

## 2026-03-11 Task 9 — Bedrock Brand Fallback

### Only "geyser" as brand token
- The brand string that Geyser bridge sends is "Geyser" (exact, sometimes with version suffix).
- No other bridge product uses a different identifier in the brand.
- Using a `bedrockBrandTokens` slice (not a hardcoded constant) makes it easy to add tokens in future without changing the matching logic.

### Contains-match, not prefix or exact
- Geyser sometimes sends "Geyser v2.2.0" or "Geyser-Bedrock", so prefix/exact would miss those.
- Contains on the lowercased brand is simple and covers all real-world variants observed in the Java codebase.

### ApplyBedrockBrand: Enabled guard first
- Check `cfg.Enabled` before `IsBedrock` to avoid any string work when the feature is disabled.
- This matches the Java pattern: detect-then-act, with early return if the detector is disabled.

### IsBedrock is a pure function (no player state)
- Keeping `IsBedrock` side-effect-free makes it independently unit-testable without needing a player object.
- `ApplyBedrockBrand` is the stateful layer that uses it, maintaining single-responsibility.

### Package-internal test (not detection_test)
- Tests use `package detection` (internal) to access `newDetectedPlayer` directly, matching the existing player_test.go convention.

## 2026-03-11 Task 5 — Generic Matcher Core

### GenericMatch as free function (not method)
Chose `GenericMatch(GenericMatchInput)` over a `Matcher` struct with a `Match` method. The function
is stateless w.r.t. config — it receives a `GenericCheck` value directly. This keeps it pure-ish
and trivially unit-testable without constructing a whole config tree.

### GenericMatchInput struct over positional args
The input struct is preferred because GenericCheck.pass() in Java takes 3 positional args, but the Go
port needs 2 more (history, skipDuplicates). A struct makes the call site self-documenting and
avoids argument confusion. Zero-cost to add fields later for Task 6 (e.g. logger).

### MessageHistory as standalone struct (not embedded in PlayerStore)
Java's `messageHistory` is a global static field on `HackedServer`. In Go, we keep it separate from
PlayerStore so the plugin Init function can hold both independently. This also keeps the player model
(Task 3) focused on player state and the message history focused on dedup state.

### IsDuplicate single-lock write (not read-then-write)
`IsDuplicate` always acquires a write lock because the common case on first call writes a new entry.
A read-then-upgrade pattern would be possible but adds complexity for a non-hot path.

## 2026-03-11 Task 7 — Forge/NeoForge Detection

### Pure functions (no event subscriptions)
forge.go exports only pure detection functions. Event subscriptions (Task 14) call these functions
and queue triggers. This keeps the detection logic testable without Gate proxy mocking.

### ForgeActionTrigger as data output
Returns []ForgeActionTrigger instead of executing actions directly. ActionIDs are looked up in
ActionsConfig.Actions at execution time (Task 11/14). Name field carries display context for
alert placeholders.

### Idempotency via HasGenericCheck guard
clientTypeTrigger checks HasGenericCheck("forge"|"neoforge") before triggering. Mirrors Java
ForgeHandshakeProcessor's hadForgeCheck local variable pattern. Prevents duplicate alerts on
repeated events.

### forgeClientTypeFromModInfoType: FML prefix catch-all
Matches "FML", "FML2", "FML3" (and any future FML* variants) via strings.HasPrefix. Unknown
types default to ForgeClientForge as a safe fallback — better to treat an unknown Forge variant
as Forge than to silently skip detection.

### Deduplication in ProcessMods
Builds a set of previously-known mod IDs from player.ForgeMods before processing new mods.
Prevents re-triggering action IDs for mods already recorded from a previous REGISTER event.

## 2026-03-11 — T14 Full Init Wiring + Lifecycle

### Decision: plugin_test.go as internal package (package detection)
- **Choice**: Use `package detection` (internal test package).
- **Rationale**: `repoRoot()` and `resourcesDirFromBaseDir()` already exist as internal
  helpers. Exporting them only to satisfy an external test package would pollute the
  public API with test-only symbols. Internal package gives direct access to all
  unexported helpers consistently with all other test files in the package.

### Decision: TestInitWiring uses HandleChannelRegister for generic-check assertion
- **Choice**: Find a check with `"minecraft:register"` in its channels, build a matching
  `rawPayload`, and call `HandleChannelRegister` to confirm trigger + player state.
- **Rationale**: `HandleBrandPayload` calls `GenericMatch` with `channel="minecraft:brand"`.
  The real `generic.toml` has ZERO checks with `"minecraft:brand"` as a bare channel
  without a `message_has` filter — so any brand-only assertion would always skip.
  `HandleChannelRegister` with `channel="minecraft:register"` matches `forge_mod_loader_v1`
  (has `message_has="legacy:fml"`), giving a reliable, config-driven assertion.
- `HandleBrandPayload("vanilla", ...)` is still called in the test as a no-panic smoke test.

### Decision: onDisconnectEvent removes from BOTH store AND history
- **Choice**: `store.Remove(id)` then `history.Remove(id)` in disconnect handler.
- **Rationale**: `PlayerStore` and `MessageHistory` are independent data structures with
  separate lifetimes. Both must be cleaned on disconnect to prevent unbounded memory growth.
  Confirmed by `TestDisconnectCleanup`: history entries survive store removal unless
  `history.Remove` is called explicitly.

## 2026-03-11 — F4 Scope Fidelity Verdict Decision

### Decision: apply strict plan grep semantics
- **Choice**: Use the exact F4 QA checks from plan for verdict, without relaxing pattern interpretation.
- **Rationale**: F4 is a scope gate; acceptance criteria are command-defined and evidence-driven.
- **Outcome**: REJECT because `grep -R -n "Geyser\|Floodgate" plugins/detection` returns matches.
