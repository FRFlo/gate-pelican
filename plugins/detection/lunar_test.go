package detection

import (
	"testing"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// minimalLunarConfig builds a LunarConfig with enabled=true and mark flags set.
// mod_actions has "sodium" → ["alert"] to test per-mod triggering.
func minimalLunarConfig() LunarConfig {
	return LunarConfig{
		Enabled: true,
		Settings: LunarSettings{
			MarkLunarClient: true,
			MarkFabric:      true,
			MarkForge:       true,
		},
		Actions: LunarActions{
			LunarClient: []string{"alert"},
			Fabric:      []string{"fabric_alert"},
			Forge:       []string{"forge_alert"},
		},
		ModActions: map[string][]string{
			"sodium":     {"alert"},
			"forgewurst": {"alert", "kick"},
			"wurst":      {"kick"},
		},
	}
}

// newLunarTestPlayer creates a fresh DetectedPlayer for each test case.
func newLunarTestPlayer() *DetectedPlayer {
	return newDetectedPlayer(uuid.New())
}

// makeMod is a concise helper to build a LunarModInfo for test setup.
func makeMod(id, version, modType string) LunarModInfo {
	return LunarModInfo{ID: id, DisplayName: id, Version: version, Type: modType}
}

// fabricMod returns a LunarModInfo whose Type contains "FABRIC" — IsFabric() == true.
const fabricType = "TYPE_FABRIC_EXTERNAL"

// forgeMod type constant
const forgeType = "TYPE_FORGE_EXTERNAL"

// lunarOnlyMod is a type that is neither Fabric nor Forge.
const lunarOnlyType = "TYPE_UNKNOWN"

// ---------------------------------------------------------------------------
// TestLunarBlacklist — QA target (blacklisted mod triggers actions)
// ---------------------------------------------------------------------------

// TestLunarBlacklist verifies the happy-path scenarios where the Lunar processor
// produces action triggers for blacklisted/configured mods and marks.
func TestLunarBlacklist(t *testing.T) {
	t.Run("BlacklistedModTriggersActions", testLunarBlacklistedModTriggersActions)
	t.Run("BlacklistedModActionIDs", testLunarBlacklistedModActionIDs)
	t.Run("BlacklistedModName", testLunarBlacklistedModName)
	t.Run("MultipleBlacklistedMods", testLunarMultipleBlacklistedMods)
	t.Run("LunarClientMarkTrigger", testLunarClientMarkTrigger)
	t.Run("FabricMarkTrigger", testLunarFabricMarkTrigger)
	t.Run("ForgeMarkTrigger", testLunarForgeMarkTrigger)
	t.Run("LunarClientGenericCheckSet", testLunarClientGenericCheckSet)
	t.Run("FabricGenericCheckSet", testLunarFabricGenericCheckSet)
	t.Run("ForgeGenericCheckSet", testLunarForgeGenericCheckSet)
	t.Run("ModsPersistedToPlayer", testLunarModsPersistedToPlayer)
	t.Run("PlayerHasLunarDataAfterProcess", testLunarPlayerHasDataAfterProcess)
	t.Run("ForgeModWithVersionInName", testLunarForgeModVersionInName)
}

// testLunarBlacklistedModTriggersActions verifies that a mod listed in
// mod_actions causes a LunarActionTrigger to be returned.
func testLunarBlacklistedModTriggersActions(t *testing.T) {
	player := newLunarTestPlayer()
	cfg := minimalLunarConfig()
	mods := []LunarModInfo{makeMod("sodium", "0.5.3", lunarOnlyType)}

	result := ProcessLunarHandshake(player, mods, cfg)

	found := false
	for _, tr := range result.Triggers {
		if tr.Name == "sodium" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected trigger for 'sodium' but got triggers: %v", result.Triggers)
	}
}

// testLunarBlacklistedModActionIDs verifies the action IDs configured for
// the mod are included in the trigger.
func testLunarBlacklistedModActionIDs(t *testing.T) {
	player := newLunarTestPlayer()
	cfg := minimalLunarConfig()
	mods := []LunarModInfo{makeMod("forgewurst", "1.0", lunarOnlyType)}

	result := ProcessLunarHandshake(player, mods, cfg)

	var trigger *LunarActionTrigger
	for i := range result.Triggers {
		if result.Triggers[i].Name == "forgewurst" {
			trigger = &result.Triggers[i]
			break
		}
	}
	if trigger == nil {
		t.Fatal("expected trigger for 'forgewurst', found none")
	}
	wantIDs := []string{"alert", "kick"}
	if len(trigger.ActionIDs) != len(wantIDs) {
		t.Fatalf("expected actionIDs %v, got %v", wantIDs, trigger.ActionIDs)
	}
	for i, want := range wantIDs {
		if trigger.ActionIDs[i] != want {
			t.Errorf("actionIDs[%d]: want %q, got %q", i, want, trigger.ActionIDs[i])
		}
	}
}

// testLunarBlacklistedModName verifies the Name field uses the lowercase mod ID
// (since ShowModVersions is false in minimalLunarConfig).
func testLunarBlacklistedModName(t *testing.T) {
	player := newLunarTestPlayer()
	cfg := minimalLunarConfig() // ShowModVersions = false
	mods := []LunarModInfo{makeMod("Sodium", "0.5.3", lunarOnlyType)}

	result := ProcessLunarHandshake(player, mods, cfg)

	var found bool
	for _, tr := range result.Triggers {
		if tr.Name == "sodium" { // lower-cased
			found = true
		}
	}
	if !found {
		t.Errorf("expected trigger name 'sodium' (lowercased), got: %v", result.Triggers)
	}
}

// testLunarMultipleBlacklistedMods verifies each mod with a mod_action entry
// gets its own trigger.
func testLunarMultipleBlacklistedMods(t *testing.T) {
	player := newLunarTestPlayer()
	cfg := minimalLunarConfig()
	mods := []LunarModInfo{
		makeMod("sodium", "0.5.3", lunarOnlyType),
		makeMod("forgewurst", "1.0", lunarOnlyType),
		makeMod("wurst", "7.38", lunarOnlyType),
	}

	result := ProcessLunarHandshake(player, mods, cfg)

	// Count mod-action triggers (not client-type triggers)
	modTriggerNames := map[string]bool{}
	for _, tr := range result.Triggers {
		switch tr.Name {
		case "sodium", "forgewurst", "wurst":
			modTriggerNames[tr.Name] = true
		}
	}
	for _, want := range []string{"sodium", "forgewurst", "wurst"} {
		if !modTriggerNames[want] {
			t.Errorf("expected mod trigger %q, not found in: %v", want, result.Triggers)
		}
	}
}

// testLunarClientMarkTrigger verifies that the "Lunar Client" trigger is
// produced on first detection (no prior lunar_client check, no prior data).
func testLunarClientMarkTrigger(t *testing.T) {
	player := newLunarTestPlayer()
	cfg := minimalLunarConfig()
	mods := []LunarModInfo{makeMod("optifine", "1.0", lunarOnlyType)}

	result := ProcessLunarHandshake(player, mods, cfg)

	var found bool
	for _, tr := range result.Triggers {
		if tr.Name == "Lunar Client" {
			found = true
			if len(tr.ActionIDs) == 0 {
				t.Error("Lunar Client trigger has empty ActionIDs")
			}
		}
	}
	if !found {
		t.Errorf("expected 'Lunar Client' trigger, got: %v", result.Triggers)
	}
}

// testLunarFabricMarkTrigger verifies the Fabric trigger is emitted when
// the mod list contains a Fabric-type mod and no prior fabric check exists.
func testLunarFabricMarkTrigger(t *testing.T) {
	player := newLunarTestPlayer()
	cfg := minimalLunarConfig()
	mods := []LunarModInfo{makeMod("fabricmod", "1.0", fabricType)}

	result := ProcessLunarHandshake(player, mods, cfg)

	var found bool
	for _, tr := range result.Triggers {
		if tr.Name == "Fabric" {
			found = true
			if len(tr.ActionIDs) == 0 {
				t.Error("Fabric trigger has empty ActionIDs")
			}
		}
	}
	if !found {
		t.Errorf("expected 'Fabric' trigger, got: %v", result.Triggers)
	}
}

// testLunarForgeMarkTrigger verifies the Forge trigger is emitted when
// the mod list contains a Forge-type mod and no prior forge check exists.
func testLunarForgeMarkTrigger(t *testing.T) {
	player := newLunarTestPlayer()
	cfg := minimalLunarConfig()
	mods := []LunarModInfo{makeMod("forgemod", "1.0", forgeType)}

	result := ProcessLunarHandshake(player, mods, cfg)

	var found bool
	for _, tr := range result.Triggers {
		if tr.Name == "Forge" {
			found = true
			if len(tr.ActionIDs) == 0 {
				t.Error("Forge trigger has empty ActionIDs")
			}
		}
	}
	if !found {
		t.Errorf("expected 'Forge' trigger, got: %v", result.Triggers)
	}
}

// testLunarClientGenericCheckSet verifies the "lunar_client" generic check is
// recorded on the player after processing.
func testLunarClientGenericCheckSet(t *testing.T) {
	player := newLunarTestPlayer()
	cfg := minimalLunarConfig()
	mods := []LunarModInfo{makeMod("sodium", "0.5.3", lunarOnlyType)}

	ProcessLunarHandshake(player, mods, cfg)

	if !player.HasGenericCheck("lunar_client") {
		t.Error("expected player to have 'lunar_client' generic check after processing")
	}
}

// testLunarFabricGenericCheckSet verifies the "fabric" generic check is
// recorded on the player when a Fabric-type mod is detected.
func testLunarFabricGenericCheckSet(t *testing.T) {
	player := newLunarTestPlayer()
	cfg := minimalLunarConfig()
	mods := []LunarModInfo{makeMod("fabricapi", "0.91", fabricType)}

	ProcessLunarHandshake(player, mods, cfg)

	if !player.HasGenericCheck("fabric") {
		t.Error("expected player to have 'fabric' generic check after Fabric mod detected")
	}
}

// testLunarForgeGenericCheckSet verifies the "forge" generic check is
// recorded on the player when a Forge-type mod is detected.
func testLunarForgeGenericCheckSet(t *testing.T) {
	player := newLunarTestPlayer()
	cfg := minimalLunarConfig()
	mods := []LunarModInfo{makeMod("forgehax", "0.2", forgeType)}

	ProcessLunarHandshake(player, mods, cfg)

	if !player.HasGenericCheck("forge") {
		t.Error("expected player to have 'forge' generic check after Forge mod detected")
	}
}

// testLunarModsPersistedToPlayer verifies that after processing, the player's
// lunar mod list reflects the provided mods.
func testLunarModsPersistedToPlayer(t *testing.T) {
	player := newLunarTestPlayer()
	cfg := minimalLunarConfig()
	mods := []LunarModInfo{
		makeMod("sodium", "0.5.3", lunarOnlyType),
		makeMod("iris", "1.6", lunarOnlyType),
	}

	ProcessLunarHandshake(player, mods, cfg)

	if !player.HasLunarMod("sodium") {
		t.Error("expected player.HasLunarMod('sodium') to be true")
	}
	if !player.HasLunarMod("iris") {
		t.Error("expected player.HasLunarMod('iris') to be true")
	}
	if !player.HasLunarModsData() {
		t.Error("expected player.HasLunarModsData() to be true")
	}
}

// testLunarPlayerHasDataAfterProcess verifies HasLunarModsData is true after
// any call (even with an empty mod list).
func testLunarPlayerHasDataAfterProcess(t *testing.T) {
	player := newLunarTestPlayer()
	cfg := minimalLunarConfig()
	mods := []LunarModInfo{} // empty but valid — pure Lunar Client, no mods

	ProcessLunarHandshake(player, mods, cfg)

	if !player.HasLunarModsData() {
		t.Error("expected HasLunarModsData() true after processing (even empty mod list)")
	}
}

// testLunarForgeModVersionInName verifies that when ShowModVersions is true
// the trigger Name includes the version string.
func testLunarForgeModVersionInName(t *testing.T) {
	player := newLunarTestPlayer()
	cfg := minimalLunarConfig()
	cfg.Settings.ShowModVersions = true
	mods := []LunarModInfo{makeMod("forgewurst", "1.2.3", lunarOnlyType)}

	result := ProcessLunarHandshake(player, mods, cfg)

	var found bool
	for _, tr := range result.Triggers {
		if tr.Name == "forgewurst 1.2.3" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected trigger name 'forgewurst 1.2.3' (ShowModVersions=true), got: %v", result.Triggers)
	}
}

// ---------------------------------------------------------------------------
// TestLunarNoBlacklist — QA target (non-blacklisted mods produce no triggers)
// ---------------------------------------------------------------------------

// TestLunarNoBlacklist verifies that the processor produces no action triggers
// for mods not listed in mod_actions, and that idempotency prevents duplicate
// triggers on repeated calls.
func TestLunarNoBlacklist(t *testing.T) {
	t.Run("UnknownModNoModActionTrigger", testLunarUnknownModNoModActionTrigger)
	t.Run("DisabledProcessorNoTriggers", testLunarDisabledProcessorNoTriggers)
	t.Run("NilPlayerNoTriggers", testLunarNilPlayerNoTriggers)
	t.Run("NilModsNoTriggers", testLunarNilModsNoTriggers)
	t.Run("IdempotentLunarClientMark", testLunarIdempotentLunarClientMark)
	t.Run("IdempotentFabricMark", testLunarIdempotentFabricMark)
	t.Run("IdempotentForgeMark", testLunarIdempotentForgeMark)
	t.Run("IdempotentModAction", testLunarIdempotentModAction)
	t.Run("MarkDisabledNoClientTrigger", testLunarMarkDisabledNoClientTrigger)
	t.Run("MarkFabricDisabledNoFabricTrigger", testLunarMarkFabricDisabledNoFabricTrigger)
	t.Run("MarkForgeDisabledNoForgeTrigger", testLunarMarkForgeDisabledNoForgeTrigger)
	t.Run("NoActionsConfiguredNoTrigger", testLunarNoActionsConfiguredNoTrigger)
	t.Run("FabricMarkRequiresFabricMod", testLunarFabricMarkRequiresFabricMod)
	t.Run("ForgeMarkRequiresForgeMod", testLunarForgeMarkRequiresForgeMod)
}

// testLunarUnknownModNoModActionTrigger verifies that a mod with no mod_actions
// entry produces no mod-specific trigger (only client-type triggers fire).
func testLunarUnknownModNoModActionTrigger(t *testing.T) {
	player := newLunarTestPlayer()
	cfg := minimalLunarConfig()
	mods := []LunarModInfo{makeMod("unknownmod", "1.0", lunarOnlyType)}

	result := ProcessLunarHandshake(player, mods, cfg)

	// No mod-action trigger for "unknownmod" — only "Lunar Client" should fire.
	for _, tr := range result.Triggers {
		if tr.Name == "unknownmod" {
			t.Errorf("unexpected trigger for mod without mod_actions entry: %v", tr)
		}
	}
}

// testLunarDisabledProcessorNoTriggers verifies that cfg.Enabled=false
// causes the processor to return an empty result without mutating player.
func testLunarDisabledProcessorNoTriggers(t *testing.T) {
	player := newLunarTestPlayer()
	cfg := minimalLunarConfig()
	cfg.Enabled = false
	mods := []LunarModInfo{makeMod("sodium", "0.5.3", lunarOnlyType)}

	result := ProcessLunarHandshake(player, mods, cfg)

	if result.HasTriggers() {
		t.Errorf("expected no triggers when disabled, got: %v", result.Triggers)
	}
	if player.HasLunarModsData() {
		t.Error("expected player NOT to have lunar data (processor disabled)")
	}
}

// testLunarNilPlayerNoTriggers verifies nil player is handled gracefully.
func testLunarNilPlayerNoTriggers(t *testing.T) {
	cfg := minimalLunarConfig()
	mods := []LunarModInfo{makeMod("sodium", "0.5.3", lunarOnlyType)}

	result := ProcessLunarHandshake(nil, mods, cfg)

	if result.HasTriggers() {
		t.Errorf("expected no triggers with nil player, got: %v", result.Triggers)
	}
}

// testLunarNilModsNoTriggers verifies nil mods slice is handled gracefully.
func testLunarNilModsNoTriggers(t *testing.T) {
	player := newLunarTestPlayer()
	cfg := minimalLunarConfig()

	result := ProcessLunarHandshake(player, nil, cfg)

	if result.HasTriggers() {
		t.Errorf("expected no triggers with nil mods, got: %v", result.Triggers)
	}
}

// testLunarIdempotentLunarClientMark verifies that calling ProcessLunarHandshake
// twice does NOT produce a second "Lunar Client" trigger (idempotent).
func testLunarIdempotentLunarClientMark(t *testing.T) {
	player := newLunarTestPlayer()
	cfg := minimalLunarConfig()
	mods := []LunarModInfo{makeMod("optifine", "1.0", lunarOnlyType)}

	first := ProcessLunarHandshake(player, mods, cfg)
	second := ProcessLunarHandshake(player, mods, cfg)

	firstLunar := countTriggersByName(first.Triggers, "Lunar Client")
	secondLunar := countTriggersByName(second.Triggers, "Lunar Client")

	if firstLunar != 1 {
		t.Errorf("first call: expected 1 'Lunar Client' trigger, got %d", firstLunar)
	}
	if secondLunar != 0 {
		t.Errorf("second call: expected 0 'Lunar Client' triggers (idempotent), got %d", secondLunar)
	}
}

// testLunarIdempotentFabricMark verifies Fabric mark is not re-emitted on the
// second call when the check already exists.
func testLunarIdempotentFabricMark(t *testing.T) {
	player := newLunarTestPlayer()
	cfg := minimalLunarConfig()
	mods := []LunarModInfo{makeMod("fabricapi", "0.91", fabricType)}

	first := ProcessLunarHandshake(player, mods, cfg)
	second := ProcessLunarHandshake(player, mods, cfg)

	firstFabric := countTriggersByName(first.Triggers, "Fabric")
	secondFabric := countTriggersByName(second.Triggers, "Fabric")

	if firstFabric != 1 {
		t.Errorf("first call: expected 1 'Fabric' trigger, got %d", firstFabric)
	}
	if secondFabric != 0 {
		t.Errorf("second call: expected 0 'Fabric' triggers (idempotent), got %d", secondFabric)
	}
}

// testLunarIdempotentForgeMark verifies Forge mark is not re-emitted on repeat.
func testLunarIdempotentForgeMark(t *testing.T) {
	player := newLunarTestPlayer()
	cfg := minimalLunarConfig()
	mods := []LunarModInfo{makeMod("forgemod", "0.1", forgeType)}

	first := ProcessLunarHandshake(player, mods, cfg)
	second := ProcessLunarHandshake(player, mods, cfg)

	firstForge := countTriggersByName(first.Triggers, "Forge")
	secondForge := countTriggersByName(second.Triggers, "Forge")

	if firstForge != 1 {
		t.Errorf("first call: expected 1 'Forge' trigger, got %d", firstForge)
	}
	if secondForge != 0 {
		t.Errorf("second call: expected 0 'Forge' triggers (idempotent), got %d", secondForge)
	}
}

// testLunarIdempotentModAction verifies per-mod actions are not re-triggered
// for mods that were already in the player's previous mod list.
func testLunarIdempotentModAction(t *testing.T) {
	player := newLunarTestPlayer()
	cfg := minimalLunarConfig()
	mods := []LunarModInfo{makeMod("sodium", "0.5.3", lunarOnlyType)}

	first := ProcessLunarHandshake(player, mods, cfg)
	second := ProcessLunarHandshake(player, mods, cfg)

	firstSodium := countTriggersByName(first.Triggers, "sodium")
	secondSodium := countTriggersByName(second.Triggers, "sodium")

	if firstSodium != 1 {
		t.Errorf("first call: expected 1 'sodium' trigger, got %d", firstSodium)
	}
	if secondSodium != 0 {
		t.Errorf("second call: expected 0 'sodium' triggers (idempotent), got %d", secondSodium)
	}
}

// testLunarMarkDisabledNoClientTrigger verifies no "Lunar Client" trigger fires
// when mark_lunar_client is false, even though the processor runs.
func testLunarMarkDisabledNoClientTrigger(t *testing.T) {
	player := newLunarTestPlayer()
	cfg := minimalLunarConfig()
	cfg.Settings.MarkLunarClient = false
	mods := []LunarModInfo{makeMod("optifine", "1.0", lunarOnlyType)}

	result := ProcessLunarHandshake(player, mods, cfg)

	for _, tr := range result.Triggers {
		if tr.Name == "Lunar Client" {
			t.Errorf("unexpected 'Lunar Client' trigger when mark_lunar_client=false: %v", tr)
		}
	}
	if player.HasGenericCheck("lunar_client") {
		t.Error("expected 'lunar_client' check NOT set when mark_lunar_client=false")
	}
}

// testLunarMarkFabricDisabledNoFabricTrigger verifies no Fabric trigger when
// mark_fabric=false.
func testLunarMarkFabricDisabledNoFabricTrigger(t *testing.T) {
	player := newLunarTestPlayer()
	cfg := minimalLunarConfig()
	cfg.Settings.MarkFabric = false
	mods := []LunarModInfo{makeMod("fabricapi", "0.91", fabricType)}

	result := ProcessLunarHandshake(player, mods, cfg)

	for _, tr := range result.Triggers {
		if tr.Name == "Fabric" {
			t.Errorf("unexpected 'Fabric' trigger when mark_fabric=false: %v", tr)
		}
	}
	if player.HasGenericCheck("fabric") {
		t.Error("expected 'fabric' check NOT set when mark_fabric=false")
	}
}

// testLunarMarkForgeDisabledNoForgeTrigger verifies no Forge trigger when
// mark_forge=false.
func testLunarMarkForgeDisabledNoForgeTrigger(t *testing.T) {
	player := newLunarTestPlayer()
	cfg := minimalLunarConfig()
	cfg.Settings.MarkForge = false
	mods := []LunarModInfo{makeMod("forgemod", "0.1", forgeType)}

	result := ProcessLunarHandshake(player, mods, cfg)

	for _, tr := range result.Triggers {
		if tr.Name == "Forge" {
			t.Errorf("unexpected 'Forge' trigger when mark_forge=false: %v", tr)
		}
	}
	if player.HasGenericCheck("forge") {
		t.Error("expected 'forge' check NOT set when mark_forge=false")
	}
}

// testLunarNoActionsConfiguredNoTrigger verifies that when lunar_client actions
// list is empty, the "Lunar Client" trigger is NOT added (but the check IS set).
func testLunarNoActionsConfiguredNoTrigger(t *testing.T) {
	player := newLunarTestPlayer()
	cfg := minimalLunarConfig()
	cfg.Actions.LunarClient = nil // no actions configured
	mods := []LunarModInfo{makeMod("optifine", "1.0", lunarOnlyType)}

	result := ProcessLunarHandshake(player, mods, cfg)

	for _, tr := range result.Triggers {
		if tr.Name == "Lunar Client" {
			t.Errorf("unexpected 'Lunar Client' trigger when actions list is empty: %v", tr)
		}
	}
	// The generic check should still be set even though no action fires.
	if !player.HasGenericCheck("lunar_client") {
		t.Error("expected 'lunar_client' check to be set even when no actions configured")
	}
}

// testLunarFabricMarkRequiresFabricMod verifies the Fabric trigger is NOT emitted
// when the mod list contains no Fabric-type mods (even if mark_fabric=true).
func testLunarFabricMarkRequiresFabricMod(t *testing.T) {
	player := newLunarTestPlayer()
	cfg := minimalLunarConfig() // mark_fabric = true
	mods := []LunarModInfo{makeMod("optifine", "1.0", lunarOnlyType)}

	result := ProcessLunarHandshake(player, mods, cfg)

	for _, tr := range result.Triggers {
		if tr.Name == "Fabric" {
			t.Errorf("unexpected 'Fabric' trigger when no Fabric mods present: %v", tr)
		}
	}
}

// testLunarForgeMarkRequiresForgeMod verifies the Forge trigger is NOT emitted
// when the mod list contains no Forge-type mods (even if mark_forge=true).
func testLunarForgeMarkRequiresForgeMod(t *testing.T) {
	player := newLunarTestPlayer()
	cfg := minimalLunarConfig() // mark_forge = true
	mods := []LunarModInfo{makeMod("optifine", "1.0", lunarOnlyType)}

	result := ProcessLunarHandshake(player, mods, cfg)

	for _, tr := range result.Triggers {
		if tr.Name == "Forge" {
			t.Errorf("unexpected 'Forge' trigger when no Forge mods present: %v", tr)
		}
	}
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// countTriggersByName counts how many triggers match the given name.
func countTriggersByName(triggers []LunarActionTrigger, name string) int {
	n := 0
	for _, tr := range triggers {
		if tr.Name == name {
			n++
		}
	}
	return n
}
