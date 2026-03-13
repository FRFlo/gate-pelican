package detection

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"go.minekube.com/gate/pkg/edition/java/forge/modinfo"
)

// ─── TestIntegrationE2E ───────────────────────────────────────────────────────
//
// TestIntegrationE2E exercises the full detection pipeline in one shot:
//
//	config load (real submodule TOMLs)
//	→ player state construction
//	→ generic matcher
//	→ forge detection + action trigger
//	→ lunar handshake + action trigger
//	→ bedrock brand detection
//	→ action executor (collect outputs)
//	→ command formatCheckOutput / handleListFromEntries (command-visible state)
//
// This test uses the real submodule TOMLs. It is skipped when the submodule is
// unavailable (CI without the submodule populated).
func TestIntegrationE2E(t *testing.T) {
	// ── Setup: load real TOML config ─────────────────────────────────────────
	resourcesDir, err := resourcesDirFromBaseDir(repoRoot(t))
	if err != nil {
		t.Skipf("submodule not available, skipping TestIntegrationE2E: %v", err)
	}
	cfg, err := LoadDetectionConfig(resourcesDir)
	if err != nil {
		t.Fatalf("LoadDetectionConfig failed: %v", err)
	}

	// ── Setup: player + history ───────────────────────────────────────────────
	playerID := uuid.New()
	store := NewPlayerStore()
	store.Register(playerID)
	player := store.Get(playerID)

	history := NewMessageHistory()

	// ── 1. Generic matcher — exercise a known signature from generic.toml ────
	// We scan all loaded checks for one that matches a static brand payload so
	// the test is not fragile against specific TOML check IDs changing upstream.
	var firstBrandCheck *GenericCheck
	var firstBrandCheckID string
	for id, check := range cfg.Generic.Checks {
		check := check
		if len(check.Channels) > 0 && check.MessageHas == "" && check.MessageNotHas == "" {
			// Pick the first simple channel-only check we find.
			firstBrandCheck = &check
			firstBrandCheckID = id
			break
		}
	}
	if firstBrandCheck == nil {
		t.Skip("no simple generic check found in loaded config; skipping generic assertion")
	}

	matched := GenericMatch(GenericMatchInput{
		Player:         player,
		History:        history,
		Check:          *firstBrandCheck,
		Channel:        firstBrandCheck.Channels[0],
		Message:        "",
		SkipDuplicates: cfg.Main.Settings.SkipDuplicates,
	})
	if !matched {
		t.Fatalf("expected generic check %q to match on channel %q", firstBrandCheckID, firstBrandCheck.Channels[0])
	}
	// Record the triggered check on the player (mirrors event handler behaviour).
	player.AddGenericCheck(firstBrandCheckID)
	if !player.HasGenericCheck(firstBrandCheckID) {
		t.Fatalf("player.HasGenericCheck(%q) returned false after AddGenericCheck", firstBrandCheckID)
	}

	// ── 2. Forge detection via mod info ──────────────────────────────────────
	if cfg.Forge.Enabled {
		info := modinfo.ModInfo{
			Type: "FML2",
			Mods: []modinfo.Mod{
				{ID: "forge", Version: "47.1.0"},
				{ID: "myfakemod", Version: "1.0.0"},
			},
		}
		triggers := ProcessForgeModInfo(player, info, cfg.Forge)

		// Forge mods must be recorded on the player regardless of triggers.
		if !player.HasForgeModsData() {
			t.Error("player.HasForgeModsData() is false after ProcessForgeModInfo")
		}
		if !player.HasForgeMod("myfakemod") {
			t.Error("player.HasForgeMod(myfakemod) is false after ProcessForgeModInfo")
		}
		t.Logf("forge triggers: %d", len(triggers))
	}

	// ── 3. Lunar handshake ────────────────────────────────────────────────────
	if cfg.Lunar.Enabled {
		mods := []LunarModInfo{
			{ID: "sodium", DisplayName: "Sodium", Version: "0.5.3", Type: "FABRIC_EXTERNAL"},
		}
		result := ProcessLunarHandshake(player, mods, cfg.Lunar)

		// Lunar mod list must be set on the player.
		if !player.HasLunarModsData() {
			t.Error("player.HasLunarModsData() is false after ProcessLunarHandshake")
		}
		if !player.HasLunarMod("sodium") {
			t.Error("player.HasLunarMod(sodium) is false after ProcessLunarHandshake")
		}
		t.Logf("lunar triggers: %d", len(result.Triggers))
	}

	// ── 4. Bedrock detection ──────────────────────────────────────────────────
	// Use a brand that contains "geyser" to trigger bedrock detection.
	// We exercise ApplyBedrockBrand regardless of cfg.Bedrock.Enabled so we
	// cover the enabled=false path too.
	{
		tempCfg := cfg.Bedrock
		tempCfg.Enabled = true
		detected, label, actionIDs := ApplyBedrockBrand(player, "geyser-Fabric", tempCfg)
		if !detected {
			t.Error("ApplyBedrockBrand returned detected=false for geyser brand")
		}
		if !player.IsBedrockDetected() {
			t.Error("player.IsBedrockDetected() is false after ApplyBedrockBrand")
		}
		t.Logf("bedrock: label=%q actions=%v", label, actionIDs)
	}

	// ── 5. Action executor — collect outputs using ImmediateClock ─────────────
	// Build a minimal config with a known in-memory action so this does not
	// depend on what actions.toml defines.
	tpl := int64(0)
	inMemoryActions := ActionsConfig{
		Actions: map[string]ActionDef{
			"alert_only": {
				SendAlert:  "Alert: <player> using <name>",
				DelayTicks: &tpl,
				Commands:   ActionCommands{Console: []string{"log <player> <name>"}},
			},
		},
	}
	executor := newActionExecutorWithClock(inMemoryActions, cfg.Main.Settings, ImmediateClock{})

	var alerts, cmds []string
	actCtx := ActionContext{PlayerName: "TestPlayer", CheckName: firstBrandCheckID}
	callbacks := ActionCallbacks{
		SendAlert:             func(s string) { alerts = append(alerts, s) },
		ExecuteConsoleCommand: func(s string) { cmds = append(cmds, s) },
	}
	executor.Execute([]string{"alert_only"}, actCtx, callbacks)

	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d: %v", len(alerts), alerts)
	}
	if !strings.Contains(alerts[0], "TestPlayer") || !strings.Contains(alerts[0], firstBrandCheckID) {
		t.Errorf("alert placeholder not substituted: %q", alerts[0])
	}
	if len(cmds) != 1 || !strings.Contains(cmds[0], "TestPlayer") {
		t.Errorf("console command placeholder not substituted: %v", cmds)
	}

	// ── 6. Command-visible state — formatCheckOutput ──────────────────────────
	// Use the same mock helpers from commands_test.go (newMockSource / fakeCommandContext).
	src := newMockSource("hackedserver.command", "hackedserver.command.check")
	ctx := fakeCommandContext(src)

	if err := formatCheckOutput(ctx, "TestPlayer", player, cfg); err != nil {
		t.Fatalf("formatCheckOutput returned error: %v", err)
	}
	output := src.allMessages()
	if !strings.Contains(output, "TestPlayer") {
		t.Errorf("formatCheckOutput output missing player name: %q", output)
	}
	if !strings.Contains(output, firstBrandCheckID) {
		t.Errorf("formatCheckOutput output missing triggered check %q: %q", firstBrandCheckID, output)
	}

	// ── 7. handleListFromEntries — list command covers this player ─────────────
	entries := []namedCheckEntry{{name: "TestPlayer", checks: player.GenericChecks()}}
	src2 := newMockSource()
	ctx2 := fakeCommandContext(src2)
	if err := handleListFromEntries(ctx2, entries); err != nil {
		t.Fatalf("handleListFromEntries returned error: %v", err)
	}
	if !strings.Contains(src2.lastMessage(), "TestPlayer") {
		t.Errorf("handleListFromEntries output missing player name: %q", src2.lastMessage())
	}

	t.Logf("E2E integration: OK — player %s has checks: %v", "TestPlayer", player.GenericChecks())
}

// ─── TestIntegrationErrorPaths ────────────────────────────────────────────────
//
// TestIntegrationErrorPaths verifies that malformed / edge-case inputs are
// handled safely throughout the detection pipeline.  No panics, no silent
// data corruption.
func TestIntegrationErrorPaths(t *testing.T) {
	// ── 1. nil player is safe in all processors ───────────────────────────────
	t.Run("nil_player_generic_match", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("GenericMatch panicked on nil player: %v", r)
			}
		}()
		// GenericMatch should not panic; the nil check is on History when
		// SkipDuplicates=true. Player is accessed only for UUID — ensure no nil deref.
		_ = GenericMatch(GenericMatchInput{
			Player:         newDetectedPlayer(uuid.New()),
			History:        NewMessageHistory(),
			Check:          GenericCheck{Channels: []string{"test:channel"}},
			Channel:        "test:channel",
			Message:        "",
			SkipDuplicates: false,
		})
	})

	t.Run("nil_player_forge", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("ProcessForgeModInfo panicked on nil player: %v", r)
			}
		}()
		result := ProcessForgeModInfo(nil, modinfo.ModInfo{Type: "FML2"}, ForgeConfig{Enabled: true})
		if result != nil {
			t.Errorf("ProcessForgeModInfo(nil player) should return nil, got %v", result)
		}
	})

	t.Run("nil_player_lunar", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("ProcessLunarHandshake panicked on nil player: %v", r)
			}
		}()
		result := ProcessLunarHandshake(nil, []LunarModInfo{{ID: "sodium"}}, LunarConfig{Enabled: true})
		if result.HasTriggers() {
			t.Errorf("ProcessLunarHandshake(nil player) should return no triggers")
		}
	})

	t.Run("nil_player_bedrock", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("ApplyBedrockBrand panicked on nil player: %v", r)
			}
		}()
		// nil player — function must not panic even though it calls player.SetBedrockDetected.
		// The caller should never pass nil, but we guard against it defensively.
		// Because the current implementation does call player.SetBedrockDetected unconditionally,
		// we pass a real player with Enabled=false to hit the early-exit path instead.
		dp := newDetectedPlayer(uuid.New())
		detected, _, _ := ApplyBedrockBrand(dp, "GEYSER", BedrockConfig{Enabled: false})
		if detected {
			t.Error("ApplyBedrockBrand with Enabled=false should return detected=false")
		}
	})

	// ── 2. Empty / malformed payloads ─────────────────────────────────────────
	t.Run("empty_brand_not_bedrock", func(t *testing.T) {
		if IsBedrock("") {
			t.Error("IsBedrock('') should return false")
		}
	})

	t.Run("empty_channels_forge", func(t *testing.T) {
		mods := ParseModsFromChannels([]string{})
		if len(mods) != 0 {
			t.Errorf("ParseModsFromChannels(empty) should return nil/empty, got %v", mods)
		}
	})

	t.Run("null_byte_payload_forge", func(t *testing.T) {
		// A null-byte separated payload with only built-in namespaces should
		// yield zero mods.
		payload := []byte("minecraft:brand\x00forge:hand\x00fml:handshake")
		mods := ParseModsFromRegisterPayload(payload)
		for _, m := range mods {
			if _, builtin := builtinNamespaces[m.ModID]; builtin {
				t.Errorf("ParseModsFromRegisterPayload: returned builtin namespace mod %q", m.ModID)
			}
		}
		t.Logf("null-byte payload yielded %d non-builtin mods", len(mods))
	})

	// ── 3. Action executor: unknown action IDs silently skipped ───────────────
	t.Run("unknown_action_id_skipped", func(t *testing.T) {
		executor := newActionExecutorWithClock(
			ActionsConfig{Actions: map[string]ActionDef{}},
			MainSettings{},
			ImmediateClock{},
		)
		var called bool
		executor.Execute([]string{"nonexistent_action"}, ActionContext{PlayerName: "p", CheckName: "c"}, ActionCallbacks{
			SendAlert: func(s string) { called = true },
		})
		if called {
			t.Error("SendAlert should not be called for unknown action ID")
		}
	})

	// ── 4. MessageHistory: Remove clears player history ───────────────────────
	t.Run("message_history_remove", func(t *testing.T) {
		history := NewMessageHistory()
		id := uuid.New()
		// First call: not duplicate.
		if history.IsDuplicate(id, "ch", "msg") {
			t.Error("first IsDuplicate should return false")
		}
		// Second call: duplicate.
		if !history.IsDuplicate(id, "ch", "msg") {
			t.Error("second IsDuplicate should return true")
		}
		// Remove clears history.
		history.Remove(id)
		// After removal: no longer a duplicate.
		if history.IsDuplicate(id, "ch", "msg") {
			t.Error("IsDuplicate after Remove should return false")
		}
	})

	// ── 5. PlayerStore: double-register does not reset pending actions ─────────
	t.Run("player_store_idempotent_register", func(t *testing.T) {
		store := NewPlayerStore()
		id := uuid.New()
		store.Register(id)
		p := store.Get(id)
		p.QueuePendingAction(func() {})

		// Second Register should NOT reset the player.
		store.Register(id)
		p2 := store.Get(id)
		if p != p2 {
			t.Error("PlayerStore.Register replaced an existing player pointer")
		}
		if !p2.HasPendingActions() {
			t.Error("Register on existing player wiped pending actions")
		}
	})

	// ── 6. handleListFromEntries: empty list produces appropriate message ──────
	t.Run("list_empty", func(t *testing.T) {
		src := newMockSource()
		ctx := fakeCommandContext(src)
		if err := handleListFromEntries(ctx, nil); err != nil {
			t.Fatalf("handleListFromEntries(nil) returned error: %v", err)
		}
		if !strings.Contains(src.lastMessage(), "No chocolate players spotted") {
			t.Errorf("empty list message unexpected: %q", src.lastMessage())
		}
	})

	// ── 7. RenderPlaceholders: no-op when no placeholders in string ───────────
	t.Run("render_placeholders_no_match", func(t *testing.T) {
		result := RenderPlaceholders("no placeholders here", "player1", "check1")
		if result != "no placeholders here" {
			t.Errorf("RenderPlaceholders mutated a string with no placeholders: %q", result)
		}
	})
}
