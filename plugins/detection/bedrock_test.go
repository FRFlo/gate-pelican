package detection

import (
	"testing"

	"github.com/google/uuid"
)

// TestBedrockBrandPositive verifies that IsBedrock and ApplyBedrockBrand
// correctly identify brands that indicate a Bedrock client.
func TestBedrockBrandPositive(t *testing.T) {
	cases := []struct {
		name  string
		brand string
	}{
		{name: "exact_lowercase", brand: "geyser"},
		{name: "exact_uppercase", brand: "GEYSER"},
		{name: "mixed_case", brand: "GEYSER"},
		{name: "prefix_version", brand: "GEYSER v2.2.0"},
		{name: "prefix_bedrock", brand: "GEYSER-Bedrock"},
		{name: "suffix_geyser", brand: "vanilla/geyser"},
		{name: "contains_geyser", brand: "my-geyser-client"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if !IsBedrock(tc.brand) {
				t.Errorf("IsBedrock(%q) = false, want true", tc.brand)
			}
		})
	}
}

// TestBedrockBrandNegative verifies that IsBedrock does NOT match brands that
// are not Bedrock clients.
func TestBedrockBrandNegative(t *testing.T) {
	cases := []struct {
		name  string
		brand string
	}{
		{name: "empty", brand: ""},
		{name: "vanilla", brand: "vanilla"},
		{name: "fabric", brand: "fabric"},
		{name: "forge", brand: "forge"},
		{name: "lunar_client", brand: "lunarclient/3.0.0-lunar"},
		{name: "labymod", brand: "LabyMod v3/1.20.4"},
		{name: "badlion", brand: "BLC/3.22.5"},
		{name: "optifine", brand: "OptiFine"},
		{name: "random_string", brand: "somerandombrand"},
		{name: "partial_mismatch", brand: "geys"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if IsBedrock(tc.brand) {
				t.Errorf("IsBedrock(%q) = true, want false", tc.brand)
			}
		})
	}
}

// TestApplyBedrockBrandPositive verifies that ApplyBedrockBrand sets the player
// state and returns the label/actions from config when detection fires.
func TestApplyBedrockBrandPositive(t *testing.T) {
	cfg := BedrockConfig{
		Enabled: true,
		Label:   "Bedrock",
		Actions: []string{"alert", "kick"},
	}

	cases := []struct {
		name  string
		brand string
	}{
		{name: "exact_geyser", brand: "geyser"},
		{name: "geyser_version", brand: "GEYSER v2.2.0"},
		{name: "upper_geyser", brand: "GEYSER"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			player := newDetectedPlayer(uuid.New())

			detected, label, actionIDs := ApplyBedrockBrand(player, tc.brand, cfg)

			if !detected {
				t.Fatalf("ApplyBedrockBrand(%q): detected=false, want true", tc.brand)
			}
			if label != cfg.Label {
				t.Errorf("label=%q, want %q", label, cfg.Label)
			}
			if len(actionIDs) != len(cfg.Actions) {
				t.Errorf("len(actionIDs)=%d, want %d", len(actionIDs), len(cfg.Actions))
			}
			for i, a := range actionIDs {
				if a != cfg.Actions[i] {
					t.Errorf("actionIDs[%d]=%q, want %q", i, a, cfg.Actions[i])
				}
			}
			if !player.IsBedrockDetected() {
				t.Error("player.IsBedrockDetected() = false, want true")
			}
		})
	}
}

// TestApplyBedrockBrandNegative verifies that ApplyBedrockBrand does NOT
// mutate player state when the brand does not match or detection is disabled.
func TestApplyBedrockBrandNegative(t *testing.T) {
	t.Run("non_bedrock_brand", func(t *testing.T) {
		cfg := BedrockConfig{
			Enabled: true,
			Label:   "Bedrock",
			Actions: []string{"alert"},
		}
		player := newDetectedPlayer(uuid.New())

		detected, label, actionIDs := ApplyBedrockBrand(player, "vanilla", cfg)

		if detected {
			t.Error("ApplyBedrockBrand(vanilla): detected=true, want false")
		}
		if label != "" {
			t.Errorf("label=%q, want empty", label)
		}
		if len(actionIDs) != 0 {
			t.Errorf("len(actionIDs)=%d, want 0", len(actionIDs))
		}
		if player.IsBedrockDetected() {
			t.Error("player must not be marked bedrock-detected on brand mismatch")
		}
	})

	t.Run("disabled_config", func(t *testing.T) {
		cfg := BedrockConfig{
			Enabled: false, // disabled
			Label:   "Bedrock",
			Actions: []string{"alert"},
		}
		player := newDetectedPlayer(uuid.New())

		detected, label, actionIDs := ApplyBedrockBrand(player, "geyser", cfg)

		if detected {
			t.Error("ApplyBedrockBrand with disabled cfg: detected=true, want false")
		}
		if label != "" || len(actionIDs) != 0 {
			t.Error("expected empty label and actions when cfg.Enabled=false")
		}
		if player.IsBedrockDetected() {
			t.Error("player must not be marked bedrock-detected when cfg.Enabled=false")
		}
	})

	t.Run("empty_brand", func(t *testing.T) {
		cfg := BedrockConfig{
			Enabled: true,
			Label:   "Bedrock",
			Actions: []string{"alert"},
		}
		player := newDetectedPlayer(uuid.New())

		detected, _, _ := ApplyBedrockBrand(player, "", cfg)

		if detected {
			t.Error("ApplyBedrockBrand with empty brand: detected=true, want false")
		}
		if player.IsBedrockDetected() {
			t.Error("player must not be marked bedrock-detected on empty brand")
		}
	})

	t.Run("empty_actions_config", func(t *testing.T) {
		cfg := BedrockConfig{
			Enabled: true,
			Label:   "Bedrock",
			Actions: []string{}, // no actions configured (default bedrock.toml)
		}
		player := newDetectedPlayer(uuid.New())

		detected, label, actionIDs := ApplyBedrockBrand(player, "GEYSER", cfg)

		if !detected {
			t.Error("detection must succeed even when no actions configured")
		}
		if label != "Bedrock" {
			t.Errorf("label=%q, want Bedrock", label)
		}
		if len(actionIDs) != 0 {
			t.Errorf("expected 0 action IDs, got %d", len(actionIDs))
		}
		if !player.IsBedrockDetected() {
			t.Error("player must be marked bedrock-detected")
		}
	})
}
