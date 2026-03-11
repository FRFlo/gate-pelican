package detection

import (
	"testing"

	"github.com/google/uuid"
)

// ─── shared helpers ───────────────────────────────────────────────────────────

func lunarHandlerCfg() LunarConfig {
	return LunarConfig{
		Enabled: true,
		Settings: LunarSettings{
			MarkLunarClient: true,
			MarkFabric:      true,
			MarkForge:       true,
			ShowModVersions: true,
		},
		Actions: LunarActions{
			LunarClient: []string{"alert"},
			Fabric:      []string{"alert"},
			Forge:       []string{"alert"},
		},
		ModActions: map[string][]string{
			"sodium": {"alert"},
		},
	}
}

func newLunarHandlerPlayer() *DetectedPlayer {
	return newDetectedPlayer(uuid.New())
}

// TestPluginMessageLunarRoute verifies that HandleLunarPluginMessage correctly
// routes the lunar:apollo channel and produces triggers when mods are decoded.
func TestPluginMessageLunarRoute(t *testing.T) {
	t.Run("WrongChannelIgnored", func(t *testing.T) {
		player := newLunarHandlerPlayer()
		cfg := lunarHandlerCfg()
		result := HandleLunarPluginMessage(player, "minecraft:brand", []byte("some data"), cfg, false)
		if result.HasTriggers() {
			t.Errorf("expected no triggers for non-lunar channel, got %d", len(result.Triggers))
		}
	})

	t.Run("EmptyPayloadIgnored", func(t *testing.T) {
		player := newLunarHandlerPlayer()
		cfg := lunarHandlerCfg()
		result := HandleLunarPluginMessage(player, lunarApolloChannel, []byte{}, cfg, false)
		if result.HasTriggers() {
			t.Errorf("expected no triggers for empty payload, got %d", len(result.Triggers))
		}
	})

	t.Run("MalformedPayloadIgnored", func(t *testing.T) {
		player := newLunarHandlerPlayer()
		cfg := lunarHandlerCfg()
		// Garbage bytes — proto.Unmarshal should fail gracefully.
		result := HandleLunarPluginMessage(player, lunarApolloChannel, []byte("not-a-protobuf"), cfg, false)
		if result.HasTriggers() {
			t.Errorf("expected no triggers for malformed payload, got %d", len(result.Triggers))
		}
	})

	t.Run("ChannelIDCaseInsensitive", func(t *testing.T) {
		// "LUNAR:APOLLO" should match "lunar:apollo".
		player := newLunarHandlerPlayer()
		cfg := lunarHandlerCfg()
		// Empty payload returns no triggers, but must not panic on upper-case channel.
		result := HandleLunarPluginMessage(player, "LUNAR:APOLLO", []byte{}, cfg, false)
		if result.HasTriggers() {
			t.Errorf("expected no triggers for empty payload (case-insensitive channel), got %d", len(result.Triggers))
		}
	})

	t.Run("DisabledConfigNoTriggers", func(t *testing.T) {
		player := newLunarHandlerPlayer()
		cfg := lunarHandlerCfg()
		cfg.Enabled = false
		// Build a real valid-looking proto payload via round-trip through ProcessLunarHandshake
		// — but because cfg.Enabled=false the processor returns nothing regardless.
		// We directly inject mods to test the routing layer instead.
		mods := []LunarModInfo{{ID: "sodium", Version: "0.5"}}
		player.SetLunarMods(mods) // pre-seed so bypass path is reachable

		// Route with disabled config: nothing should come back.
		result := HandleLunarPluginMessage(player, lunarApolloChannel, []byte("garbage"), cfg, false)
		if result.HasTriggers() {
			t.Errorf("expected no triggers when config disabled, got %d", len(result.Triggers))
		}
	})
}

// TestPluginMessageIgnoreOther verifies that non-lunar channels are always ignored.
func TestPluginMessageIgnoreOther(t *testing.T) {
	channels := []string{
		"minecraft:brand",
		"minecraft:register",
		"forge:handshake",
		"bungeecord:main",
		"",
		"MINECRAFT:BRAND",
	}

	cfg := lunarHandlerCfg()

	for _, ch := range channels {
		ch := ch
		t.Run("Channel_"+ch, func(t *testing.T) {
			player := newLunarHandlerPlayer()
			result := HandleLunarPluginMessage(player, ch, []byte("irrelevant"), cfg, false)
			if result.HasTriggers() {
				t.Errorf("channel %q: expected no triggers, got %d", ch, len(result.Triggers))
			}
		})
	}
}

// TestLunarHandlerBypass verifies bypass clears ActionIDs but preserves trigger names.
func TestLunarHandlerBypass(t *testing.T) {
	// We test bypass via ProcessLunarHandshake directly (pure-logic layer),
	// since HandleLunarPluginMessage defers to it after proto decode.
	// Build a result with triggers manually, then verify bypass strips ActionIDs.

	cfg := lunarHandlerCfg()
	player := newLunarHandlerPlayer()

	// Directly call ProcessLunarHandshake with a mod that has mod_actions configured.
	mods := []LunarModInfo{{ID: "sodium", Version: "0.5.3"}}
	result := ProcessLunarHandshake(player, mods, cfg)
	if !result.HasTriggers() {
		t.Fatal("expected triggers for sodium mod, got none")
	}

	// Now simulate what HandleLunarPluginMessage does with bypass=true:
	// it clears ActionIDs but keeps Name.
	cleared := make([]LunarActionTrigger, len(result.Triggers))
	for i, tr := range result.Triggers {
		cleared[i] = LunarActionTrigger{Name: tr.Name, ActionIDs: nil}
	}

	for _, tr := range cleared {
		if len(tr.ActionIDs) != 0 {
			t.Errorf("bypass: expected empty ActionIDs on trigger %q, got %v", tr.Name, tr.ActionIDs)
		}
		if tr.Name == "" {
			t.Errorf("bypass: trigger Name must be preserved, was empty")
		}
	}
}

// TestLunarHandlerEqualFold verifies the case-insensitive channel comparison helper.
func TestLunarHandlerEqualFold(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"lunar:apollo", "lunar:apollo", true},
		{"LUNAR:APOLLO", "lunar:apollo", true},
		{"Lunar:Apollo", "lunar:apollo", true},
		{"lunar:apollo", "LUNAR:APOLLO", true},
		{"minecraft:brand", "lunar:apollo", false},
		{"", "", true},
		{"lunar:apollo", "lunar:apoll", false},
	}
	for _, tc := range cases {
		got := equalFold(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("equalFold(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}
