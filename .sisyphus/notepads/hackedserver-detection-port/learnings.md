# Learnings — HackedServer Detection Port

## 2026-03-11 Session Bootstrap

### Project Layout
- Module: `github.com/minekube/gate-plugin-template`
- Go 1.24.1, toolchain go1.24.6
- Branch: `feat/detection`
- Worktree: `C:/Users/fdelelis/Desktop/gate-plugged`
- Plugin package target: `plugins/detection/`

### Plugin Registration Pattern (from pelican)
```go
// gate.go pattern
proxy.Plugins = append(proxy.Plugins, pelican.Plugin)

// plugin.go pattern
var Plugin = proxy.Plugin{
    Name: "PluginName",
    Init: func(ctx context.Context, p *proxy.Proxy) error {
        log := logr.FromContextOrDiscard(ctx)
        // subscribe events, load config, register commands
        event.Subscribe(p.Event(), 0, handlerFunc)
        return nil
    },
}
```

### Gate Event APIs Available
- `*proxy.PlayerClientBrandEvent` → `.Brand() string`, `.Player() proxy.Player`
- `*proxy.PlayerChannelRegisterEvent` → `.Channels() []message.ChannelIdentifier`, `.Player() proxy.Player`
- `*proxy.PlayerModInfoEvent` → `.ModInfo() modinfo.ModInfo` (Forge native!)
- `*proxy.PluginMessageEvent` → `.Identifier().ID() string`, `.Data() []byte`, `.Source()`, `.Target()`
- `*proxy.DisconnectEvent` → `.Player() proxy.Player`
- MUST register custom channels: `p.ChannelRegistrar().Register(id)`
- Channel ID creation: `message.ChannelIdentifierFrom("lunar:apollo")`

### Available Dependencies (already in go.mod)
- `github.com/pelletier/go-toml/v2 v2.2.4` — TOML parsing (currently indirect)
- `go.minekube.com/brigodier v0.0.2` — Commands (currently indirect)
- `google.golang.org/protobuf v1.36.11` — Protobuf decode (currently indirect)
- `github.com/go-logr/logr v1.4.3` — Logging (direct)
- `github.com/robinbraemer/event v0.1.1` — Event subscriptions (direct)
- `github.com/google/uuid v1.6.0` — UUID (indirect)

### TOML Config Files (submodule source of truth)
All at `HackedServer/hackedserver-core/src/main/resources/`:
- `config.toml` — global settings (debug, skip_duplicates, action_delay_ticks)
- `generic.toml` — 30 detection signatures
- `actions.toml` — action definitions (alert, send_alert, commands, delay_ticks)
- `forge.toml` — forge/neoforge settings, blacklist/whitelist mods
- `lunar.toml` — lunar client settings, mod actions
- `bedrock.toml` — bedrock detection settings

### HackedPlayer Java → Go Mapping
```go
type DetectedPlayer struct {
    UUID            uuid.UUID
    mu              sync.RWMutex
    genericChecks   map[string]struct{}       // set of triggered check IDs
    lunarMods       map[string]LunarModInfo   // lowercase ID → info
    lunarModsKnown  bool
    forgeMods       map[string]ForgeModInfo   // lowercase ID → info
    forgeModsKnown  bool
    forgeClientType *ForgeClientType          // nil, FORGE, or NEOFORGE
    bedrockDetected bool
    pendingActions  []func()                  // run at first server join
}
```

### GenericCheck.pass() Logic (Java → Go)
```
pass(player, channel, message):
  if channel not in check.Channels → false
  if messageHas != "" && !strings.Contains(toLower(msg), toLower(messageHas)) → false
  if messageNotHas != "" && strings.Contains(toLower(msg), toLower(messageNotHas)) → false
  if skip_duplicates && isDuplicate(player, channel, message) → false
  return true
```

### Action Model
```go
type Action struct {
    ID               string
    SendAlert        string   // MiniMessage format, placeholders: <player>, <name>
    ConsoleCommands  []string
    PlayerCommands   []string
    OppedPlayerCommands []string
    DelayTicks       int64    // -1 = use global
}
```

### Forge TOML Structure
- `enabled`, `settings.mark_forge`, `settings.mark_neoforge`
- `settings.show_mods_in_check`, `settings.show_mod_versions`
- `actions.forge`, `actions.neoforge` — string[] action IDs
- `mod_actions` — map[modID][]string
- `category.whitelisted.mods`, `category.blacklisted.mods`

### Lunar TOML Structure  
- `enabled`, `settings.mark_lunar_client`, `settings.mark_fabric`, `settings.mark_forge`
- `settings.show_mods_in_check`, `settings.show_mod_versions`, `settings.show_mod_types`
- `actions.lunar_client`, `actions.fabric`, `actions.forge` — string[] action IDs
- `mod_actions` — map[modID][]string

### Bedrock TOML Structure
- `enabled` (false by default — brand fallback only)
- `label` — "Bedrock"
- `actions` — string[] action IDs

### MUST NOT
- No /detection inv
- No Geyser/Floodgate Java API
- No YAML config mirrors
- No silent startup if submodule missing

## 2026-03-11 Task 1 — Plugin Skeleton + Registration

### Files Created
- `plugins/detection/plugin.go` — `package detection`, `var Plugin = proxy.Plugin{Name: "Detection", Init: ...}`
- `plugins/detection/plugin_test.go` — `package detection_test`, `TestPlugin` with subtests: name, init_not_nil, init_returns_nil

### gate.go Registration Pattern Applied
```go
proxy.Plugins = append(proxy.Plugins, pelican.Plugin)
proxy.Plugins = append(proxy.Plugins, detection.Plugin)
```
Import added: `"github.com/minekube/gate-plugin-template/plugins/detection"`

### Test Approach: external test package (`detection_test`)
Using `package detection_test` (not `package detection`) mirrors standard Go convention and allows testing only the exported API. `Plugin.Init` is safely called with `nil` proxy because the scaffold has no event subscriptions yet.

### Build/Test Results
- `go build ./...` → exit 0, no output (clean)
- `go test ./plugins/detection/... -run TestPlugin -v` → PASS (3 subtests), exit 0

### LSP note
LSP reports errors in Gate's own temp-dir source files (internal package access violations) — these pre-exist in the module cache and do NOT affect our build/test. Confirmed via `go build` exit 0.

## 2026-03-11 Task 4 — Submodule TOML Path Resolver

### Package Bootstrap
- `plugins/detection/` package created fresh (Tasks 1-3 not yet done at time of T4).
- Package compiles standalone with no Gate imports — resolver only uses `os` and `path/filepath`.

### Resolver Pattern
- `TOMLPaths` struct with one field per TOML file (Config, Generic, Actions, Forge, Lunar, Bedrock).
- `ResolveSubmodulePaths(baseDir string) (TOMLPaths, error)` — takes repo root, derives path via `filepath.Join`.
- Two-phase validation: directory existence check first (gives targeted submodule error), then per-file readability.
- No fallback, no silent continue.

### Testing Pattern
- `repoRoot(t)` helper uses `runtime.Caller(0)` to walk up 3 dirs from test file to repo root.
  This is portable — works on any machine regardless of working directory.
- Three test scenarios: happy path (real submodule), missing directory, existing directory with missing files.
- All three pass: `ok github.com/minekube/gate-plugin-template/plugins/detection 2.447s`

### Error Message Contract
- Always includes: `git submodule init` and `git submodule update --recursive`.
- Always mentions `HackedServer` path in the message.
- Validated by string containment checks in tests.

## 2026-03-11 Task 2 — Config Structs + TOML Mapping

### Config Struct Design
- `DetectionConfig` is the aggregate of all six sub-configs.
- `MainConfig` wraps a `[settings]` table; all six TOML shapes are faithfully mapped.
- `GenericConfig.Checks` and `ActionsConfig.Actions` use `map[string]T` because their TOML keys are free-form check/action IDs at the top level.
- `ActionDef.DelayTicks` is `*int64` (pointer) so absence of the key maps to `nil`, distinct from explicit `0`. This is critical for the "use global" vs "explicit zero" semantic.

### TOML Decode Strategy for Flat-Key Files
- `generic.toml` and `actions.toml` have free-form top-level keys mixed with reserved keys (`enabled`).
- Strategy: decode into `map[string]interface{}`, iterate keys, re-marshal each value to TOML bytes, then unmarshal into typed struct. This avoids custom UnmarshalTOML and keeps the code simple.
- `go-toml/v2` marshal of `interface{}` preserves inner maps correctly.

### Path Handling
- `config_toml.go` uses a simple `baseDir + "/" + name + ".toml"` join (forward-slash only).
- On Windows this means paths passed to `LoadDetectionConfig` must use forward slashes (or the caller can normalize). This is intentional: the path resolver (Task 4) handles OS-specific joining before passing the base dir.
- Tests use `repoRoot(t)` (already defined in `config_toml_paths_test.go`) + the `submoduleResourcesDir` constant.

### go-toml/v2 Version
- `github.com/pelletier/go-toml/v2 v2.2.4` — already present as indirect dep, no new dep added.

### Test Coverage Achieved
- 11 sub-tests in `TestConfigTomlMapping` covering all 6 TOMLs, field values, check count, message_has/not_has, action placeholders, forge colors, lunar marks, bedrock defaults.
- `TestConfigMissingFile` verifies explicit path + submodule hint in error.

## 2026-03-11 Task 3 — Player State Model

### Files Created
- `plugins/detection/player.go` — `DetectedPlayer`, `PlayerStore`, plus value types `ForgeClientType`, `LunarModInfo`, `ForgeModInfo`
- `plugins/detection/player_test.go` — `TestPlayerConcurrency`, `TestPlayerStore`, and 9 additional semantic unit tests

### Type Declarations Consolidated in player.go
Rather than a separate `types.go`, all value types that downstream tasks will need (ForgeClientType, LunarModInfo, ForgeModInfo) are declared in player.go. This keeps the player model self-contained and avoids circular import pressure for future files.

### Concurrency Design: Single RWMutex
Java's HackedPlayer used per-field `synchronized` blocks on the map objects plus `volatile` for booleans. In Go, all mutable state (genericChecks, lunarMods, forgeMods, forgeClientType, bedrockDetected, pendingActions) is guarded by a single `sync.RWMutex` on DetectedPlayer. This:
- Eliminates lock ordering issues
- Provides correct volatile semantics for all fields
- Minimizes overhead (reads use RLock, writes use Lock)
- Keeps the code simple and auditable

### pending Actions: Slice Not Queue
Java used `ConcurrentLinkedQueue<Runnable>`. Go stdlib has no concurrent queue. We use a `[]func()` slice protected by the same RWMutex. ExecutePendingActions atomically swaps the slice to nil and runs actions outside the lock — allowing actions to safely call back into the player (e.g. QueuePendingAction inside an action).

### PlayerStore: Double-Checked Locking for Get
`Get(id)` uses read-lock first (fast path), then upgrades to write-lock if not found (slow path), re-checking after acquiring the write lock to handle concurrent Gets for the same UUID. This is the standard Go pattern for computeIfAbsent and avoids a write lock on every read.

### Register Preserves Existing (Critical Semantic)
`Register(id)` does NOT overwrite an existing player. This mirrors HackedServer's `computeIfAbsent(uuid, HackedPlayer::new)` comment: "avoids replacing an existing player that may have been created by getPlayer() during packet handling, which would lose any pending actions that were queued." Test `RegisterPreservesExisting` validates this explicitly.

### Race Detector Note
`go test -race` requires CGO which requires a C compiler. This Windows dev machine has CGO_ENABLED=0 and no gcc/mingw installed. Tests pass without -race. For -race verification, run on Linux CI with a C toolchain.

### Test Results
- `go test ./plugins/detection/... -run TestPlayerConcurrency -v` → PASS (6 tests), exit 0
- `go test ./plugins/detection/... -run TestPlayerStore -v` → PASS (8 subtests), exit 0
- `go build ./plugins/detection/...` → exit 0
- `go vet ./plugins/detection/...` → exit 0

## 2026-03-11 Task 9 — Bedrock Brand Fallback

### Files Created
- `plugins/detection/bedrock.go` — `IsBedrock(brand string) bool`, `ApplyBedrockBrand(player, brand, cfg) (detected, label, actionIDs)`
- `plugins/detection/bedrock_test.go` — `TestBedrockBrandPositive`, `TestBedrockBrandNegative`, `TestApplyBedrockBrandPositive`, `TestApplyBedrockBrandNegative`

### Detection Logic
- `bedrockBrandTokens = []string{"geyser"}` — the only brand token.
- `IsBedrock`: case-insensitive `strings.Contains(strings.ToLower(brand), token)` across all tokens. Empty brand → false (fast path).
- `ApplyBedrockBrand`: checks `cfg.Enabled` first (returns false without mutation if disabled), calls `IsBedrock`, then calls `player.SetBedrockDetected(true)` and returns `cfg.Label`, `cfg.Actions`.

### Brand Token Decision
- Only "geyser" needed — this is the brand that Geyser bridge clients send.
- Matching is contains (not prefix/exact) to handle version suffixes like "Geyser v2.2.0".

### Test Coverage (24 sub-tests)
- Positive: exact_lowercase, exact_uppercase, mixed_case, prefix_version, prefix_bedrock, suffix_geyser, contains_geyser
- Negative: empty, vanilla, fabric, forge, lunar_client, labymod, badlion, optifine, random_string, partial_mismatch ("geys")
- ApplyBedrockBrand positive: exact_geyser, geyser_version, upper_geyser
- ApplyBedrockBrand negative: non_bedrock_brand, disabled_config, empty_brand, empty_actions_config (default bedrock.toml state)

### Test Results
- `go test ./plugins/detection/... -run TestBedrockBrandPositive -v` → PASS (7 sub-tests), exit 0
- `go test ./plugins/detection/... -run TestBedrockBrandNegative -v` → PASS (10 sub-tests), exit 0
- `go test ./plugins/detection/... -v` (full suite, 50+ tests) → PASS, exit 0

### No Dependencies Added
- Only imports: `strings` (stdlib), `testing` + `github.com/google/uuid` (existing)

## 2026-03-11 Task 5 — Generic Matcher Core

### Files Created
- `plugins/detection/generic.go` — `MessageHistory`, `GenericMatchInput`, `GenericMatch()`, `channelMatches()`
- `plugins/detection/generic_test.go` — `TestGenericMatcherKnown` (32 subtests), `TestGenericMatcherDedupe` (7 subtests)

### API Design
- `GenericMatch(in GenericMatchInput) bool` — pure-ish function accepting a struct input.
  Choosing a struct over positional args makes future extension (e.g. adding a logger) zero-cost for callers.
- `MessageHistory` is a standalone struct, not embedded in PlayerStore. This allows Task 6's event handler
  to hold it separately from the store and call `history.Remove(uuid)` on disconnect.
- `messagePayload` is an unexported value-type struct used as a map key — mirrors Java's `MessagePayload`
  equals/hashCode contract (channel + message equality, case-sensitive).

### Logic Parity with Java GenericCheck.pass()
1. Channel check: exact case-sensitive match (`List.contains()` in Java is case-sensitive).
2. message_has: `strings.Contains(toLower(msg), toLower(has))` — case-insensitive, mirrors Java.
3. message_not_has: same case-insensitive contains, negated.
4. skip_duplicates: `history.IsDuplicate()` mutates history (records new payload) on first call —
   mirrors Java's `Set.add()` return semantics.

### Dedupe Semantics (Java → Go mapping)
Java `HackedServer.isMessageDuplicate`:
  `Set<MessagePayload>.add(payload)` → returns false if already present → `isMessageDuplicate = true`
Go `MessageHistory.IsDuplicate`:
  `map[messagePayload]struct{}` write-with-check → returns `seen` bool (true = already present = duplicate)

### Test Coverage
- `TestGenericMatcherKnown`: 32 subtests covering all 30 generic.toml fixture signatures
  (channel-only, message_has, message_not_has, combined forge/neoforge exclusion, multi-channel,
  case-sensitivity in channels, empty channels list).
- `TestGenericMatcherDedupe`: 7 subtests covering first-true/repeat-false, different-message not
  suppressed, different-channel not suppressed, dedupe-off, per-player isolation, nil history safety,
  Remove() clears history.

### Test Results
- `go test ./plugins/detection/... -run TestGenericMatcherKnown` → PASS (32 subtests), exit 0
- `go test ./plugins/detection/... -run TestGenericMatcherDedupe` → PASS (7 subtests), exit 0
- `go build ./...` → exit 0

## 2026-03-11 Task 7 — Forge/NeoForge Detection

### Files Created
- `plugins/detection/forge.go` — ForgeActionTrigger, ParseClientTypeFromBrand, ParseModsFromChannels,
  ParseModsFromRegisterPayload, ProcessForgeModInfo, ProcessClientType, ProcessMods,
  IsForgeBlacklisted, IsForgeWhitelisted
- `plugins/detection/forge_test.go` — TestForgeBlacklist (11 subtests), TestForgeRegisterFallback (23 subtests)

### Gate modinfo import path
`go.minekube.com/gate/pkg/edition/java/forge/modinfo` — ModInfo{Type string, Mods []Mod{ID, Version}}

### FML type string mapping
- "FML", "FML2", "FML3" (strings.HasPrefix "FML") → ForgeClientForge
- "NEOFORGE" → ForgeClientNeoForge
- Unknown type → ForgeClientForge (safe fallback)

### Builtin namespace exclusions for REGISTER channel parsing
Excluded from mod inference: minecraft, neoforge, forge, fml, c, fabric
Prefix-excluded: fabric-*, fabricloader*
NOT excluded: "fabrication" (a mod, not a Fabric API module)

### Action trigger pattern
ProcessForgeModInfo/ProcessMods/ProcessClientType return []ForgeActionTrigger{Name, ActionIDs}
Callers (Task 14) queue these as QueuePendingAction closures — not executed inline.

### Test Results
- `go test ./plugins/detection/... -run TestForgeBlacklist` → PASS (11 subtests), exit 0
- `go test ./plugins/detection/... -run TestForgeRegisterFallback` → PASS (23 subtests), exit 0
- `go test ./plugins/detection/...` → all tests PASS, no regressions, exit 0
- `go build ./plugins/detection/...` → exit 0

## 2026-03-11 — T15 fix + integration_test.go repair

### integration_test.go: use fakeCommandContext+newMockSource, not custom mock types
- The broken first version defined `recordingSource` + `mockCommandContext` with a
  `commandSource` interface that doesn't match `*command.Context` parameter types.
- Correct pattern: `src := newMockSource(perms...)` + `ctx := fakeCommandContext(src)`.
  Both helpers live in `commands_test.go` in the same `package detection` and are
  already used by all other command tests.
- `formatCheckOutput(ctx *command.Context, ...)` and `handleListFromEntries(ctx *command.Context, ...)`
  require `*command.Context`, not a custom wrapper.

### LSP ghost file: lunar_pb/lunarclient/apollo/player/v1/player.pb.go
- The LSP reports a duplicate-type conflict involving `player.pb.go`, but this file
  does not exist on disk (confirmed by `find` and `git ls-files`).
- `go build` and `go vet` both exit 0 — no actual compiler conflict.
- Root cause: stale LSP cache. Ignore these LSP diagnostics; they are not real errors.

## 2026-03-11 — T14 Full Init Wiring + Lifecycle

### plugin_test.go: internal vs external test package
- `plugin_test.go` must be `package detection` (internal), NOT `package detection_test`.
- Reason: `resourcesDirFromBaseDir` and `repoRoot()` are unexported/internal helpers.
  `repoRoot()` is already defined in `config_toml_paths_test.go` (same package).
  External test packages cannot access these without exporting them — avoid exporting
  test-only helpers.
- `detection.RepoRootForTest` and `detection.ResourcesDirFromBaseDir` are anti-patterns:
  they export internal helpers just to satisfy an external test package.

### generic.toml channel semantics: register vs. brand channels
- Checks with `channels = ["LABYMOD"]`, `["LMC"]`, `["WECUI"]` etc. are triggered when
  a `PluginMessageEvent` arrives with that specific channel ID (not via register packet).
- `HandleChannelRegister` calls `GenericMatch` with `channel = "minecraft:register"`.
  Only checks whose `.Channels` list contains `"minecraft:register"` (e.g. `forge_mod_loader_v1`)
  will fire via `HandleChannelRegister`.
- Checks with `channels = ["MC|Brand", "minecraft:brand"]` fire via `HandleBrandPayload`
  only if `brand` contains their `message_has` substring.
- TestInitWiring must search for a check with `"minecraft:register"` in its channels,
  then pass a `rawPayload` that satisfies its `message_has` filter (e.g. `"legacy:fml"`).

### onServerPostConnectEvent: first-join guard
- `e.PreviousServer() == nil` is the correct Gate API check for first server join.
- Pending actions queued during brand/channel/mod-info events are only executed on
  first join, not on server switches.

### buildCallbacks: stub implementation acceptable for Task 14
- Real alert broadcast and command execution (routing to all online staff, running
  proxy commands) is wired in a later task. For Task 14, stubs that log via logr
  are sufficient — they satisfy the `ActionCallbacks` interface and keep the build clean.

## 2026-03-11 — Task 16 Integration Test Wave

### State of integration_test.go on entry
- `integration_test.go` was already fully implemented from the T15 repair session.
- Both `TestIntegrationE2E` and `TestIntegrationErrorPaths` were present and correct.
- Task 16 verified, ran tests, wrote evidence files. No code changes were needed.

### TestIntegrationE2E coverage
- Loads real submodule TOMLs via `resourcesDirFromBaseDir(repoRoot(t))` — skips if submodule absent.
- Scans `cfg.Generic.Checks` for the first channel-only check (no message_has filter) to stay
  config-agnostic. On this machine it picks up `cracked_vape` (channel LOLIMAHCKER) or similar.
- Exercises full pipeline: GenericMatch → AddGenericCheck → ProcessForgeModInfo → ProcessLunarHandshake
  → ApplyBedrockBrand → ActionExecutor(ImmediateClock) → formatCheckOutput → handleListFromEntries.
- forge triggers=0 and lunar triggers=0 in output is expected: the real forge.toml/lunar.toml
  don't have mark_ flags configured to fire for the synthetic test inputs.

### TestIntegrationErrorPaths coverage (12 subtests)
- nil player safety for GenericMatch, Forge, Lunar, Bedrock processors
- empty/malformed payloads (empty brand, empty channels, null-byte payload of builtins only)
- unknown action ID silently skipped
- MessageHistory.Remove clears history correctly
- PlayerStore double-Register preserves existing player pointer + pending actions
- handleListFromEntries(nil) → "No players..." message
- RenderPlaceholders identity when no placeholders present

### Evidence files written
- `.sisyphus/evidence/task-16-e2e.txt` — full test output + pipeline coverage description
- `.sisyphus/evidence/task-16-errors.txt` — full test output + error path coverage description

### Full package suite: no regressions
- `go test ./plugins/detection/...` → PASS, exit 0
- All ~120+ tests across all sub-test groups pass

## 2026-03-11 — Task 17 Build/Vet Finalization

### Final Verification Results
- `go build ./...` → exit 0, no output (clean build)
- `go test ./plugins/detection/... -v` → PASS, exit 0, all 130+ subtests pass
- `go vet ./plugins/detection/...` → exit 0, no issues
- `go test ./plugins/detection/... -race` → ENVIRONMENT BLOCKED (exit 2)
  stderr: "go: -race requires cgo; enable cgo by setting CGO_ENABLED=1"
  Classification: environment limitation, not a test failure.

### Evidence Files
- `.sisyphus/evidence/task-17-main.txt` — build/test/vet results
- `.sisyphus/evidence/task-17-race.txt` — race result + classification

### LSP Ghost File Note (Confirmed Non-Issue)
LSP diagnostics show redeclaration errors in `player.pb.go` (lunar_pb package).
`go build` and `go vet` both exit 0, confirming these are stale LSP cache entries.
The file does not exist as a real source conflict for the compiler.

## 2026-03-11 — F4 Scope Fidelity Gate

### Strict grep rule can fail on benign textual references
- F4 plan uses `grep -R -n "Geyser\|Floodgate" -n plugins/detection && exit 1 || true`.
- This catches non-integration occurrences too (comments/tests mentioning `Geyser`), not only API integrations.
- Under strict plan semantics, any such match triggers REJECT even when code explicitly avoids Java API integration.

## 2026-03-11 F1 Plan Compliance Audit
- Verdict: APPROVE
- Evidence: .sisyphus/evidence/task-f1-plan-compliance.txt

