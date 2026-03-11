package detection

import (
	"sync"
	"testing"
	"time"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// ptr64 returns a pointer to the given int64, used to set ActionDef.DelayTicks.
func ptr64(v int64) *int64 { return &v }

// recordingCallbacks returns a set of callbacks that record every invocation
// and a sync.Mutex-protected slice of recorded strings per callback type.
type callbackRecorder struct {
	mu                  sync.Mutex
	alerts              []string
	consoleCommands     []string
	playerCommands      []string
	oppedPlayerCommands []string
}

func newRecorder() *callbackRecorder { return &callbackRecorder{} }

func (r *callbackRecorder) callbacks() ActionCallbacks {
	return ActionCallbacks{
		SendAlert: func(s string) {
			r.mu.Lock()
			r.alerts = append(r.alerts, s)
			r.mu.Unlock()
		},
		ExecuteConsoleCommand: func(s string) {
			r.mu.Lock()
			r.consoleCommands = append(r.consoleCommands, s)
			r.mu.Unlock()
		},
		ExecutePlayerCommand: func(s string) {
			r.mu.Lock()
			r.playerCommands = append(r.playerCommands, s)
			r.mu.Unlock()
		},
		ExecuteOppedPlayerCommand: func(s string) {
			r.mu.Lock()
			r.oppedPlayerCommands = append(r.oppedPlayerCommands, s)
			r.mu.Unlock()
		},
	}
}

// alertOnlyConfig builds a minimal ActionsConfig with a single action that
// only sends an alert and has no delay (ticks = nil → uses global = 0).
func alertOnlyConfig(id, alert string) ActionsConfig {
	return ActionsConfig{
		Actions: map[string]ActionDef{
			id: {
				SendAlert: alert,
			},
		},
	}
}

// zeroSettings returns MainSettings with ActionDelayTicks = 0 (immediate).
func zeroSettings() MainSettings {
	return MainSettings{ActionDelayTicks: 0}
}

// ─── TestActionsExecute ───────────────────────────────────────────────────────

// TestActionsExecute covers synchronous (zero-delay) action execution.
func TestActionsExecute(t *testing.T) {
	t.Run("alert_placeholder_substitution", func(t *testing.T) {
		cfg := alertOnlyConfig("alert", "<red><player> <gray>logged in using <red><name><gray>!")
		exec := newActionExecutorWithClock(cfg, zeroSettings(), ImmediateClock{})
		rec := newRecorder()

		exec.Execute([]string{"alert"}, ActionContext{PlayerName: "Steve", CheckName: "Fabric"}, rec.callbacks())

		if len(rec.alerts) != 1 {
			t.Fatalf("want 1 alert, got %d", len(rec.alerts))
		}
		want := "<red>Steve <gray>logged in using <red>Fabric<gray>!"
		if rec.alerts[0] != want {
			t.Errorf("alert = %q, want %q", rec.alerts[0], want)
		}
	})

	t.Run("console_command_substitution", func(t *testing.T) {
		cfg := ActionsConfig{
			Actions: map[string]ActionDef{
				"kick": {
					Commands: ActionCommands{
						Console: []string{"kick <player> You used <name>"},
					},
				},
			},
		}
		exec := newActionExecutorWithClock(cfg, zeroSettings(), ImmediateClock{})
		rec := newRecorder()

		exec.Execute([]string{"kick"}, ActionContext{PlayerName: "Alex", CheckName: "Forge"}, rec.callbacks())

		if len(rec.consoleCommands) != 1 {
			t.Fatalf("want 1 console command, got %d", len(rec.consoleCommands))
		}
		want := "kick Alex You used Forge"
		if rec.consoleCommands[0] != want {
			t.Errorf("console command = %q, want %q", rec.consoleCommands[0], want)
		}
	})

	t.Run("player_command_substitution", func(t *testing.T) {
		cfg := ActionsConfig{
			Actions: map[string]ActionDef{
				"notify": {
					Commands: ActionCommands{
						Player: []string{"msg <player> You are flagged as <name>"},
					},
				},
			},
		}
		exec := newActionExecutorWithClock(cfg, zeroSettings(), ImmediateClock{})
		rec := newRecorder()

		exec.Execute([]string{"notify"}, ActionContext{PlayerName: "Notch", CheckName: "LunarClient"}, rec.callbacks())

		if len(rec.playerCommands) != 1 {
			t.Fatalf("want 1 player command, got %d", len(rec.playerCommands))
		}
		want := "msg Notch You are flagged as LunarClient"
		if rec.playerCommands[0] != want {
			t.Errorf("player command = %q, want %q", rec.playerCommands[0], want)
		}
	})

	t.Run("opped_player_command_substitution", func(t *testing.T) {
		cfg := ActionsConfig{
			Actions: map[string]ActionDef{
				"op_action": {
					Commands: ActionCommands{
						OppedPlayer: []string{"say <name> detected <player>"},
					},
				},
			},
		}
		exec := newActionExecutorWithClock(cfg, zeroSettings(), ImmediateClock{})
		rec := newRecorder()

		exec.Execute([]string{"op_action"}, ActionContext{PlayerName: "Hero", CheckName: "Bedrock"}, rec.callbacks())

		if len(rec.oppedPlayerCommands) != 1 {
			t.Fatalf("want 1 opped player command, got %d", len(rec.oppedPlayerCommands))
		}
		want := "say Bedrock detected Hero"
		if rec.oppedPlayerCommands[0] != want {
			t.Errorf("opped player command = %q, want %q", rec.oppedPlayerCommands[0], want)
		}
	})

	t.Run("multiple_actions_executed_in_order", func(t *testing.T) {
		cfg := ActionsConfig{
			Actions: map[string]ActionDef{
				"alert": {SendAlert: "alert:<player>"},
				"kick":  {Commands: ActionCommands{Console: []string{"kick <player>"}}},
			},
		}
		exec := newActionExecutorWithClock(cfg, zeroSettings(), ImmediateClock{})
		rec := newRecorder()

		exec.Execute([]string{"alert", "kick"}, ActionContext{PlayerName: "TestPlayer", CheckName: "Vape"}, rec.callbacks())

		if len(rec.alerts) != 1 {
			t.Errorf("want 1 alert, got %d", len(rec.alerts))
		}
		if len(rec.consoleCommands) != 1 {
			t.Errorf("want 1 console cmd, got %d", len(rec.consoleCommands))
		}
		if rec.alerts[0] != "alert:TestPlayer" {
			t.Errorf("alert = %q", rec.alerts[0])
		}
		if rec.consoleCommands[0] != "kick TestPlayer" {
			t.Errorf("console cmd = %q", rec.consoleCommands[0])
		}
	})

	t.Run("unknown_action_id_skipped", func(t *testing.T) {
		cfg := alertOnlyConfig("alert", "hello")
		exec := newActionExecutorWithClock(cfg, zeroSettings(), ImmediateClock{})
		rec := newRecorder()

		exec.Execute([]string{"nonexistent", "alert"}, ActionContext{PlayerName: "P", CheckName: "C"}, rec.callbacks())

		// Only the known "alert" action fires.
		if len(rec.alerts) != 1 {
			t.Errorf("want 1 alert (nonexistent skipped), got %d", len(rec.alerts))
		}
	})

	t.Run("empty_action_ids_no_panic", func(t *testing.T) {
		cfg := alertOnlyConfig("alert", "hello")
		exec := newActionExecutorWithClock(cfg, zeroSettings(), ImmediateClock{})
		rec := newRecorder()

		exec.Execute(nil, ActionContext{PlayerName: "P", CheckName: "C"}, rec.callbacks())
		exec.Execute([]string{}, ActionContext{PlayerName: "P", CheckName: "C"}, rec.callbacks())

		if len(rec.alerts) != 0 {
			t.Errorf("expected 0 alerts, got %d", len(rec.alerts))
		}
	})

	t.Run("nil_callbacks_no_panic", func(t *testing.T) {
		cfg := ActionsConfig{
			Actions: map[string]ActionDef{
				"full": {
					SendAlert: "alert:<player>",
					Commands: ActionCommands{
						Console:     []string{"console <player>"},
						Player:      []string{"player <player>"},
						OppedPlayer: []string{"opped <player>"},
					},
				},
			},
		}
		exec := newActionExecutorWithClock(cfg, zeroSettings(), ImmediateClock{})

		// All callbacks nil — must not panic.
		exec.Execute([]string{"full"}, ActionContext{PlayerName: "P", CheckName: "C"}, ActionCallbacks{})
	})

	t.Run("multiple_commands_all_executed", func(t *testing.T) {
		cfg := ActionsConfig{
			Actions: map[string]ActionDef{
				"multi": {
					Commands: ActionCommands{
						Console: []string{"cmd1 <player>", "cmd2 <player>", "cmd3 <name>"},
					},
				},
			},
		}
		exec := newActionExecutorWithClock(cfg, zeroSettings(), ImmediateClock{})
		rec := newRecorder()

		exec.Execute([]string{"multi"}, ActionContext{PlayerName: "Dave", CheckName: "Wurst"}, rec.callbacks())

		if len(rec.consoleCommands) != 3 {
			t.Fatalf("want 3 console cmds, got %d", len(rec.consoleCommands))
		}
		if rec.consoleCommands[0] != "cmd1 Dave" {
			t.Errorf("[0] = %q", rec.consoleCommands[0])
		}
		if rec.consoleCommands[1] != "cmd2 Dave" {
			t.Errorf("[1] = %q", rec.consoleCommands[1])
		}
		if rec.consoleCommands[2] != "cmd3 Wurst" {
			t.Errorf("[2] = %q", rec.consoleCommands[2])
		}
	})

	t.Run("no_alert_when_send_alert_empty", func(t *testing.T) {
		cfg := ActionsConfig{
			Actions: map[string]ActionDef{
				"silent": {
					Commands: ActionCommands{Console: []string{"ban <player>"}},
				},
			},
		}
		exec := newActionExecutorWithClock(cfg, zeroSettings(), ImmediateClock{})
		rec := newRecorder()

		exec.Execute([]string{"silent"}, ActionContext{PlayerName: "P", CheckName: "C"}, rec.callbacks())

		if len(rec.alerts) != 0 {
			t.Errorf("expected 0 alerts for empty send_alert, got %d", len(rec.alerts))
		}
		if len(rec.consoleCommands) != 1 {
			t.Errorf("expected 1 console cmd, got %d", len(rec.consoleCommands))
		}
	})
}

// ─── TestActionsDelay ─────────────────────────────────────────────────────────

// TestActionsDelay verifies delay resolution and scheduling semantics.
func TestActionsDelay(t *testing.T) {
	t.Run("global_delay_used_when_per_action_nil", func(t *testing.T) {
		// DelayTicks is nil → use global (ActionDelayTicks = 20 ticks = 1s).
		cfg := alertOnlyConfig("alert", "test")
		settings := MainSettings{ActionDelayTicks: 20}

		var recordedDelay time.Duration
		var capturedFn func()
		clk := &capturingClock{
			record: func(d time.Duration, fn func()) {
				recordedDelay = d
				capturedFn = fn
			},
		}

		exec := newActionExecutorWithClock(cfg, settings, clk)
		rec := newRecorder()
		exec.Execute([]string{"alert"}, ActionContext{PlayerName: "P", CheckName: "C"}, rec.callbacks())

		wantDelay := 20 * tickDuration // 1 second
		if recordedDelay != wantDelay {
			t.Errorf("delay = %v, want %v", recordedDelay, wantDelay)
		}

		// Action must not have fired yet (clock did not call fn).
		if len(rec.alerts) != 0 {
			t.Errorf("alert fired before clock tick")
		}

		// Now simulate clock firing.
		if capturedFn != nil {
			capturedFn()
		}
		if len(rec.alerts) != 1 {
			t.Errorf("want 1 alert after clock fires, got %d", len(rec.alerts))
		}
	})

	t.Run("per_action_delay_overrides_global", func(t *testing.T) {
		// DelayTicks = ptr(40) → use 40 ticks, ignoring global.
		cfg := ActionsConfig{
			Actions: map[string]ActionDef{
				"kick_delayed": {
					SendAlert:  "bye <player>",
					DelayTicks: ptr64(40),
				},
			},
		}
		settings := MainSettings{ActionDelayTicks: 20} // global = 20 ticks

		var recordedDelay time.Duration
		clk := &capturingClock{
			record: func(d time.Duration, _ func()) {
				recordedDelay = d
			},
		}

		exec := newActionExecutorWithClock(cfg, settings, clk)
		rec := newRecorder()
		exec.Execute([]string{"kick_delayed"}, ActionContext{PlayerName: "P", CheckName: "C"}, rec.callbacks())

		wantDelay := 40 * tickDuration // 2 seconds
		if recordedDelay != wantDelay {
			t.Errorf("delay = %v, want %v", recordedDelay, wantDelay)
		}
		_ = rec
	})

	t.Run("per_action_zero_delay_overrides_global", func(t *testing.T) {
		// DelayTicks = ptr(0) → explicit zero, override global even if global > 0.
		cfg := ActionsConfig{
			Actions: map[string]ActionDef{
				"instant": {
					SendAlert:  "instant <player>",
					DelayTicks: ptr64(0),
				},
			},
		}
		settings := MainSettings{ActionDelayTicks: 20} // global would be 20

		var recordedDelay time.Duration
		clk := &capturingClock{
			record: func(d time.Duration, _ func()) {
				recordedDelay = d
			},
		}

		exec := newActionExecutorWithClock(cfg, settings, clk)
		rec := newRecorder()
		exec.Execute([]string{"instant"}, ActionContext{PlayerName: "P", CheckName: "C"}, rec.callbacks())

		// 0 ticks → 0 duration (no delay).
		if recordedDelay != 0 {
			t.Errorf("delay = %v, want 0 (explicit zero override)", recordedDelay)
		}
		_ = rec
	})

	t.Run("global_zero_delay_immediate_execution", func(t *testing.T) {
		// DelayTicks nil, global = 0 → ImmediateClock fires synchronously.
		cfg := alertOnlyConfig("alert", "hello <player>")
		settings := MainSettings{ActionDelayTicks: 0}

		exec := newActionExecutorWithClock(cfg, settings, ImmediateClock{})
		rec := newRecorder()
		exec.Execute([]string{"alert"}, ActionContext{PlayerName: "Bob", CheckName: "X"}, rec.callbacks())

		if len(rec.alerts) != 1 {
			t.Fatalf("want 1 alert with zero delay, got %d", len(rec.alerts))
		}
		if rec.alerts[0] != "hello Bob" {
			t.Errorf("alert = %q", rec.alerts[0])
		}
	})

	t.Run("real_clock_fires_asynchronously", func(t *testing.T) {
		// Sanity check: RealClock with a small delay fires asynchronously.
		cfg := ActionsConfig{
			Actions: map[string]ActionDef{
				"async_alert": {
					SendAlert:  "async <player>",
					DelayTicks: ptr64(1), // 1 tick = 50ms
				},
			},
		}
		exec := newActionExecutorWithClock(cfg, zeroSettings(), RealClock{})
		rec := newRecorder()

		exec.Execute([]string{"async_alert"}, ActionContext{PlayerName: "Q", CheckName: "R"}, rec.callbacks())

		// Must not have fired yet (50ms hasn't passed).
		if len(rec.alerts) != 0 {
			t.Error("alert fired synchronously — expected async scheduling")
		}

		// Wait for the timer to fire.
		time.Sleep(200 * time.Millisecond)

		rec.mu.Lock()
		n := len(rec.alerts)
		rec.mu.Unlock()

		if n != 1 {
			t.Errorf("want 1 alert after timer, got %d", n)
		}
	})

	t.Run("multiple_actions_independent_delays", func(t *testing.T) {
		// Two actions with different delays — each fires independently.
		cfg := ActionsConfig{
			Actions: map[string]ActionDef{
				"fast": {SendAlert: "fast <player>", DelayTicks: ptr64(1)},
				"slow": {SendAlert: "slow <player>", DelayTicks: ptr64(4)},
			},
		}
		exec := newActionExecutorWithClock(cfg, zeroSettings(), RealClock{})
		rec := newRecorder()

		exec.Execute([]string{"fast", "slow"}, ActionContext{PlayerName: "Multi", CheckName: "C"}, rec.callbacks())

		// After ~100ms (> 50ms but < 200ms), only "fast" should have fired.
		time.Sleep(100 * time.Millisecond)
		rec.mu.Lock()
		fastCount := len(rec.alerts)
		rec.mu.Unlock()
		if fastCount != 1 {
			t.Errorf("after 100ms: want 1 alert (fast only), got %d", fastCount)
		}

		// After another 300ms total (> 4 ticks = 200ms), both should have fired.
		time.Sleep(300 * time.Millisecond)
		rec.mu.Lock()
		totalCount := len(rec.alerts)
		rec.mu.Unlock()
		if totalCount != 2 {
			t.Errorf("after 400ms total: want 2 alerts, got %d", totalCount)
		}
	})
}

// ─── TestRenderPlaceholders ───────────────────────────────────────────────────

// TestRenderPlaceholders covers the pure substitution helper directly.
func TestRenderPlaceholders(t *testing.T) {
	cases := []struct {
		name       string
		input      string
		playerName string
		checkName  string
		want       string
	}{
		{
			name:       "both_placeholders",
			input:      "<red><player> <gray>logged in using <red><name><gray>!",
			playerName: "Steve",
			checkName:  "Fabric",
			want:       "<red>Steve <gray>logged in using <red>Fabric<gray>!",
		},
		{
			name:       "player_only",
			input:      "kick <player> goodbye",
			playerName: "Alex",
			checkName:  "",
			want:       "kick Alex goodbye",
		},
		{
			name:       "name_only",
			input:      "detected <name>",
			playerName: "",
			checkName:  "Forge",
			want:       "detected Forge",
		},
		{
			name:       "no_placeholders",
			input:      "no substitution needed",
			playerName: "P",
			checkName:  "C",
			want:       "no substitution needed",
		},
		{
			name:       "multiple_occurrences",
			input:      "<player>+<player>=<name>",
			playerName: "Eve",
			checkName:  "Wurst",
			want:       "Eve+Eve=Wurst",
		},
		{
			name:       "empty_string",
			input:      "",
			playerName: "P",
			checkName:  "C",
			want:       "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RenderPlaceholders(tc.input, tc.playerName, tc.checkName)
			if got != tc.want {
				t.Errorf("RenderPlaceholders(%q, %q, %q) = %q, want %q",
					tc.input, tc.playerName, tc.checkName, got, tc.want)
			}
		})
	}
}

// ─── TestResolveDelay ─────────────────────────────────────────────────────────

// TestResolveDelay tests the tick-to-duration math in resolveDelay directly.
func TestResolveDelay(t *testing.T) {
	cases := []struct {
		name        string
		defDelay    *int64
		globalDelay int64
		want        time.Duration
	}{
		{"nil_uses_global_20", nil, 20, 20 * tickDuration},
		{"nil_uses_global_0", nil, 0, 0},
		{"ptr_40_overrides", ptr64(40), 20, 40 * tickDuration},
		{"ptr_0_overrides_global", ptr64(0), 20, 0},
		{"ptr_1_tick", ptr64(1), 0, tickDuration},
		{"negative_global_treated_as_zero", nil, -5, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			exec := newActionExecutorWithClock(
				ActionsConfig{},
				MainSettings{ActionDelayTicks: tc.globalDelay},
				ImmediateClock{},
			)
			def := ActionDef{DelayTicks: tc.defDelay}
			got := exec.resolveDelay(def)
			if got != tc.want {
				t.Errorf("resolveDelay = %v, want %v", got, tc.want)
			}
		})
	}
}

// ─── capturingClock ───────────────────────────────────────────────────────────

// capturingClock records the duration and fn passed to AfterFunc without
// calling fn, letting tests assert on the scheduled delay.
type capturingClock struct {
	record func(d time.Duration, fn func())
}

func (c *capturingClock) AfterFunc(d time.Duration, fn func()) {
	c.record(d, fn)
}
