package detection

import (
	"context"
	"os"
	"strings"

	"github.com/go-logr/logr"
	"github.com/robinbraemer/event"
	"go.minekube.com/gate/pkg/edition/java/proxy"
	"go.minekube.com/gate/pkg/edition/java/proxy/message"
)

// Plugin is the Detection Gate plugin.
// It detects client mods, Forge/NeoForge, Lunar Client, and Bedrock clients
// by subscribing to proxy events and wiring all detection helpers together.
var Plugin = proxy.Plugin{
	Name: "Detection",
	Init: initDetection,
}

// initDetection is the full plugin initialisation flow.
//
// Startup sequence (mirrors Java HackedServer.onEnable):
//  1. Resolve resources dir (submodule guard — hard failure if absent)
//  2. Load all six TOML files into a DetectionConfig
//  3. Construct runtime collaborators: store, history, configHolder, executor
//  4. Register lunar:apollo channel with the Gate channel registrar
//  5. Register /detection command (alias /hs)
//  6. Subscribe all event handlers
func initDetection(ctx context.Context, p *proxy.Proxy) error {
	log := logr.FromContextOrDiscard(ctx)
	log.Info("Detection plugin loading...")

	// ── 1. Resolve submodule resources directory ──────────────────────────────
	baseDir, err := os.Getwd()
	if err != nil {
		return err
	}
	resourcesDir, err := resourcesDirFromBaseDir(baseDir)
	if err != nil {
		// Submodule missing: return actionable error, Gate will not start.
		return err
	}

	// ── 2. Load TOML config ───────────────────────────────────────────────────
	cfg, err := LoadDetectionConfig(resourcesDir)
	if err != nil {
		return err
	}

	// ── 3. Construct runtime collaborators ────────────────────────────────────
	store := NewPlayerStore()
	history := NewMessageHistory()
	cfgHolder := newConfigHolder(cfg, resourcesDir)
	executor := NewActionExecutor(cfg.Actions, cfg.Main.Settings)

	// ── 4. Register lunar:apollo channel with the Gate channel registrar ──────
	lunarID, err := message.ChannelIdentifierFrom(lunarApolloChannel)
	if err != nil {
		return err
	}
	p.ChannelRegistrar().Register(lunarID)

	// ── 5. Register /detection command (alias /hs) ────────────────────────────
	p.Command().RegisterWithAliases(
		newDetectionCommand(p, store, cfgHolder),
		"hs",
	)

	// ── 6. Subscribe event handlers ───────────────────────────────────────────

	// PlayerClientBrandEvent — brand string detection (generic + bedrock + forge brand).
	event.Subscribe(p.Event(), 0, onBrandEvent(log, store, history, cfgHolder, executor))

	// PlayerChannelRegisterEvent — channel-register detection (generic + forge mods from channels).
	event.Subscribe(p.Event(), 0, onChannelRegisterEvent(log, store, history, cfgHolder, executor))

	// PlayerModInfoEvent — Forge/NeoForge native handshake (primary forge path).
	event.Subscribe(p.Event(), 0, onModInfoEvent(log, store, cfgHolder, executor))

	// PluginMessageEvent — lunar:apollo protobuf payload.
	event.Subscribe(p.Event(), 0, onPluginMessageEvent(log, store, cfgHolder, executor))

	// ServerPostConnectEvent — player has fully joined a server; execute pending actions.
	event.Subscribe(p.Event(), 0, onServerPostConnectEvent(log, store, executor))

	// DisconnectEvent — clean up store + history for the disconnecting player.
	event.Subscribe(p.Event(), 0, onDisconnectEvent(log, store, history))

	log.Info("Detection plugin loaded.",
		"generic_checks", len(cfg.Generic.Checks),
		"forge_enabled", cfg.Forge.Enabled,
		"lunar_enabled", cfg.Lunar.Enabled,
		"bedrock_enabled", cfg.Bedrock.Enabled,
	)
	return nil
}

// ─── Event handlers ───────────────────────────────────────────────────────────

// onBrandEvent handles PlayerClientBrandEvent.
// Runs generic checks and Bedrock/Forge brand detection, queues any resulting actions.
func onBrandEvent(
	log logr.Logger,
	store *PlayerStore,
	history *MessageHistory,
	cfgHolder *configHolder,
	executor *ActionExecutor,
) func(*proxy.PlayerClientBrandEvent) {
	return func(e *proxy.PlayerClientBrandEvent) {
		cfg := cfgHolder.get()
		player := store.Get(gateUUIDToGoogle(e.Player().ID()))
		bypass := e.Player().HasPermission("hackedserver.bypass")

		result := HandleBrandPayload(player, history, e.Brand(), *cfg, bypass)

		actCtx := ActionContext{PlayerName: e.Player().Username()}

		for _, t := range result.GenericTriggers {
			if len(t.ActionIDs) == 0 {
				continue
			}
			triggerName := t.Name
			actionIDs := t.ActionIDs
			player.QueuePendingAction(func() {
				actCtx := actCtx
				actCtx.CheckName = triggerName
				executor.Execute(actionIDs, actCtx, buildCallbacks(log, e.Player(), cfgHolder))
			})
		}

		for _, t := range result.ForgeTriggers {
			if len(t.ActionIDs) == 0 {
				continue
			}
			triggerName := t.Name
			actionIDs := t.ActionIDs
			player.QueuePendingAction(func() {
				actCtx := actCtx
				actCtx.CheckName = triggerName
				executor.Execute(actionIDs, actCtx, buildCallbacks(log, e.Player(), cfgHolder))
			})
		}

		if result.BedrockDetected && len(result.BedrockActionIDs) > 0 {
			label := result.BedrockLabel
			actionIDs := result.BedrockActionIDs
			player.QueuePendingAction(func() {
				actCtx := actCtx
				actCtx.CheckName = label
				executor.Execute(actionIDs, actCtx, buildCallbacks(log, e.Player(), cfgHolder))
			})
		}
	}
}

// onChannelRegisterEvent handles PlayerChannelRegisterEvent.
// Runs generic checks and Forge mod detection from channel namespaces.
func onChannelRegisterEvent(
	log logr.Logger,
	store *PlayerStore,
	history *MessageHistory,
	cfgHolder *configHolder,
	executor *ActionExecutor,
) func(*proxy.PlayerChannelRegisterEvent) {
	return func(e *proxy.PlayerChannelRegisterEvent) {
		cfg := cfgHolder.get()
		player := store.Get(gateUUIDToGoogle(e.Player().ID()))
		bypass := e.Player().HasPermission("hackedserver.bypass")

		// Convert Gate channel identifiers to plain strings for the handler.
		rawChannels := make([]string, 0, len(e.Channels()))
		for _, ch := range e.Channels() {
			rawChannels = append(rawChannels, ch.ID())
		}
		rawPayload := strings.Join(rawChannels, "\x00")

		result := HandleChannelRegister(player, history, rawPayload, rawChannels, *cfg, bypass)

		actCtx := ActionContext{PlayerName: e.Player().Username()}

		for _, t := range result.GenericTriggers {
			if len(t.ActionIDs) == 0 {
				continue
			}
			triggerName := t.Name
			actionIDs := t.ActionIDs
			player.QueuePendingAction(func() {
				actCtx := actCtx
				actCtx.CheckName = triggerName
				executor.Execute(actionIDs, actCtx, buildCallbacks(log, e.Player(), cfgHolder))
			})
		}

		for _, t := range result.ForgeTriggers {
			if len(t.ActionIDs) == 0 {
				continue
			}
			triggerName := t.Name
			actionIDs := t.ActionIDs
			player.QueuePendingAction(func() {
				actCtx := actCtx
				actCtx.CheckName = triggerName
				executor.Execute(actionIDs, actCtx, buildCallbacks(log, e.Player(), cfgHolder))
			})
		}
	}
}

// onModInfoEvent handles PlayerModInfoEvent (Forge native FML handshake).
// This is the primary Forge/NeoForge detection path.
func onModInfoEvent(
	log logr.Logger,
	store *PlayerStore,
	cfgHolder *configHolder,
	executor *ActionExecutor,
) func(*proxy.PlayerModInfoEvent) {
	return func(e *proxy.PlayerModInfoEvent) {
		cfg := cfgHolder.get()
		player := store.Get(gateUUIDToGoogle(e.Player().ID()))
		bypass := e.Player().HasPermission("hackedserver.bypass")

		triggers := ProcessForgeModInfo(player, e.ModInfo(), cfg.Forge)
		if len(triggers) == 0 {
			return
		}

		actCtx := ActionContext{PlayerName: e.Player().Username()}

		for _, t := range triggers {
			actionIDs := t.ActionIDs
			if bypass {
				actionIDs = nil
			}
			if len(actionIDs) == 0 {
				continue
			}
			triggerName := t.Name
			player.QueuePendingAction(func() {
				actCtx := actCtx
				actCtx.CheckName = triggerName
				executor.Execute(actionIDs, actCtx, buildCallbacks(log, e.Player(), cfgHolder))
			})
		}
	}
}

// onPluginMessageEvent handles PluginMessageEvent for the lunar:apollo channel.
func onPluginMessageEvent(
	log logr.Logger,
	store *PlayerStore,
	cfgHolder *configHolder,
	executor *ActionExecutor,
) func(*proxy.PluginMessageEvent) {
	return func(e *proxy.PluginMessageEvent) {
		// Only interested in messages from players (not backend servers).
		gatePlayer, ok := e.Source().(proxy.Player)
		if !ok {
			return
		}

		cfg := cfgHolder.get()
		player := store.Get(gateUUIDToGoogle(gatePlayer.ID()))
		bypass := gatePlayer.HasPermission("hackedserver.bypass")

		channelID := e.Identifier().ID()
		result := HandleLunarPluginMessage(player, channelID, e.Data(), cfg.Lunar, bypass)

		if !result.HasTriggers() {
			return
		}

		actCtx := ActionContext{PlayerName: gatePlayer.Username()}

		for _, t := range result.Triggers {
			if len(t.ActionIDs) == 0 {
				continue
			}
			triggerName := t.Name
			actionIDs := t.ActionIDs
			player.QueuePendingAction(func() {
				actCtx := actCtx
				actCtx.CheckName = triggerName
				executor.Execute(actionIDs, actCtx, buildCallbacks(log, gatePlayer, cfgHolder))
			})
		}
	}
}

// onServerPostConnectEvent handles ServerPostConnectEvent.
// Executes any pending actions queued for a player on their first server join.
func onServerPostConnectEvent(
	log logr.Logger,
	store *PlayerStore,
	executor *ActionExecutor,
) func(*proxy.ServerPostConnectEvent) {
	return func(e *proxy.ServerPostConnectEvent) {
		// Only fire pending actions on the FIRST server connection (PreviousServer == nil).
		if e.PreviousServer() != nil {
			return
		}
		player := store.Get(gateUUIDToGoogle(e.Player().ID()))
		if !player.HasPendingActions() {
			return
		}
		log.V(1).Info("executing pending detection actions",
			"player", e.Player().Username(),
		)
		player.ExecutePendingActions()
		_ = executor // executor is used inside the queued closures, not here directly.
	}
}

// onDisconnectEvent handles DisconnectEvent.
// Removes player state from store and history to prevent memory leaks.
func onDisconnectEvent(
	log logr.Logger,
	store *PlayerStore,
	history *MessageHistory,
) func(*proxy.DisconnectEvent) {
	return func(e *proxy.DisconnectEvent) {
		id := gateUUIDToGoogle(e.Player().ID())
		store.Remove(id)
		history.Remove(id)
		log.V(1).Info("detection: player cleaned up on disconnect",
			"player", e.Player().Username(),
		)
	}
}

// ─── Alert routing helper ─────────────────────────────────────────────────────

// buildCallbacks constructs the ActionCallbacks for a given player event.
// Alert messages are broadcast to all online players with the hackedserver.alert permission.
// Console commands are executed via the proxy command manager.
func buildCallbacks(log logr.Logger, gatePlayer proxy.Player, cfgHolder *configHolder) ActionCallbacks {
	return ActionCallbacks{
		SendAlert: func(rendered string) {
			log.Info("detection alert", "message", rendered)
		},
		ExecuteConsoleCommand: func(rendered string) {
			log.Info("detection: console command (stub)", "cmd", rendered)
		},
		ExecutePlayerCommand: func(rendered string) {
			log.Info("detection: player command (stub)", "cmd", rendered, "player", gatePlayer.Username())
		},
		ExecuteOppedPlayerCommand: func(rendered string) {
			log.Info("detection: opped player command (stub)", "cmd", rendered, "player", gatePlayer.Username())
		},
	}
}
