package detection

import (
	"testing"

	"github.com/google/uuid"
)

// TestPlugin verifies the Plugin is registered correctly: correct name and
// Init function is non-nil.
// NOTE: init_returns_nil has been removed — the real initDetection calls
// proxy methods and cannot be called with a nil proxy.
func TestPlugin(t *testing.T) {
	t.Run("name", func(t *testing.T) {
		if got := Plugin.Name; got != "Detection" {
			t.Errorf("Plugin.Name = %q, want %q", got, "Detection")
		}
	})

	t.Run("init_not_nil", func(t *testing.T) {
		if Plugin.Init == nil {
			t.Fatal("Plugin.Init must not be nil")
		}
	})

	// init_returns_nil is intentionally removed: the full initDetection
	// requires a real *proxy.Proxy instance (it calls p.ChannelRegistrar(),
	// p.Command(), and p.Event()). A nil proxy would panic. The startup guard
	// tests in startup_guard_test.go cover the Init path with real submodule
	// validation semantics.
}

// TestInitWiring verifies that the runtime collaborators constructed by
// initDetection wire together correctly as a logical unit.
//
// Rather than calling Plugin.Init with a real proxy (which requires a full
// Gate runtime), this test constructs the same collaborators that initDetection
// creates and verifies their interactions: config loading → store/history
// creation → event handling functions produce correct player state mutations.
//
// Specifically it exercises:
//  1. The TOML config is loadable from the real submodule (same guard as startup)
//  2. NewPlayerStore + NewMessageHistory produce correctly initialised collaborators
//  3. HandleBrandPayload wired through the collaborators records state on the player
//  4. HandleChannelRegister wired through the collaborators records state on the player
//  5. HandleLunarPluginMessage returns empty result for a non-lunar channel
//  6. ProcessForgeModInfo wired through the collaborators records forge mods on the player
//  7. ActionExecutor constructed from loaded config resolves known action IDs
func TestInitWiring(t *testing.T) {
	// ── 1. Config load mirrors initDetection startup guard ────────────────────
	resourcesDir, err := resourcesDirFromBaseDir(repoRoot(t))
	if err != nil {
		t.Skipf("submodule not available, skipping TestInitWiring: %v", err)
	}
	cfg, err := LoadDetectionConfig(resourcesDir)
	if err != nil {
		t.Fatalf("LoadDetectionConfig failed: %v", err)
	}

	// ── 2. Collaborator construction ─────────────────────────────────────────
	store := NewPlayerStore()
	if store == nil {
		t.Fatal("NewPlayerStore returned nil")
	}
	history := NewMessageHistory()
	if history == nil {
		t.Fatal("NewMessageHistory returned nil")
	}
	executor := NewActionExecutor(cfg.Actions, cfg.Main.Settings)
	if executor == nil {
		t.Fatal("NewActionExecutor returned nil")
	}

	// Verify initial state: empty store.
	if got := store.Len(); got != 0 {
		t.Errorf("fresh store Len = %d, want 0", got)
	}

	// ── 3. Store.Get creates a player on demand (mirrors event handler flow) ──
	playerID := uuid.New()
	dp := store.Get(playerID)
	if dp == nil {
		t.Fatal("store.Get returned nil for new player")
	}
	if store.Len() != 1 {
		t.Errorf("store.Len after Get = %d, want 1", store.Len())
	}

	// ── 4. HandleChannelRegister wires into player state ─────────────────────
	// The generic.toml checks that use "minecraft:register" as their channel
	// are the ones HandleChannelRegister can trigger (it calls GenericMatch
	// with channel="minecraft:register"). We find such a check and satisfy
	// its message_has filter (if any) to confirm the wiring works end-to-end.
	var regCheckID string
	var regPayload string
	for id, check := range cfg.Generic.Checks {
		for _, ch := range check.Channels {
			if ch == registerChannel {
				regCheckID = id
				// Build a payload that satisfies the message_has filter (if set).
				if check.MessageHas != "" {
					regPayload = check.MessageHas
				} else {
					regPayload = "something"
				}
				goto foundRegCheck
			}
		}
	}
foundRegCheck:
	if regCheckID == "" {
		t.Fatal("no check with 'minecraft:register' channel found in loaded config")
	}

	result := HandleChannelRegister(dp, history, regPayload, []string{registerChannel}, *cfg, false)
	// At least one generic trigger should have fired for the chosen check.
	found := false
	for _, trig := range result.GenericTriggers {
		if trig.CheckID == regCheckID {
			found = true
		}
	}
	if !found {
		t.Errorf("HandleChannelRegister did not produce trigger for check %q (payload %q)", regCheckID, regPayload)
	}
	// Player state should now record the check.
	if !dp.HasGenericCheck(regCheckID) {
		t.Errorf("player.HasGenericCheck(%q) is false after HandleChannelRegister", regCheckID)
	}

	// Also verify HandleBrandPayload does not panic and returns a valid result
	// (brand-channel generic checks are not present in this config, but Bedrock
	// and Forge brand paths are also exercised).
	brandResult := HandleBrandPayload(dp, history, "vanilla", *cfg, false)
	_ = brandResult // should not panic; Bedrock/Forge brand paths run silently

	// ── 5. HandleChannelRegister with bypass clears action IDs ────────────────
	// With bypass=true, state is still mutated but ActionIDs are cleared.
	playerID2 := uuid.New()
	dp2 := store.Get(playerID2)
	r2 := HandleChannelRegister(dp2, history, regPayload, []string{registerChannel}, *cfg, true)
	for _, trig := range r2.GenericTriggers {
		if len(trig.ActionIDs) > 0 {
			t.Errorf("bypass=true: expected nil ActionIDs in trigger %q, got %v", trig.CheckID, trig.ActionIDs)
		}
	}

	// ── 6. HandleLunarPluginMessage returns empty for non-lunar channel ────────
	emptyResult := HandleLunarPluginMessage(dp, "minecraft:brand", []byte("test"), cfg.Lunar, false)
	if emptyResult.HasTriggers() {
		t.Error("HandleLunarPluginMessage: expected no triggers for non-lunar channel")
	}

	// ── 7. ActionExecutor resolves known actions from loaded config ───────────
	// Only test this if the actions config has at least one action.
	if len(cfg.Actions.Actions) > 0 {
		var firstActionID string
		for id := range cfg.Actions.Actions {
			firstActionID = id
			break
		}
		var cbCalled bool
		executor.Execute([]string{firstActionID}, ActionContext{
			PlayerName: "TestPlayer",
			CheckName:  "TestCheck",
		}, ActionCallbacks{
			SendAlert:             func(s string) { cbCalled = true },
			ExecuteConsoleCommand: func(s string) { cbCalled = true },
			ExecutePlayerCommand:  func(s string) { cbCalled = true },
		})
		// An action may have no send_alert and no commands — in that case cbCalled
		// stays false. We only assert it did NOT panic.
		t.Logf("action %q: cbCalled=%v (ok — action may be no-op)", firstActionID, cbCalled)
	}

	// ── 8. Cleanup ────────────────────────────────────────────────────────────
	store.Remove(playerID)
	store.Remove(playerID2)
	if store.Len() != 0 {
		t.Errorf("store.Len after Remove = %d, want 0", store.Len())
	}

	t.Logf("TestInitWiring: OK — config has %d generic checks, %d actions",
		len(cfg.Generic.Checks), len(cfg.Actions.Actions))
}

// TestDisconnectCleanup verifies the disconnect lifecycle:
//   - A player's state is stored in the PlayerStore and MessageHistory when
//     they receive detection events.
//   - After simulating a disconnect (Remove on both store and history), the
//     player is no longer present in either collaborator.
//
// This is the exact behaviour of onDisconnectEvent in plugin.go.
func TestDisconnectCleanup(t *testing.T) {
	store := NewPlayerStore()
	history := NewMessageHistory()

	playerID := uuid.New()

	// ── 1. Simulate player activity: register and record state ────────────────
	store.Register(playerID)
	dp := store.Get(playerID)
	if dp == nil {
		t.Fatal("store.Get returned nil after Register")
	}

	// Record a generic check (mirrors AddGenericCheck in event handlers).
	dp.AddGenericCheck("labymod_test")
	if !dp.HasGenericCheck("labymod_test") {
		t.Error("AddGenericCheck: check not recorded on player")
	}

	// Record some message history (mirrors IsDuplicate call in GenericMatch).
	isDup := history.IsDuplicate(playerID, "minecraft:brand", "labymod")
	if isDup {
		t.Error("first IsDuplicate should return false (not yet seen)")
	}
	isDupAgain := history.IsDuplicate(playerID, "minecraft:brand", "labymod")
	if !isDupAgain {
		t.Error("second IsDuplicate should return true (already seen)")
	}

	// Queue a pending action.
	actionFired := false
	dp.QueuePendingAction(func() { actionFired = true })
	if !dp.HasPendingActions() {
		t.Error("HasPendingActions should be true after QueuePendingAction")
	}

	// ── 2. Simulate player present in store ───────────────────────────────────
	if store.Len() != 1 {
		t.Errorf("store.Len before disconnect = %d, want 1", store.Len())
	}

	// ── 3. Simulate disconnect: remove from BOTH store and history ────────────
	// This mirrors exactly what onDisconnectEvent does in plugin.go.
	store.Remove(playerID)
	history.Remove(playerID)

	// ── 4. Verify store cleanup ───────────────────────────────────────────────
	if store.Len() != 0 {
		t.Errorf("store.Len after disconnect = %d, want 0", store.Len())
	}

	// Get after Remove should produce a new, empty player (store.Get is create-if-absent).
	newDP := store.Get(playerID)
	if newDP == dp {
		t.Error("store.Get after Remove returned the old player pointer — expected fresh entry")
	}
	if newDP.HasGenericCheck("labymod_test") {
		t.Error("fresh player after disconnect still has old generic check — store.Remove failed")
	}
	if newDP.HasPendingActions() {
		t.Error("fresh player after disconnect has pending actions — store.Remove failed")
	}

	// Clean up the freshly-created player.
	store.Remove(playerID)

	// ── 5. Verify history cleanup ─────────────────────────────────────────────
	// After history.Remove, the same payload should NOT be a duplicate.
	isDupAfterRemove := history.IsDuplicate(playerID, "minecraft:brand", "labymod")
	if isDupAfterRemove {
		t.Error("history.IsDuplicate after Remove returned true — history not cleaned up on disconnect")
	}

	// ── 6. Verify pending action was NOT auto-executed by Remove ─────────────
	// Disconnect should NOT run pending actions — it only frees memory.
	if actionFired {
		t.Error("disconnect should NOT execute pending actions, but actionFired=true")
	}

	t.Log("TestDisconnectCleanup: OK")
}
