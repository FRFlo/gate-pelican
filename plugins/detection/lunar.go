package detection

import "strings"

// ─── Action trigger ───────────────────────────────────────────────────────────

// LunarActionTrigger represents a pending action output from the Lunar intelligence
// processor. It carries the human-readable display name (used in alert placeholders)
// and the list of action IDs (keys into ActionsConfig.Actions) to execute.
//
// Callers queue these as closures via DetectedPlayer.QueuePendingAction rather
// than executing them inline — execution happens at first server join (Task 14).
type LunarActionTrigger struct {
	// Name is the human-readable identifier for the trigger
	// (e.g. "Lunar Client", "Fabric", "Forge", "sodium 0.5.3").
	Name string
	// ActionIDs is the list of action IDs from lunar.toml (or mod_actions) to execute.
	ActionIDs []string
}

// ─── Lunar Handshake Result ───────────────────────────────────────────────────

// LunarHandshakeResult is the structured output of ProcessLunarHandshake.
// It mirrors Java's LunarHandshakeResult: a (possibly empty) list of triggers.
type LunarHandshakeResult struct {
	// Triggers is the list of action triggers produced by this handshake.
	// Empty when no actionable detections occurred (Lunar check already known,
	// no mod_actions configured, processor disabled, etc.).
	Triggers []LunarActionTrigger
}

// HasTriggers reports whether any triggers were produced.
func (r LunarHandshakeResult) HasTriggers() bool {
	return len(r.Triggers) > 0
}

// ─── Processor ────────────────────────────────────────────────────────────────

// ProcessLunarHandshake is the Lunar Client intelligence processor.
// It is called after ParseLunarHandshake successfully decodes the lunar:apollo
// channel payload and maps the decoded mods to player state + action triggers.
//
// It is a pure-logic function (no event subscriptions, no Gate proxy calls):
//   - Snapshots relevant pre-update state to determine what is genuinely new.
//   - Calls player.SetLunarMods to persist the mod list.
//   - Returns zero or more LunarActionTrigger values for the caller (Task 12/14)
//     to queue as pending actions via player.QueuePendingAction.
//
// Idempotency: marks/triggers are not re-emitted for state that was already
// recorded (lunar_client mark exists, fabric/forge already seen, mod already in
// previous list). Repeated calls with identical data produce no triggers.
//
// Returns an empty LunarHandshakeResult when:
//   - player or mods is nil
//   - cfg.Enabled is false
//
// Java mapping: LunarHandshakeProcessor.process(HackedPlayer, List<LunarModInfo>)
func ProcessLunarHandshake(player *DetectedPlayer, mods []LunarModInfo, cfg LunarConfig) LunarHandshakeResult {
	if player == nil || mods == nil || !cfg.Enabled {
		return LunarHandshakeResult{}
	}

	// ── Snapshot pre-update state ──────────────────────────────────────────
	// Mirror Java: capture what was true BEFORE setLunarMods mutates the player.
	hadLunarData := player.HasLunarModsData()
	previousMods := player.LunarMods()
	hadFabric := containsLunarFabric(previousMods)
	hadForge := containsLunarForge(previousMods)

	hadLunarCheck := player.HasGenericCheck("lunar_client")
	hadFabricCheck := player.HasGenericCheck("fabric")
	hadForgeCheck := player.HasGenericCheck("forge")

	// ── Persist new mod list ───────────────────────────────────────────────
	// This replaces the prior list and sets lunarModsKnown = true.
	// Java: player.setLunarMods(mods)
	player.SetLunarMods(mods)

	// ── Compute post-update flags ──────────────────────────────────────────
	hasFabric := containsLunarFabric(mods)
	hasForge := containsLunarForge(mods)

	var triggers []LunarActionTrigger

	// ── Lunar Client mark ──────────────────────────────────────────────────
	// Java: if (LunarConfig.shouldMarkLunarClient())
	//         addGenericCheck when !hadLunarCheck
	//         trigger when !hadLunarCheck && !hadLunarData
	if cfg.Settings.MarkLunarClient {
		if !hadLunarCheck {
			player.AddGenericCheck("lunar_client")
		}
		if !hadLunarCheck && !hadLunarData {
			if len(cfg.Actions.LunarClient) > 0 {
				triggers = append(triggers, LunarActionTrigger{
					Name:      "Lunar Client",
					ActionIDs: cfg.Actions.LunarClient,
				})
			}
		}
	}

	// ── Fabric mark ────────────────────────────────────────────────────────
	// Java: if (LunarConfig.shouldMarkFabric() && hasFabric)
	//         addGenericCheck when !hadFabricCheck
	//         trigger when !hadFabricCheck && !hadFabric
	if cfg.Settings.MarkFabric && hasFabric {
		if !hadFabricCheck {
			player.AddGenericCheck("fabric")
		}
		if !hadFabricCheck && !hadFabric {
			if len(cfg.Actions.Fabric) > 0 {
				triggers = append(triggers, LunarActionTrigger{
					Name:      "Fabric",
					ActionIDs: cfg.Actions.Fabric,
				})
			}
		}
	}

	// ── Forge mark ─────────────────────────────────────────────────────────
	// Java: if (LunarConfig.shouldMarkForge() && hasForge)
	//         addGenericCheck when !hadForgeCheck
	//         trigger when !hadForgeCheck && !hadForge
	if cfg.Settings.MarkForge && hasForge {
		if !hadForgeCheck {
			player.AddGenericCheck("forge")
		}
		if !hadForgeCheck && !hadForge {
			if len(cfg.Actions.Forge) > 0 {
				triggers = append(triggers, LunarActionTrigger{
					Name:      "Forge",
					ActionIDs: cfg.Actions.Forge,
				})
			}
		}
	}

	// ── Per-mod actions ────────────────────────────────────────────────────
	// Java: iterate mods, skip empty IDs and mods already in previousIds,
	//       look up mod_actions by normalized mod ID.
	if len(mods) > 0 {
		// Build set of previously-known mod IDs (case-insensitive, matching
		// LunarConfig.normalizeModId in Java which uses toLowerCase).
		prevIDs := make(map[string]struct{}, len(previousMods))
		for _, m := range previousMods {
			if m.ID != "" {
				prevIDs[strings.ToLower(m.ID)] = struct{}{}
			}
		}

		for _, mod := range mods {
			modID := strings.ToLower(mod.ID)
			if modID == "" {
				continue
			}
			if _, alreadyKnown := prevIDs[modID]; alreadyKnown {
				continue
			}
			actionIDs, ok := cfg.ModActions[modID]
			if !ok || len(actionIDs) == 0 {
				continue
			}
			triggers = append(triggers, LunarActionTrigger{
				Name:      formatLunarMod(mod, cfg),
				ActionIDs: actionIDs,
			})
		}
	}

	return LunarHandshakeResult{Triggers: triggers}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// containsLunarFabric reports whether any mod in the list is a Fabric mod.
// Java mapping: LunarHandshakeProcessor.containsFabric
func containsLunarFabric(mods []LunarModInfo) bool {
	for _, m := range mods {
		if m.IsFabric() {
			return true
		}
	}
	return false
}

// containsLunarForge reports whether any mod in the list is a Forge mod.
// Java mapping: LunarHandshakeProcessor.containsForge
func containsLunarForge(mods []LunarModInfo) bool {
	for _, m := range mods {
		if m.IsForge() {
			return true
		}
	}
	return false
}

// formatLunarMod returns the display-ready mod string for use in alert placeholders.
// Includes version if cfg.Settings.ShowModVersions is true.
// Java mapping: LunarConfig.formatMod(LunarModInfo)
func formatLunarMod(mod LunarModInfo, cfg LunarConfig) string {
	name := strings.ToLower(mod.ID)
	if cfg.Settings.ShowModVersions && mod.Version != "" {
		name = name + " " + mod.Version
	}
	return name
}
