package detection

import (
	"strings"
	"testing"
)

// TestConfigTomlMapping loads all six TOML files from the real submodule and
// verifies key field values for parity with the HackedServer source.
// It uses repoRoot() defined in config_toml_paths_test.go.
func TestConfigTomlMapping(t *testing.T) {
	// Use the submoduleResourcesDir constant (from config_toml_paths.go) with
	// the repo root to build the exact same base directory the loader expects.
	// On Windows, filepath.Join would use backslashes, but our tomlPath uses
	// "/", so we re-use the constant directly with forward slashes.
	base := repoRoot(t)
	baseDir := base + "/" + submoduleResourcesDir
	cfg, err := LoadDetectionConfig(baseDir)
	if err != nil {
		t.Fatalf("LoadDetectionConfig(%q): %v", baseDir, err)
	}

	t.Run("config_toml", func(t *testing.T) {
		s := cfg.Main.Settings
		// Defaults from submodule: action_delay_ticks = 20, skip_duplicates = false
		if s.ActionDelayTicks != 20 {
			t.Errorf("Main.Settings.ActionDelayTicks = %d, want 20", s.ActionDelayTicks)
		}
		if s.SkipDuplicates {
			t.Errorf("Main.Settings.SkipDuplicates = true, want false")
		}
		if s.Debug {
			t.Errorf("Main.Settings.Debug = true, want false")
		}
	})

	t.Run("generic_toml_enabled", func(t *testing.T) {
		if !cfg.Generic.Enabled {
			t.Error("Generic.Enabled = false, want true")
		}
	})

	t.Run("generic_toml_check_count", func(t *testing.T) {
		// The submodule ships 30 checks (one bedrock entry is commented out).
		const wantAtLeast = 25
		if got := len(cfg.Generic.Checks); got < wantAtLeast {
			t.Errorf("Generic.Checks count = %d, want >= %d", got, wantAtLeast)
		}
	})

	t.Run("generic_toml_known_checks", func(t *testing.T) {
		checks := cfg.Generic.Checks
		cases := []struct {
			id       string
			name     string
			category string
			channels []string
		}{
			{"labymod_v1", "Labymod v1", "client", []string{"LABYMOD"}},
			{"fabric", "Fabric", "loader", []string{"MC|Brand", "minecraft:brand"}},
			{"forge", "Forge", "loader", []string{"MC|Brand", "minecraft:brand"}},
			{"badlion_client", "Badlion Client", "client", []string{"MC|Brand", "minecraft:brand"}},
			{"world_downloader_v3", "World Downloader", "mod", []string{"WDL|INIT"}},
		}
		for _, tc := range cases {
			c, ok := checks[tc.id]
			if !ok {
				t.Errorf("Generic.Checks[%q] not found", tc.id)
				continue
			}
			if c.Name != tc.name {
				t.Errorf("Generic.Checks[%q].Name = %q, want %q", tc.id, c.Name, tc.name)
			}
			if c.Category != tc.category {
				t.Errorf("Generic.Checks[%q].Category = %q, want %q", tc.id, c.Category, tc.category)
			}
			if len(c.Channels) != len(tc.channels) {
				t.Errorf("Generic.Checks[%q].Channels = %v, want %v", tc.id, c.Channels, tc.channels)
			}
		}
	})

	t.Run("generic_toml_message_fields", func(t *testing.T) {
		// "forge" has message_has = "forge" and message_not_has = "neoforge"
		forge, ok := cfg.Generic.Checks["forge"]
		if !ok {
			t.Fatal("Generic.Checks[forge] not found")
		}
		if !strings.Contains(forge.MessageHas, "forge") {
			t.Errorf("forge.MessageHas = %q, want to contain 'forge'", forge.MessageHas)
		}
		if !strings.Contains(forge.MessageNotHas, "neoforge") {
			t.Errorf("forge.MessageNotHas = %q, want to contain 'neoforge'", forge.MessageNotHas)
		}
	})

	t.Run("generic_toml_actions", func(t *testing.T) {
		c, ok := cfg.Generic.Checks["labymod_v1"]
		if !ok {
			t.Fatal("Generic.Checks[labymod_v1] not found")
		}
		if len(c.Actions) == 0 || c.Actions[0] != "alert" {
			t.Errorf("Generic.Checks[labymod_v1].Actions = %v, want [alert]", c.Actions)
		}
	})

	t.Run("actions_toml", func(t *testing.T) {
		alert, ok := cfg.Actions.Actions["alert"]
		if !ok {
			t.Fatal("Actions.Actions[alert] not found")
		}
		if alert.SendAlert == "" {
			t.Error("Actions.Actions[alert].SendAlert is empty")
		}
		// The alert send_alert contains "<player>" placeholder.
		if !strings.Contains(alert.SendAlert, "<player>") {
			t.Errorf("alert.SendAlert = %q, want to contain '<player>'", alert.SendAlert)
		}
		// No delay_ticks on the default alert.
		if alert.DelayTicks != nil {
			t.Errorf("alert.DelayTicks = %v, want nil (use global)", alert.DelayTicks)
		}
	})

	t.Run("forge_toml", func(t *testing.T) {
		f := cfg.Forge
		if !f.Enabled {
			t.Error("Forge.Enabled = false, want true")
		}
		if !f.Settings.MarkForge {
			t.Error("Forge.Settings.MarkForge = false, want true")
		}
		if !f.Settings.MarkNeoForge {
			t.Error("Forge.Settings.MarkNeoForge = false, want true")
		}

		// Blacklisted mods must include "forgewurst".
		found := false
		for _, m := range f.Category.Blacklisted.Mods {
			if m == "forgewurst" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Forge.Category.Blacklisted.Mods does not contain 'forgewurst': %v", f.Category.Blacklisted.Mods)
		}

		// Whitelisted mods must include "sodium".
		found = false
		for _, m := range f.Category.Whitelisted.Mods {
			if m == "sodium" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Forge.Category.Whitelisted.Mods does not contain 'sodium': %v", f.Category.Whitelisted.Mods)
		}
	})

	t.Run("forge_toml_colors", func(t *testing.T) {
		if cfg.Forge.Category.Whitelisted.Color != "<green>" {
			t.Errorf("Forge.Category.Whitelisted.Color = %q, want '<green>'", cfg.Forge.Category.Whitelisted.Color)
		}
		if cfg.Forge.Category.Blacklisted.Color != "<red>" {
			t.Errorf("Forge.Category.Blacklisted.Color = %q, want '<red>'", cfg.Forge.Category.Blacklisted.Color)
		}
	})

	t.Run("lunar_toml", func(t *testing.T) {
		l := cfg.Lunar
		if !l.Enabled {
			t.Error("Lunar.Enabled = false, want true")
		}
		if !l.Settings.MarkLunarClient {
			t.Error("Lunar.Settings.MarkLunarClient = false, want true")
		}
		if !l.Settings.MarkFabric {
			t.Error("Lunar.Settings.MarkFabric = false, want true")
		}
		if !l.Settings.MarkForge {
			t.Error("Lunar.Settings.MarkForge = false, want true")
		}
		// mod_actions is declared in TOML but empty by default.
		// Just check the map is initialised (may be nil when empty).
	})

	t.Run("bedrock_toml", func(t *testing.T) {
		b := cfg.Bedrock
		// enabled = false by default in the submodule
		if b.Enabled {
			t.Error("Bedrock.Enabled = true, want false (brand-fallback only)")
		}
		if b.Label != "Bedrock" {
			t.Errorf("Bedrock.Label = %q, want 'Bedrock'", b.Label)
		}
		// actions = [] by default
		if len(b.Actions) != 0 {
			t.Errorf("Bedrock.Actions = %v, want empty", b.Actions)
		}
	})
}

// TestConfigMissingFile verifies that LoadDetectionConfig returns a descriptive
// error — including the full file path — when the base directory does not exist.
func TestConfigMissingFile(t *testing.T) {
	const nonExistent = "/tmp/does-not-exist-detection-test-12345"
	_, err := LoadDetectionConfig(nonExistent)
	if err == nil {
		t.Fatal("LoadDetectionConfig returned nil error for missing directory, want explicit error")
	}

	// Error must mention the missing path so callers can diagnose the problem.
	if !strings.Contains(err.Error(), nonExistent) {
		t.Errorf("error %q does not contain path %q", err.Error(), nonExistent)
	}

	// Error must mention the git submodule remediation hint.
	const hint = "git submodule"
	if !strings.Contains(err.Error(), hint) {
		t.Errorf("error %q does not contain submodule hint %q", err.Error(), hint)
	}
}
