package detection_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/minekube/gate-plugin-template/plugins/detection"
)

// ─── helpers ────────────────────────────────────────────────────────────────

func newPlayer() *detection.DetectedPlayer {
	store := detection.NewPlayerStore()
	id := uuid.New()
	store.Register(id)
	return store.Get(id)
}

func check(channels []string, messageHas, messageNotHas string) detection.GenericCheck {
	return detection.GenericCheck{
		Channels:      channels,
		MessageHas:    messageHas,
		MessageNotHas: messageNotHas,
		Name:          "test",
		Category:      "test",
	}
}

// ─── TestGenericMatcherKnown ─────────────────────────────────────────────────
//
// Verifies that known signatures from generic.toml fire correctly.
// Tests cover the channel-only, message_has, message_not_has, and combined cases
// drawn directly from the submodule config.

func TestGenericMatcherKnown(t *testing.T) {
	t.Parallel()
	history := detection.NewMessageHistory()

	cases := []struct {
		name    string
		check   detection.GenericCheck
		channel string
		message string
		want    bool
	}{
		// ── channel-only checks (no message_has / message_not_has) ──────────
		{
			name:    "labymod_v1 matches LABYMOD channel",
			check:   check([]string{"LABYMOD"}, "", ""),
			channel: "LABYMOD",
			message: "anything",
			want:    true,
		},
		{
			name:    "labymod_v1 rejects wrong channel",
			check:   check([]string{"LABYMOD"}, "", ""),
			channel: "LMC",
			message: "anything",
			want:    false,
		},
		{
			name:    "labymod_v2 matches LMC channel",
			check:   check([]string{"LMC"}, "", ""),
			channel: "LMC",
			message: "data",
			want:    true,
		},
		{
			name:    "labymod_v3 matches labymod3:main channel",
			check:   check([]string{"labymod3:main"}, "", ""),
			channel: "labymod3:main",
			message: "",
			want:    true,
		},
		{
			name:    "cracked_vape matches LOLIMAHCKER",
			check:   check([]string{"LOLIMAHCKER"}, "", ""),
			channel: "LOLIMAHCKER",
			message: "payload",
			want:    true,
		},
		{
			name:    "world_downloader_v3 matches WDL|INIT",
			check:   check([]string{"WDL|INIT"}, "", ""),
			channel: "WDL|INIT",
			message: "",
			want:    true,
		},
		// ── message_has checks (case-insensitive) ───────────────────────────
		{
			name:    "fabric brand (lowercase payload)",
			check:   check([]string{"MC|Brand", "minecraft:brand"}, "fabric", ""),
			channel: "minecraft:brand",
			message: "vanilla/fabric",
			want:    true,
		},
		{
			name:    "fabric brand (uppercase payload) — case-insensitive",
			check:   check([]string{"MC|Brand", "minecraft:brand"}, "fabric", ""),
			channel: "MC|Brand",
			message: "FABRIC",
			want:    true,
		},
		{
			name:    "fabric brand wrong channel",
			check:   check([]string{"MC|Brand", "minecraft:brand"}, "fabric", ""),
			channel: "REGISTER",
			message: "fabric",
			want:    false,
		},
		{
			name:    "fabric brand missing substring",
			check:   check([]string{"MC|Brand", "minecraft:brand"}, "fabric", ""),
			channel: "minecraft:brand",
			message: "vanilla",
			want:    false,
		},
		{
			name:    "liteloader brand",
			check:   check([]string{"MC|Brand", "minecraft:brand"}, "LiteLoader", ""),
			channel: "minecraft:brand",
			message: "LiteLoader 1.12",
			want:    true,
		},
		{
			name:    "liteloader brand case-insensitive (lower payload)",
			check:   check([]string{"MC|Brand", "minecraft:brand"}, "LiteLoader", ""),
			channel: "minecraft:brand",
			message: "liteloader 1.12",
			want:    true,
		},
		{
			name:    "optifine brand",
			check:   check([]string{"MC|Brand", "minecraft:brand"}, "optifine", ""),
			channel: "MC|Brand",
			message: "vanilla/OptiFine",
			want:    true,
		},
		{
			name:    "world_downloader_v2 brand",
			check:   check([]string{"MC|Brand", "minecraft:brand"}, "WorldDownloader", ""),
			channel: "minecraft:brand",
			message: "vanilla/WorldDownloader",
			want:    true,
		},
		{
			name:    "world_downloader_v4 WDL|CONTROL with WDL",
			check:   check([]string{"WDL|CONTROL"}, "WDL", ""),
			channel: "WDL|CONTROL",
			message: "WDL control data",
			want:    true,
		},
		{
			name:    "forge_mod_loader_v1 REGISTER with legacy:fml",
			check:   check([]string{"REGISTER", "minecraft:register"}, "legacy:fml", ""),
			channel: "REGISTER",
			message: "legacy:fml\x00minecraft:unregister",
			want:    true,
		},
		{
			name:    "forge_mod_loader_v1 minecraft:register channel",
			check:   check([]string{"REGISTER", "minecraft:register"}, "legacy:fml", ""),
			channel: "minecraft:register",
			message: "legacy:fml payload",
			want:    true,
		},
		{
			name:    "world_downloader_v1 REGISTER with wdl",
			check:   check([]string{"REGISTER", "minecraft:register"}, "wdl", ""),
			channel: "REGISTER",
			message: "wdl:control",
			want:    true,
		},
		{
			name:    "badlion brand",
			check:   check([]string{"MC|Brand", "minecraft:brand"}, "badlion", ""),
			channel: "minecraft:brand",
			message: "Badlion Client 3.0",
			want:    true,
		},
		{
			name:    "lunar_client brand (lowercase lunarclient)",
			check:   check([]string{"MC|Brand", "minecraft:brand"}, "lunarclient", ""),
			channel: "MC|Brand",
			message: "lunarclient/1.0",
			want:    true,
		},
		{
			name:    "pvp_lounge brand PLC18",
			check:   check([]string{"MC|Brand", "minecraft:brand"}, "PLC18", ""),
			channel: "minecraft:brand",
			message: "PLC18/2.0",
			want:    true,
		},
		{
			name:    "emc brand Subsystem",
			check:   check([]string{"MC|Brand", "minecraft:brand"}, "Subsystem", ""),
			channel: "minecraft:brand",
			message: "Subsystem 1.0",
			want:    true,
		},
		{
			name:    "feather brand",
			check:   check([]string{"MC|Brand", "minecraft:brand"}, "feather", ""),
			channel: "minecraft:brand",
			message: "Feather/1.0",
			want:    true,
		},
		// ── message_not_has checks ──────────────────────────────────────────
		{
			name:    "forge brand with neoforge payload — rejected by message_not_has",
			check:   check([]string{"MC|Brand", "minecraft:brand"}, "forge", "neoforge"),
			channel: "minecraft:brand",
			message: "neoforge/20.4",
			want:    false,
		},
		{
			name:    "forge brand without neoforge payload — passes",
			check:   check([]string{"MC|Brand", "minecraft:brand"}, "forge", "neoforge"),
			channel: "minecraft:brand",
			message: "Forge/43.2",
			want:    true,
		},
		{
			name:    "neoforge brand passes for neoforge payload",
			check:   check([]string{"MC|Brand", "minecraft:brand"}, "neoforge", ""),
			channel: "minecraft:brand",
			message: "NeoForge/20.4",
			want:    true,
		},
		{
			name:    "neoforge brand rejects non-neoforge payload",
			check:   check([]string{"MC|Brand", "minecraft:brand"}, "neoforge", ""),
			channel: "minecraft:brand",
			message: "Forge/43.2",
			want:    false,
		},
		// ── multi-channel checks ────────────────────────────────────────────
		{
			name:    "forge_mod_loader_v2 FML|HS channel",
			check:   check([]string{"FML|HS", "l:fmlhs"}, "", ""),
			channel: "FML|HS",
			message: "",
			want:    true,
		},
		{
			name:    "forge_mod_loader_v2 l:fmlhs channel",
			check:   check([]string{"FML|HS", "l:fmlhs"}, "", ""),
			channel: "l:fmlhs",
			message: "",
			want:    true,
		},
		{
			name:    "forge_mod_loader_v2 wrong channel",
			check:   check([]string{"FML|HS", "l:fmlhs"}, "", ""),
			channel: "minecraft:brand",
			message: "",
			want:    false,
		},
		// ── empty channels list ─────────────────────────────────────────────
		{
			name:    "empty channels list always fails",
			check:   check([]string{}, "", ""),
			channel: "anything",
			message: "anything",
			want:    false,
		},
		// ── channel is case-sensitive (mirrors Java List.contains) ──────────
		{
			name:    "channel match is case-sensitive (wrong case fails)",
			check:   check([]string{"LABYMOD"}, "", ""),
			channel: "labymod",
			message: "anything",
			want:    false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			player := newPlayer()
			got := detection.GenericMatch(detection.GenericMatchInput{
				Player:         player,
				History:        history,
				Check:          tc.check,
				Channel:        tc.channel,
				Message:        tc.message,
				SkipDuplicates: false, // dedupe off for all known-signature tests
			})
			if got != tc.want {
				t.Errorf("GenericMatch() = %v, want %v (channel=%q, msg=%q, has=%q, notHas=%q)",
					got, tc.want, tc.channel, tc.message, tc.check.MessageHas, tc.check.MessageNotHas)
			}
		})
	}
}

// ─── TestGenericMatcherDedupe ─────────────────────────────────────────────────
//
// Verifies duplicate-suppression semantics when SkipDuplicates is true.
// Mirrors Java: first call returns true, second identical call returns false.

func TestGenericMatcherDedupe(t *testing.T) {
	t.Parallel()

	t.Run("first_trigger_true_repeated_false", func(t *testing.T) {
		t.Parallel()

		player := newPlayer()
		history := detection.NewMessageHistory()
		c := check([]string{"LABYMOD"}, "", "")

		in := detection.GenericMatchInput{
			Player:         player,
			History:        history,
			Check:          c,
			Channel:        "LABYMOD",
			Message:        "labymod data",
			SkipDuplicates: true,
		}

		// First match: should pass.
		if got := detection.GenericMatch(in); !got {
			t.Fatal("first call: expected true, got false")
		}
		// Second identical call: should be suppressed.
		if got := detection.GenericMatch(in); got {
			t.Fatal("second call (duplicate): expected false, got true")
		}
		// Third identical call: still suppressed.
		if got := detection.GenericMatch(in); got {
			t.Fatal("third call (duplicate): expected false, got true")
		}
	})

	t.Run("different_message_same_channel_not_suppressed", func(t *testing.T) {
		t.Parallel()

		player := newPlayer()
		history := detection.NewMessageHistory()
		c := check([]string{"LABYMOD"}, "", "")

		first := detection.GenericMatchInput{
			Player: player, History: history, Check: c,
			Channel: "LABYMOD", Message: "msg-one", SkipDuplicates: true,
		}
		second := detection.GenericMatchInput{
			Player: player, History: history, Check: c,
			Channel: "LABYMOD", Message: "msg-two", SkipDuplicates: true,
		}

		if got := detection.GenericMatch(first); !got {
			t.Fatal("first message: expected true")
		}
		// Different message on same channel → distinct payload → not suppressed.
		if got := detection.GenericMatch(second); !got {
			t.Fatal("different message: expected true")
		}
		// Repeat first again → suppressed.
		if got := detection.GenericMatch(first); got {
			t.Fatal("repeat first message: expected false")
		}
	})

	t.Run("different_channel_same_message_not_suppressed", func(t *testing.T) {
		t.Parallel()

		player := newPlayer()
		history := detection.NewMessageHistory()
		c := check([]string{"MC|Brand", "minecraft:brand"}, "", "")

		legacy := detection.GenericMatchInput{
			Player: player, History: history, Check: c,
			Channel: "MC|Brand", Message: "fabric", SkipDuplicates: true,
		}
		modern := detection.GenericMatchInput{
			Player: player, History: history, Check: c,
			Channel: "minecraft:brand", Message: "fabric", SkipDuplicates: true,
		}

		if got := detection.GenericMatch(legacy); !got {
			t.Fatal("MC|Brand: expected true")
		}
		// Same message but different channel → distinct payload → not suppressed.
		if got := detection.GenericMatch(modern); !got {
			t.Fatal("minecraft:brand same message: expected true")
		}
	})

	t.Run("dedupe_off_allows_repeated_triggers", func(t *testing.T) {
		t.Parallel()

		player := newPlayer()
		history := detection.NewMessageHistory()
		c := check([]string{"LABYMOD"}, "", "")

		in := detection.GenericMatchInput{
			Player:         player,
			History:        history,
			Check:          c,
			Channel:        "LABYMOD",
			Message:        "repeated",
			SkipDuplicates: false, // dedupe disabled
		}

		for i := 0; i < 5; i++ {
			if got := detection.GenericMatch(in); !got {
				t.Fatalf("call %d: expected true when SkipDuplicates=false", i+1)
			}
		}
	})

	t.Run("dedupe_isolated_per_player", func(t *testing.T) {
		t.Parallel()

		history := detection.NewMessageHistory()
		c := check([]string{"LABYMOD"}, "", "")

		playerA := newPlayer()
		playerB := newPlayer()

		inA := detection.GenericMatchInput{
			Player: playerA, History: history, Check: c,
			Channel: "LABYMOD", Message: "same-data", SkipDuplicates: true,
		}
		inB := detection.GenericMatchInput{
			Player: playerB, History: history, Check: c,
			Channel: "LABYMOD", Message: "same-data", SkipDuplicates: true,
		}

		// Player A first trigger.
		if got := detection.GenericMatch(inA); !got {
			t.Fatal("player A first: expected true")
		}
		// Player B first trigger — history is per-player, so B is not suppressed.
		if got := detection.GenericMatch(inB); !got {
			t.Fatal("player B first (different UUID): expected true")
		}
		// Player A second trigger → suppressed.
		if got := detection.GenericMatch(inA); got {
			t.Fatal("player A second: expected false (suppressed)")
		}
		// Player B second trigger → suppressed.
		if got := detection.GenericMatch(inB); got {
			t.Fatal("player B second: expected false (suppressed)")
		}
	})

	t.Run("dedupe_nil_history_with_skip_duplicates_false", func(t *testing.T) {
		t.Parallel()

		// When SkipDuplicates=false, nil history is safe (never consulted).
		player := newPlayer()
		c := check([]string{"LABYMOD"}, "", "")

		in := detection.GenericMatchInput{
			Player:         player,
			History:        nil,
			Check:          c,
			Channel:        "LABYMOD",
			Message:        "data",
			SkipDuplicates: false,
		}
		if got := detection.GenericMatch(in); !got {
			t.Fatal("nil history with SkipDuplicates=false: expected true")
		}
	})

	t.Run("remove_clears_player_history", func(t *testing.T) {
		t.Parallel()

		player := newPlayer()
		history := detection.NewMessageHistory()
		c := check([]string{"LABYMOD"}, "", "")

		in := detection.GenericMatchInput{
			Player: player, History: history, Check: c,
			Channel: "LABYMOD", Message: "data", SkipDuplicates: true,
		}

		// First call records and passes.
		if got := detection.GenericMatch(in); !got {
			t.Fatal("before remove: expected true")
		}
		// Duplicate call → suppressed.
		if got := detection.GenericMatch(in); got {
			t.Fatal("before remove (dup): expected false")
		}

		// Remove clears the player's history.
		history.Remove(player.UUID)

		// After remove, the same payload is fresh again.
		if got := detection.GenericMatch(in); !got {
			t.Fatal("after remove: expected true (history cleared)")
		}
	})
}
