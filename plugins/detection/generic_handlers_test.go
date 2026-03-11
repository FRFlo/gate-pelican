package detection

import (
	"testing"

	"github.com/google/uuid"
)

// ─── TestBrandHandler ────────────────────────────────────────────────────────
//
// Verifies HandleBrandPayload routes brand payloads into GenericMatch,
// Bedrock detection, and Forge client-type detection correctly.

func TestBrandHandler(t *testing.T) {
	t.Run("generic_check_fires_on_matching_brand_channel", func(t *testing.T) {
		// Set up a generic check that listens on "minecraft:brand"
		cfg := DetectionConfig{
			Generic: GenericConfig{
				Enabled: true,
				Checks: map[string]GenericCheck{
					"forge_brand": {
						Name:     "Forge Brand",
						Channels: []string{brandChannel},
						Actions:  []string{"alert"},
					},
				},
			},
		}
		player := newDetectedPlayer(uuid.New())
		history := NewMessageHistory()

		result := HandleBrandPayload(player, history, "forge/1.20.4", cfg, false)

		if len(result.GenericTriggers) != 1 {
			t.Fatalf("expected 1 generic trigger, got %d", len(result.GenericTriggers))
		}
		trigger := result.GenericTriggers[0]
		if trigger.CheckID != "forge_brand" {
			t.Errorf("CheckID=%q, want %q", trigger.CheckID, "forge_brand")
		}
		if trigger.Name != "Forge Brand" {
			t.Errorf("Name=%q, want %q", trigger.Name, "Forge Brand")
		}
		if len(trigger.ActionIDs) != 1 || trigger.ActionIDs[0] != "alert" {
			t.Errorf("ActionIDs=%v, want [alert]", trigger.ActionIDs)
		}
		if !player.HasGenericCheck("forge_brand") {
			t.Error("player should have forge_brand generic check recorded")
		}
	})

	t.Run("generic_check_does_not_fire_for_wrong_channel", func(t *testing.T) {
		cfg := DetectionConfig{
			Generic: GenericConfig{
				Enabled: true,
				Checks: map[string]GenericCheck{
					"some_other": {
						Name:     "Other",
						Channels: []string{"other:channel"},
						Actions:  []string{"alert"},
					},
				},
			},
		}
		player := newDetectedPlayer(uuid.New())
		history := NewMessageHistory()

		result := HandleBrandPayload(player, history, "forge/1.20.4", cfg, false)

		if len(result.GenericTriggers) != 0 {
			t.Errorf("expected 0 generic triggers, got %d", len(result.GenericTriggers))
		}
	})

	t.Run("generic_disabled_means_no_triggers", func(t *testing.T) {
		cfg := DetectionConfig{
			Generic: GenericConfig{
				Enabled: false, // disabled
				Checks: map[string]GenericCheck{
					"forge_brand": {
						Channels: []string{brandChannel},
						Actions:  []string{"alert"},
					},
				},
			},
		}
		player := newDetectedPlayer(uuid.New())
		history := NewMessageHistory()

		result := HandleBrandPayload(player, history, "forge/1.20.4", cfg, false)

		if len(result.GenericTriggers) != 0 {
			t.Errorf("expected 0 generic triggers when disabled, got %d", len(result.GenericTriggers))
		}
	})

	t.Run("bedrock_brand_triggers_detection", func(t *testing.T) {
		cfg := DetectionConfig{
			Bedrock: BedrockConfig{
				Enabled: true,
				Label:   "Bedrock",
				Actions: []string{"alert"},
			},
		}
		player := newDetectedPlayer(uuid.New())
		history := NewMessageHistory()

		result := HandleBrandPayload(player, history, "GEYSER v2.2.0", cfg, false)

		if !result.BedrockDetected {
			t.Error("expected BedrockDetected=true for geyser brand")
		}
		if result.BedrockLabel != "Bedrock" {
			t.Errorf("BedrockLabel=%q, want Bedrock", result.BedrockLabel)
		}
		if len(result.BedrockActionIDs) != 1 {
			t.Errorf("expected 1 bedrock action ID, got %d", len(result.BedrockActionIDs))
		}
		if !player.IsBedrockDetected() {
			t.Error("player.IsBedrockDetected() should be true after geyser brand")
		}
	})

	t.Run("bedrock_brand_not_triggered_for_vanilla", func(t *testing.T) {
		cfg := DetectionConfig{
			Bedrock: BedrockConfig{
				Enabled: true,
				Label:   "Bedrock",
				Actions: []string{"alert"},
			},
		}
		player := newDetectedPlayer(uuid.New())
		history := NewMessageHistory()

		result := HandleBrandPayload(player, history, "vanilla", cfg, false)

		if result.BedrockDetected {
			t.Error("expected BedrockDetected=false for vanilla brand")
		}
		if player.IsBedrockDetected() {
			t.Error("player should not be marked bedrock-detected for vanilla brand")
		}
	})

	t.Run("forge_brand_detected_and_forge_type_set", func(t *testing.T) {
		cfg := DetectionConfig{
			Forge: ForgeConfig{
				Enabled: true,
				Settings: ForgeSettings{
					MarkForge: true,
				},
				Actions: ForgeActions{
					Forge: []string{"forge_alert"},
				},
			},
		}
		player := newDetectedPlayer(uuid.New())
		history := NewMessageHistory()

		result := HandleBrandPayload(player, history, "forge/1.20.4", cfg, false)

		if len(result.ForgeTriggers) != 1 {
			t.Fatalf("expected 1 forge trigger for forge brand, got %d", len(result.ForgeTriggers))
		}
		ft := result.ForgeTriggers[0]
		if ft.Name != "Forge" {
			t.Errorf("ForgeTrigger.Name=%q, want Forge", ft.Name)
		}
		if len(ft.ActionIDs) != 1 || ft.ActionIDs[0] != "forge_alert" {
			t.Errorf("ForgeTrigger.ActionIDs=%v, want [forge_alert]", ft.ActionIDs)
		}
		fct, ok := player.ForgeClientType()
		if !ok {
			t.Error("player.ForgeClientType() should be set after forge brand")
		}
		if fct != ForgeClientForge {
			t.Errorf("player.ForgeClientType()=%v, want ForgeClientForge", fct)
		}
	})

	t.Run("neoforge_brand_detected_correctly", func(t *testing.T) {
		cfg := DetectionConfig{
			Forge: ForgeConfig{
				Enabled: true,
				Settings: ForgeSettings{
					MarkNeoForge: true,
				},
				Actions: ForgeActions{
					NeoForge: []string{"neoforge_alert"},
				},
			},
		}
		player := newDetectedPlayer(uuid.New())
		history := NewMessageHistory()

		result := HandleBrandPayload(player, history, "neoforge/1.20.6", cfg, false)

		if len(result.ForgeTriggers) != 1 {
			t.Fatalf("expected 1 forge trigger for neoforge brand, got %d", len(result.ForgeTriggers))
		}
		if result.ForgeTriggers[0].Name != "NeoForge" {
			t.Errorf("ForgeTrigger.Name=%q, want NeoForge", result.ForgeTriggers[0].Name)
		}
		fct, ok := player.ForgeClientType()
		if !ok {
			t.Error("player.ForgeClientType() should be set after neoforge brand")
		}
		if fct != ForgeClientNeoForge {
			t.Errorf("player.ForgeClientType()=%v, want ForgeClientNeoForge", fct)
		}
	})

	t.Run("forge_brand_not_retriggered_when_type_already_set", func(t *testing.T) {
		cfg := DetectionConfig{
			Forge: ForgeConfig{
				Enabled: true,
				Settings: ForgeSettings{
					MarkForge: true,
				},
				Actions: ForgeActions{
					Forge: []string{"forge_alert"},
				},
			},
		}
		player := newDetectedPlayer(uuid.New())
		player.SetForgeClientType(ForgeClientForge) // pre-set: type already known
		history := NewMessageHistory()

		result := HandleBrandPayload(player, history, "forge/1.20.4", cfg, false)

		// Should produce no triggers because type was already set.
		if len(result.ForgeTriggers) != 0 {
			t.Errorf("expected 0 forge triggers (type already set), got %d", len(result.ForgeTriggers))
		}
	})

	t.Run("vanilla_brand_no_detections", func(t *testing.T) {
		cfg := DetectionConfig{
			Generic: GenericConfig{
				Enabled: true,
				Checks: map[string]GenericCheck{
					"labymod": {
						Channels:   []string{brandChannel},
						MessageHas: "labymod",
						Name:       "LabyMod",
						Actions:    []string{"alert"},
					},
				},
			},
			Bedrock: BedrockConfig{Enabled: true, Label: "Bedrock", Actions: []string{"alert"}},
			Forge:   ForgeConfig{Enabled: true, Settings: ForgeSettings{MarkForge: true}},
		}
		player := newDetectedPlayer(uuid.New())
		history := NewMessageHistory()

		result := HandleBrandPayload(player, history, "vanilla", cfg, false)

		if len(result.GenericTriggers) != 0 {
			t.Errorf("expected 0 generic triggers for vanilla, got %d", len(result.GenericTriggers))
		}
		if result.BedrockDetected {
			t.Error("expected BedrockDetected=false for vanilla")
		}
		if len(result.ForgeTriggers) != 0 {
			t.Errorf("expected 0 forge triggers for vanilla, got %d", len(result.ForgeTriggers))
		}
	})
}

// ─── TestChannelHandler ──────────────────────────────────────────────────────
//
// Verifies HandleChannelRegister routes channel register payloads into
// GenericMatch and Forge mod detection.

func TestChannelHandler(t *testing.T) {
	t.Run("generic_check_fires_on_register_channel", func(t *testing.T) {
		cfg := DetectionConfig{
			Generic: GenericConfig{
				Enabled: true,
				Checks: map[string]GenericCheck{
					"some_mod_register": {
						Name:     "SomeMod Register",
						Channels: []string{registerChannel},
						Actions:  []string{"alert"},
					},
				},
			},
		}
		player := newDetectedPlayer(uuid.New())
		history := NewMessageHistory()

		result := HandleChannelRegister(player, history, "somemod:channel", nil, cfg, false)

		if len(result.GenericTriggers) != 1 {
			t.Fatalf("expected 1 generic trigger, got %d", len(result.GenericTriggers))
		}
		if result.GenericTriggers[0].CheckID != "some_mod_register" {
			t.Errorf("CheckID=%q, want some_mod_register", result.GenericTriggers[0].CheckID)
		}
	})

	t.Run("forge_mods_from_channels_list", func(t *testing.T) {
		cfg := DetectionConfig{
			Forge: ForgeConfig{
				Enabled:  true,
				Settings: ForgeSettings{MarkForge: false},
				ModActions: map[string][]string{
					"forgewurst": {"kick"},
				},
			},
		}
		player := newDetectedPlayer(uuid.New())
		history := NewMessageHistory()

		channels := []string{
			"forgewurst:channel",
			"minecraft:chat_type",
			"neoforge:registry_sync",
		}

		result := HandleChannelRegister(player, history, "", channels, cfg, false)

		if len(result.ForgeTriggers) != 1 {
			t.Fatalf("expected 1 forge trigger (forgewurst), got %d", len(result.ForgeTriggers))
		}
		if len(result.ForgeTriggers[0].ActionIDs) == 0 {
			t.Error("expected non-empty action IDs for forgewurst mod")
		}
		if !player.HasForgeMod("forgewurst") {
			t.Error("player should have forgewurst forge mod recorded")
		}
	})

	t.Run("forge_mods_from_raw_payload", func(t *testing.T) {
		cfg := DetectionConfig{
			Forge: ForgeConfig{
				Enabled:  true,
				Settings: ForgeSettings{MarkForge: false},
				ModActions: map[string][]string{
					"wurst": {"alert"},
				},
			},
		}
		player := newDetectedPlayer(uuid.New())
		history := NewMessageHistory()

		// Raw null-separated payload (as in a real minecraft:register packet)
		rawPayload := "wurst:channel\x00minecraft:chat_type"

		result := HandleChannelRegister(player, history, rawPayload, nil, cfg, false)

		if len(result.ForgeTriggers) != 1 {
			t.Fatalf("expected 1 forge trigger (wurst from raw payload), got %d", len(result.ForgeTriggers))
		}
		if !player.HasForgeMod("wurst") {
			t.Error("player should have wurst forge mod recorded from raw payload")
		}
	})

	t.Run("no_triggers_when_forge_disabled", func(t *testing.T) {
		cfg := DetectionConfig{
			Forge: ForgeConfig{
				Enabled: false, // disabled
				ModActions: map[string][]string{
					"forgewurst": {"kick"},
				},
			},
		}
		player := newDetectedPlayer(uuid.New())
		history := NewMessageHistory()

		result := HandleChannelRegister(player, history, "", []string{"forgewurst:channel"}, cfg, false)

		if len(result.ForgeTriggers) != 0 {
			t.Errorf("expected 0 forge triggers when forge disabled, got %d", len(result.ForgeTriggers))
		}
	})
}

// ─── TestBypassSkip ──────────────────────────────────────────────────────────
//
// Verifies that when bypass=true:
// - Player state IS mutated (detection still records checks/mods).
// - All returned ActionIDs are nil/empty (actions are not enqueued).
//
// Java mapping: hackedserver.bypass permission check in performActions().

func TestBypassSkip(t *testing.T) {
	t.Run("bypass_clears_generic_action_ids_but_records_check", func(t *testing.T) {
		cfg := DetectionConfig{
			Generic: GenericConfig{
				Enabled: true,
				Checks: map[string]GenericCheck{
					"labymod_brand": {
						Name:     "LabyMod",
						Channels: []string{brandChannel},
						Actions:  []string{"alert", "kick"},
					},
				},
			},
		}
		player := newDetectedPlayer(uuid.New())
		history := NewMessageHistory()

		result := HandleBrandPayload(player, history, "LabyMod v3/1.20.4", cfg, true /* bypass */)

		// Detection should fire (channel matches), but ActionIDs must be empty.
		if len(result.GenericTriggers) != 1 {
			t.Fatalf("expected 1 generic trigger even with bypass, got %d", len(result.GenericTriggers))
		}
		if len(result.GenericTriggers[0].ActionIDs) != 0 {
			t.Errorf("bypass: expected empty ActionIDs, got %v", result.GenericTriggers[0].ActionIDs)
		}
		// Player state MUST still be recorded.
		if !player.HasGenericCheck("labymod_brand") {
			t.Error("bypass: player state should still record the check even when bypassed")
		}
	})

	t.Run("bypass_clears_bedrock_action_ids_but_mutates_state", func(t *testing.T) {
		cfg := DetectionConfig{
			Bedrock: BedrockConfig{
				Enabled: true,
				Label:   "Bedrock",
				Actions: []string{"alert", "kick"},
			},
		}
		player := newDetectedPlayer(uuid.New())
		history := NewMessageHistory()

		result := HandleBrandPayload(player, history, "GEYSER v2.2.0", cfg, true /* bypass */)

		if !result.BedrockDetected {
			t.Error("bypass: BedrockDetected should still be true")
		}
		if len(result.BedrockActionIDs) != 0 {
			t.Errorf("bypass: expected empty BedrockActionIDs, got %v", result.BedrockActionIDs)
		}
		// Player state MUST still reflect the bedrock detection.
		if !player.IsBedrockDetected() {
			t.Error("bypass: player.IsBedrockDetected() should be true even when bypassed")
		}
	})

	t.Run("bypass_clears_forge_brand_action_ids_but_sets_type", func(t *testing.T) {
		cfg := DetectionConfig{
			Forge: ForgeConfig{
				Enabled:  true,
				Settings: ForgeSettings{MarkForge: true},
				Actions:  ForgeActions{Forge: []string{"forge_alert"}},
			},
		}
		player := newDetectedPlayer(uuid.New())
		history := NewMessageHistory()

		result := HandleBrandPayload(player, history, "forge/1.20.4", cfg, true /* bypass */)

		if len(result.ForgeTriggers) != 1 {
			t.Fatalf("bypass: expected 1 forge trigger (name still returned), got %d", len(result.ForgeTriggers))
		}
		if len(result.ForgeTriggers[0].ActionIDs) != 0 {
			t.Errorf("bypass: ForgeTrigger ActionIDs should be empty, got %v", result.ForgeTriggers[0].ActionIDs)
		}
		// Player forge type still recorded.
		_, ok := player.ForgeClientType()
		if !ok {
			t.Error("bypass: player.ForgeClientType() should be set even when bypassed")
		}
	})

	t.Run("bypass_clears_channel_register_forge_action_ids_but_records_mods", func(t *testing.T) {
		cfg := DetectionConfig{
			Forge: ForgeConfig{
				Enabled: true,
				ModActions: map[string][]string{
					"forgewurst": {"kick"},
				},
			},
		}
		player := newDetectedPlayer(uuid.New())
		history := NewMessageHistory()

		result := HandleChannelRegister(
			player, history, "", []string{"forgewurst:channel"}, cfg, true, /* bypass */
		)

		if len(result.ForgeTriggers) != 1 {
			t.Fatalf("bypass: expected 1 forge trigger even with bypass, got %d", len(result.ForgeTriggers))
		}
		if len(result.ForgeTriggers[0].ActionIDs) != 0 {
			t.Errorf("bypass: ForgeTrigger ActionIDs should be empty, got %v", result.ForgeTriggers[0].ActionIDs)
		}
		// Player mod state still recorded.
		if !player.HasForgeMod("forgewurst") {
			t.Error("bypass: player should still have forgewurst recorded")
		}
	})

	t.Run("bypass_false_preserves_action_ids", func(t *testing.T) {
		// Sanity check: with bypass=false, action IDs come through normally.
		cfg := DetectionConfig{
			Forge: ForgeConfig{
				Enabled: true,
				ModActions: map[string][]string{
					"badmod": {"kick", "alert"},
				},
			},
		}
		player := newDetectedPlayer(uuid.New())
		history := NewMessageHistory()

		result := HandleChannelRegister(
			player, history, "", []string{"badmod:channel"}, cfg, false, /* bypass=false */
		)

		if len(result.ForgeTriggers) != 1 {
			t.Fatalf("expected 1 forge trigger, got %d", len(result.ForgeTriggers))
		}
		if len(result.ForgeTriggers[0].ActionIDs) != 2 {
			t.Errorf("expected 2 action IDs, got %v", result.ForgeTriggers[0].ActionIDs)
		}
	})

	t.Run("bypass_channel_generic_clears_action_ids_but_records_check", func(t *testing.T) {
		cfg := DetectionConfig{
			Generic: GenericConfig{
				Enabled: true,
				Checks: map[string]GenericCheck{
					"hacked_register": {
						Name:     "Hacked Register",
						Channels: []string{registerChannel},
						Actions:  []string{"kick"},
					},
				},
			},
		}
		player := newDetectedPlayer(uuid.New())
		history := NewMessageHistory()

		result := HandleChannelRegister(player, history, "hacked:channel", nil, cfg, true /* bypass */)

		if len(result.GenericTriggers) != 1 {
			t.Fatalf("bypass: expected 1 generic trigger even with bypass, got %d", len(result.GenericTriggers))
		}
		if len(result.GenericTriggers[0].ActionIDs) != 0 {
			t.Errorf("bypass: ActionIDs should be empty, got %v", result.GenericTriggers[0].ActionIDs)
		}
		if !player.HasGenericCheck("hacked_register") {
			t.Error("bypass: player should still record the generic check")
		}
	})
}
