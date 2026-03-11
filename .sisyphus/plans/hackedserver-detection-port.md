# HackedServer Detection Port - Gate Go Plugin

## TL;DR
> **Quick Summary**: Implement full HackedServer client/mod detection in `plugins/detection/` for Gate, preserving behavior parity while using HackedServer submodule TOML files as the live source of truth.
>
> **Deliverables**:
> - `plugins/detection/` plugin implementation (generic, forge, lunar, bedrock, actions, commands)
> - TOML config loader reading from `HackedServer/hackedserver-core/src/main/resources/*.toml`
> - Lunar protobuf decoding support
> - Commands: `/detection reload`, `/detection check <player>`, `/detection list`
> - TDD test suite with integration coverage
>
> **Estimated Effort**: Large
> **Parallel Execution**: YES (5 waves)
> **Critical Path**: 1 -> 3 -> 5 -> 6 -> 10 -> 14 -> 16 -> FINAL

---

## Context

### Original Request
- Add `HackedServer` as git submodule (done).
- Port all Client Detection + Mod Intelligence features into Gate plugin.

### Interview Summary
- Config strategy changed: **do not port TOML files**; use submodule TOMLs directly.
- Bedrock: brand-string fallback only.
- Commands: reload/check/list only; no `inv`.
- Coverage: ALL 30+ signatures.
- Test strategy: TDD.
- Plugin name: `detection`.

### Metis Review
Metis timed out earlier; self-review guardrails applied directly:
- Use submodule TOML source-of-truth only.
- Use `PlayerModInfoEvent` as primary Forge source; raw parser fallback.
- Require explicit startup failure when submodule/config files are missing.

---

## Work Objectives

### Core Objective
Build a Go-native Gate plugin that reproduces HackedServer detection behavior with parity on signatures, mod intelligence, and action triggering.

### Concrete Deliverables
- `plugins/detection/plugin.go`
- `plugins/detection/config.go`
- `plugins/detection/config_toml.go`
- `plugins/detection/player.go`
- `plugins/detection/generic.go`
- `plugins/detection/forge.go`
- `plugins/detection/lunar.go`
- `plugins/detection/lunar_proto.go`
- `plugins/detection/bedrock.go`
- `plugins/detection/actions.go`
- `plugins/detection/commands.go`
- `plugins/detection/proto/*`
- `plugins/detection/*_test.go`
- `gate.go` plugin registration update

### Definition of Done
- [x] `go build ./...` passes
- [x] `go test ./plugins/detection/...` passes
- [x] `go vet ./plugins/detection/...` passes
- [x] All required detections/actions/commands implemented

### Must Have
- Full generic signature support (30+).
- Forge/NeoForge detection with blacklist/whitelist actions.
- Lunar handshake parsing and mod intelligence.
- Bedrock brand fallback detection.
- Automated action system with delay support.
- Commands: reload/check/list.
- TOML loaded from submodule resources directly.

### Must NOT Have
- No `/detection inv`.
- No Geyser/Floodgate Java API integration.
- No copied YAML mirror configs.
- No silent fallback if submodule TOMLs are missing.

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: YES
- **Automated tests**: TDD
- **Framework**: `go test`

### QA Policy
- Evidence path convention: `.sisyphus/evidence/task-{N}-{slug}.txt`
- Every task includes at least one happy path and one failure path scenario.

---

## Execution Strategy

### Parallel Execution Waves
```
Wave 1:
  T1 Plugin skeleton + registration
  T2 Config structs + TOML mapping
  T3 Player state model
  T4 Submodule TOML path resolver

Wave 2:
  T5 Generic matcher core
  T6 Brand/channel handlers
  T7 Forge/NeoForge detection
  T8 Lunar protobuf definitions + decoder
  T9 Bedrock brand fallback

Wave 3:
  T10 Lunar intelligence processor
  T11 Actions system
  T12 Plugin message routing (lunar:apollo)

Wave 4:
  T13 Commands (reload/check/list)
  T14 Full init wiring + lifecycle hooks
  T15 Submodule config startup guard

Wave 5:
  T16 Integration tests
  T17 Build/vet/final verification

Wave FINAL:
  F1 Plan compliance audit
  F2 Code quality review
  F3 QA evidence sweep
  F4 Scope fidelity check
```

### Dependency Matrix
- T1 -> T6,T7,T9,T10,T12,T13,T14
- T2 -> T4,T5,T6,T7,T9,T10,T11,T13,T14,T15
- T3 -> T5,T6,T7,T9,T10,T11,T12,T13
- T4 -> T15
- T5 -> T6,T12
- T6 -> T14,T16
- T7 -> T14,T16
- T8 -> T10
- T9 -> T14,T16
- T10 -> T12,T14,T16
- T11 -> T14,T16
- T12 -> T14,T16
- T13 -> T14,T16
- T14 -> T16
- T15 -> T16
- T16 -> T17
- T17 -> F1,F2,F3,F4

---

## TODOs

- [x] 1. Plugin Skeleton + Registration
  **What to do**: Add `detection.Plugin` and register in `gate.go`.
  **References**: `plugins/pelican/pelican.go`, `gate.go`.
  **QA Scenarios**:
  ```
  Scenario: Plugin compiles
    Tool: Bash
    Steps: go build ./...
    Expected Result: exit 0
    Evidence: .sisyphus/evidence/task-1-build.txt

  Scenario: Plugin test baseline
    Tool: Bash
    Steps: go test ./plugins/detection/... -run TestPlugin
    Expected Result: pass
    Evidence: .sisyphus/evidence/task-1-test.txt
  ```

- [x] 2. Config Structs + TOML Mapping
  **What to do**: Build config structs matching all six source TOMLs and loader entrypoints.
  **References**: all `HackedServer/.../resources/*.toml`.
  **QA Scenarios**:
  ```
  Scenario: TOML mapping succeeds
    Tool: Bash
    Steps: go test ./plugins/detection/... -run TestConfigTomlMapping
    Expected Result: pass
    Evidence: .sisyphus/evidence/task-2-mapping.txt

  Scenario: Missing file returns explicit error
    Tool: Bash
    Steps: go test ./plugins/detection/... -run TestConfigMissingFile
    Expected Result: pass
    Evidence: .sisyphus/evidence/task-2-missing.txt
  ```

- [x] 3. Player State Model
  **What to do**: Implement thread-safe `DetectedPlayer` + store.
  **References**: `HackedPlayer.java`, `HackedServer.java`.
  **QA Scenarios**:
  ```
  Scenario: Concurrency safety
    Tool: Bash
    Steps: go test ./plugins/detection/... -run TestPlayerConcurrency -race
    Expected Result: no race
    Evidence: .sisyphus/evidence/task-3-race.txt

  Scenario: Store lifecycle
    Tool: Bash
    Steps: go test ./plugins/detection/... -run TestPlayerStore
    Expected Result: pass
    Evidence: .sisyphus/evidence/task-3-store.txt
  ```

- [x] 4. Submodule TOML Path Resolver
  **What to do**: Resolve and validate all submodule TOML paths.
  **References**: `.gitmodules`, `HackedServer/.../resources`.
  **QA Scenarios**:
  ```
  Scenario: Paths resolve
    Tool: Bash
    Steps: go test ./plugins/detection/... -run TestResolveSubmodulePaths
    Expected Result: pass
    Evidence: .sisyphus/evidence/task-4-paths.txt

  Scenario: Missing submodule guidance
    Tool: Bash
    Steps: go test ./plugins/detection/... -run TestResolveSubmodulePathsMissing
    Expected Result: error includes git submodule init command
    Evidence: .sisyphus/evidence/task-4-missing.txt
  ```

- [x] 5. Generic Matcher Core
  **What to do**: Port Java `GenericCheck.pass()` logic with dedupe semantics.
  **References**: `GenericCheck.java`, `generic.toml`.
  **QA Scenarios**:
  ```
  Scenario: Known signature fixtures
    Tool: Bash
    Steps: go test ./plugins/detection/... -run TestGenericMatcherKnown
    Expected Result: pass
    Evidence: .sisyphus/evidence/task-5-known.txt

  Scenario: Duplicate suppression
    Tool: Bash
    Steps: go test ./plugins/detection/... -run TestGenericMatcherDedupe
    Expected Result: pass
    Evidence: .sisyphus/evidence/task-5-dedupe.txt
  ```

- [x] 6. Brand + Channel Handlers
  **What to do**: Hook `PlayerClientBrandEvent` and `PlayerChannelRegisterEvent` to matcher + state.
  **References**: `CustomPayloadListener.java`, Gate event APIs.
  **QA Scenarios**:
  ```
  Scenario: Brand detection path
    Tool: Bash
    Steps: go test ./plugins/detection/... -run TestBrandHandler
    Expected Result: pass
    Evidence: .sisyphus/evidence/task-6-brand.txt

  Scenario: Bypass permission skip
    Tool: Bash
    Steps: go test ./plugins/detection/... -run TestBypassSkip
    Expected Result: pass
    Evidence: .sisyphus/evidence/task-6-bypass.txt
  ```

- [x] 7. Forge/NeoForge Detection
  **What to do**: Use `PlayerModInfoEvent` primary + REGISTER parser fallback; apply list policies.
  **References**: `ForgeChannelParser.java`, `ForgeHandshakeProcessor.java`, `forge.toml`.
  **QA Scenarios**:
  ```
  Scenario: Blacklist trigger
    Tool: Bash
    Steps: go test ./plugins/detection/... -run TestForgeBlacklist
    Expected Result: pass
    Evidence: .sisyphus/evidence/task-7-blacklist.txt

  Scenario: Parser fallback
    Tool: Bash
    Steps: go test ./plugins/detection/... -run TestForgeRegisterFallback
    Expected Result: pass
    Evidence: .sisyphus/evidence/task-7-fallback.txt
  ```

- [x] 8. Lunar Protobuf Decoder
  **What to do**:
  - Pin proto dependency to `com.lunarclient:apollo-protos:0.0.5` (from `HackedServer/build.gradle.kts:95`).
  - Use configured repo source `https://repo.lunarclient.dev` (from `HackedServer/build.gradle.kts:35-37`).
  - Acquire protos with reproducible command:
    `curl -L -o .sisyphus/tmp/apollo-protos-0.0.5-sources.jar https://repo.lunarclient.dev/releases/com/lunarclient/apollo-protos/0.0.5/apollo-protos-0.0.5-sources.jar`
  - If sources jar is unavailable, fallback to extracting protobuf descriptors from main artifact and reconstruct required `.proto` files from documented schema package names.
  - Vendor required `.proto` files into `plugins/detection/proto/` so builds/tests do not require network access.
  - Generate Go code using explicit command:
    `protoc --go_out=. --go_opt=paths=source_relative plugins/detection/proto/*.proto`
  - Implement decoder wrappers in `lunar_proto.go` using generated types only.
  **References**:
  - `HackedServer/hackedserver-core/src/main/java/org/hackedserver/core/lunar/LunarApolloHandshakeParser.java`
  - Lunar Apollo upstream proto source (pin exact commit in implementation notes/evidence)
  **QA Scenarios**:
  ```
  Scenario: Valid payload decode
    Tool: Bash
    Steps: go test ./plugins/detection/... -run TestLunarDecodeValid
    Expected Result: pass
    Evidence: .sisyphus/evidence/task-8-valid.txt

  Scenario: Invalid payload safety
    Tool: Bash
    Steps: go test ./plugins/detection/... -run TestLunarDecodeInvalid
    Expected Result: pass
    Evidence: .sisyphus/evidence/task-8-invalid.txt

  Scenario: Offline codegen and compile validation
    Tool: Bash
    Steps:
      1. protoc --go_out=. --go_opt=paths=source_relative plugins/detection/proto/*.proto
      2. go build ./plugins/detection/...
    Expected Result: codegen artifacts exist locally and build passes without fetching protos
    Evidence: .sisyphus/evidence/task-8-offline-codegen.txt
  ```

- [x] 9. Bedrock Brand Fallback
  **What to do**: Detect Bedrock via brand string rules only.
  **References**: `BedrockDetector.java`, `bedrock.toml`.
  **QA Scenarios**:
  ```
  Scenario: Positive bedrock brand
    Tool: Bash
    Steps: go test ./plugins/detection/... -run TestBedrockBrandPositive
    Expected Result: pass
    Evidence: .sisyphus/evidence/task-9-positive.txt

  Scenario: Negative brand
    Tool: Bash
    Steps: go test ./plugins/detection/... -run TestBedrockBrandNegative
    Expected Result: pass
    Evidence: .sisyphus/evidence/task-9-negative.txt
  ```

- [x] 10. Lunar Intelligence Processor
  **What to do**: Map decoded lunar mods/marks to detections and queued actions.
  **References**: `LunarHandshakeProcessor.java`, `lunar.toml`.
  **QA Scenarios**:
  ```
  Scenario: Blacklisted lunar mod
    Tool: Bash
    Steps: go test ./plugins/detection/... -run TestLunarBlacklist
    Expected Result: pass
    Evidence: .sisyphus/evidence/task-10-blacklist.txt

  Scenario: Non-blacklisted mod
    Tool: Bash
    Steps: go test ./plugins/detection/... -run TestLunarNoBlacklist
    Expected Result: pass
    Evidence: .sisyphus/evidence/task-10-negative.txt
  ```

- [x] 11. Actions System
  **What to do**: Alerts/commands/delay execution pipeline with placeholders.
  **References**: `Action.java`, `actions.toml`.
  **QA Scenarios**:
  ```
  Scenario: Alert + command execution
    Tool: Bash
    Steps: go test ./plugins/detection/... -run TestActionsExecute
    Expected Result: pass
    Evidence: .sisyphus/evidence/task-11-exec.txt

  Scenario: Delay behavior
    Tool: Bash
    Steps: go test ./plugins/detection/... -run TestActionsDelay
    Expected Result: pass
    Evidence: .sisyphus/evidence/task-11-delay.txt
  ```

- [x] 12. PluginMessage Routing
  **What to do**: Register `lunar:apollo` and route plugin messages to lunar decode path.
  **References**: Gate plugin message API, `CustomPayloadListener.java`.
  **QA Scenarios**:
  ```
  Scenario: lunar channel route
    Tool: Bash
    Steps: go test ./plugins/detection/... -run TestPluginMessageLunarRoute
    Expected Result: pass
    Evidence: .sisyphus/evidence/task-12-lunar.txt

  Scenario: other channels ignored
    Tool: Bash
    Steps: go test ./plugins/detection/... -run TestPluginMessageIgnoreOther
    Expected Result: pass
    Evidence: .sisyphus/evidence/task-12-ignore.txt
  ```

- [x] 13. Commands (reload/check/list)
  **What to do**: Brigodier command tree with output formatting.
  **References**: `go.minekube.com/brigodier`.
  **QA Scenarios**:
  ```
  Scenario: Reload command applies TOML changes
    Tool: Bash
    Steps: go test ./plugins/detection/... -run TestCommandReload
    Expected Result: pass
    Evidence: .sisyphus/evidence/task-13-reload.txt

  Scenario: Check/list outputs
    Tool: Bash
    Steps: go test ./plugins/detection/... -run TestCommandCheckList
    Expected Result: pass
    Evidence: .sisyphus/evidence/task-13-checklist.txt
  ```

- [x] 14. Full Init Wiring + Lifecycle
  **What to do**: Wire all handlers, subscriptions, and disconnect cleanup.
  **References**: `plugins/pelican/pelican.go`.
  **QA Scenarios**:
  ```
  Scenario: init wiring complete
    Tool: Bash
    Steps: go test ./plugins/detection/... -run TestInitWiring
    Expected Result: pass
    Evidence: .sisyphus/evidence/task-14-init.txt

  Scenario: disconnect cleanup
    Tool: Bash
    Steps: go test ./plugins/detection/... -run TestDisconnectCleanup
    Expected Result: pass
    Evidence: .sisyphus/evidence/task-14-cleanup.txt
  ```

- [x] 15. Startup Guard for Submodule TOMLs
  **What to do**: Fail startup with remediation text when submodule TOMLs are unavailable.
  **References**: `.gitmodules`, submodule resource paths.
  **QA Scenarios**:
  ```
  Scenario: startup passes with submodule
    Tool: Bash
    Steps: go test ./plugins/detection/... -run TestStartupGuardPresent
    Expected Result: pass
    Evidence: .sisyphus/evidence/task-15-present.txt

  Scenario: startup fails with clear guidance when missing
    Tool: Bash
    Steps: go test ./plugins/detection/... -run TestStartupGuardMissing
    Expected Result: pass
    Evidence: .sisyphus/evidence/task-15-missing.txt
  ```

- [x] 16. Integration Test Wave
  **What to do**: Add integration tests spanning detections -> actions -> command-visible state.
  **References**: all TOML source files as fixtures.
  **QA Scenarios**:
  ```
  Scenario: full end-to-end flow
    Tool: Bash
    Steps: go test ./plugins/detection/... -run TestIntegrationE2E
    Expected Result: pass
    Evidence: .sisyphus/evidence/task-16-e2e.txt

  Scenario: malformed data path is safe
    Tool: Bash
    Steps: go test ./plugins/detection/... -run TestIntegrationErrorPaths
    Expected Result: pass
    Evidence: .sisyphus/evidence/task-16-errors.txt
  ```

- [x] 17. Build/Vet Finalization
  **What to do**: Final clean pass on build/test/vet/race.
  **QA Scenarios**:
  ```
  Scenario: build+test+vet
    Tool: Bash
    Steps:
      1. go build ./...
      2. go test ./plugins/detection/... -v
      3. go vet ./plugins/detection/...
    Expected Result: all pass
    Evidence: .sisyphus/evidence/task-17-main.txt

  Scenario: race safety
    Tool: Bash
    Steps: go test ./plugins/detection/... -race
    Expected Result: no race
    Evidence: .sisyphus/evidence/task-17-race.txt
  ```

---

## Final Verification Wave

- [x] F1. Plan Compliance Audit (`oracle`)
  **QA Scenario:**
  ```
  Scenario: F1 must-have/must-not-have audit
    Tool: Bash
    Steps:
      1. test -f plugins/detection/plugin.go
      2. test -f plugins/detection/generic.go
      3. test -f plugins/detection/forge.go
      4. test -f plugins/detection/lunar.go
      5. test -f plugins/detection/bedrock.go
      6. test -f plugins/detection/actions.go
      7. test -f plugins/detection/commands.go
      8. grep -R "/detection inv" -n plugins/detection && exit 1 || true
      9. grep -R "\.yml" -n plugins/detection && exit 1 || true
    Expected Result: required files exist; forbidden patterns absent; command exits 0
    Evidence: .sisyphus/evidence/task-f1-plan-compliance.txt
  ```

- [x] F2. Code Quality Review (`unspecified-high`)
  **QA Scenario:**
  ```
  Scenario: F2 build/test/vet quality gate
    Tool: Bash
    Steps:
      1. go build ./...
      2. go test ./plugins/detection/... -v
      3. go vet ./plugins/detection/...
    Expected Result: all commands exit 0 with no unresolved issues
    Evidence: .sisyphus/evidence/task-f2-quality.txt
  ```

- [x] F3. QA Evidence Sweep (`unspecified-high`)
  **QA Scenario:**
  ```
  Scenario: F3 evidence completeness check
    Tool: Bash
    Steps:
      1. test -f .sisyphus/evidence/task-1-build.txt
      2. test -f .sisyphus/evidence/task-1-test.txt
      3. test -f .sisyphus/evidence/task-2-mapping.txt
      4. test -f .sisyphus/evidence/task-2-missing.txt
      5. test -f .sisyphus/evidence/task-3-race.txt
      6. test -f .sisyphus/evidence/task-3-store.txt
      7. test -f .sisyphus/evidence/task-4-paths.txt
      8. test -f .sisyphus/evidence/task-4-missing.txt
      9. test -f .sisyphus/evidence/task-5-known.txt
      10. test -f .sisyphus/evidence/task-5-dedupe.txt
      11. test -f .sisyphus/evidence/task-6-brand.txt
      12. test -f .sisyphus/evidence/task-6-bypass.txt
      13. test -f .sisyphus/evidence/task-7-blacklist.txt
      14. test -f .sisyphus/evidence/task-7-fallback.txt
      15. test -f .sisyphus/evidence/task-8-valid.txt
      16. test -f .sisyphus/evidence/task-8-invalid.txt
      17. test -f .sisyphus/evidence/task-8-offline-codegen.txt
      18. test -f .sisyphus/evidence/task-9-positive.txt
      19. test -f .sisyphus/evidence/task-9-negative.txt
      20. test -f .sisyphus/evidence/task-10-blacklist.txt
      21. test -f .sisyphus/evidence/task-10-negative.txt
      22. test -f .sisyphus/evidence/task-11-exec.txt
      23. test -f .sisyphus/evidence/task-11-delay.txt
      24. test -f .sisyphus/evidence/task-12-lunar.txt
      25. test -f .sisyphus/evidence/task-12-ignore.txt
      26. test -f .sisyphus/evidence/task-13-reload.txt
      27. test -f .sisyphus/evidence/task-13-checklist.txt
      28. test -f .sisyphus/evidence/task-14-init.txt
      29. test -f .sisyphus/evidence/task-14-cleanup.txt
      30. test -f .sisyphus/evidence/task-15-present.txt
      31. test -f .sisyphus/evidence/task-15-missing.txt
      32. test -f .sisyphus/evidence/task-16-e2e.txt
      33. test -f .sisyphus/evidence/task-16-errors.txt
      34. test -f .sisyphus/evidence/task-17-main.txt
      35. test -f .sisyphus/evidence/task-17-race.txt
    Expected Result: all test -f checks pass; exit 0
    Evidence: .sisyphus/evidence/task-f3-evidence-sweep.txt
  ```

- [x] F4. Scope Fidelity Check (`deep`)
  **QA Scenario:**
  ```
  Scenario: F4 planned-vs-implemented scope diff
    Tool: Bash
    Steps:
      1. git diff --name-only -- plugins/detection gate.go > .sisyphus/evidence/task-f4-changed-files.txt
      2. grep -R "/detection inv" -n plugins/detection && exit 1 || true
      3. grep -R "Geyser\|Floodgate" -n plugins/detection && exit 1 || true
    Expected Result: changed files limited to planned scope; forbidden features absent; exit 0
    Evidence: .sisyphus/evidence/task-f4-scope-fidelity.txt
  ```

---

## Commit Strategy

- T1: `feat(detection): scaffold plugin and register in gate`
- T2-T4: `feat(detection): add submodule toml config loading`
- T5-T6: `feat(detection): implement generic detection engine`
- T7: `feat(detection): implement forge and neoforge detection`
- T8-T10: `feat(detection): add lunar mod intelligence`
- T9: `feat(detection): add bedrock brand fallback`
- T11: `feat(detection): add automated actions pipeline`
- T12-T14: `feat(detection): wire handlers and lifecycle`
- T13: `feat(detection): implement detection admin commands`
- T15: `feat(detection): add startup submodule guard`
- T16-T17: `test(detection): integration and final verification`

---

## Success Criteria

### Verification Commands
```bash
go build ./...
go test ./plugins/detection/... -v
go vet ./plugins/detection/...
go test ./plugins/detection/... -race
```

### Final Checklist
- [x] Must Have items implemented
- [x] Must NOT Have items absent
- [x] QA evidence files present for all tasks
- [x] Final verification wave approved (F1-F4)
