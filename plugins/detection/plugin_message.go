package detection

// lunarApolloChannel is the plugin-message channel ID used by Lunar Client's
// Apollo integration. Java mapping: LunarApolloHandshakeParser.CHANNEL.
const lunarApolloChannel = "lunar:apollo"

// LunarRouteResult is the output of HandleLunarPluginMessage.
// It mirrors the BrandHandlerResult / ChannelHandlerResult shape used in T6.
type LunarRouteResult struct {
	// Triggers are the action triggers produced by the lunar intelligence processor.
	// Empty when the channel was not lunar:apollo, the payload was invalid, or no
	// actionable detections occurred.
	Triggers []LunarActionTrigger
}

// HasTriggers reports whether any triggers were produced.
func (r LunarRouteResult) HasTriggers() bool {
	return len(r.Triggers) > 0
}

// HandleLunarPluginMessage routes an incoming plugin-message to the Lunar
// Apollo path when the channel matches "lunar:apollo".
//
// It is a pure-logic function (no Gate proxy calls, no event subscriptions):
//   - Returns an empty result for any channel other than "lunar:apollo".
//   - Returns an empty result when ParseLunarHandshake fails.
//   - Delegates to ProcessLunarHandshake when decode succeeds.
//
// The bypass flag mirrors Java's hackedserver.bypass permission:
//   - When bypass is true, player state is still mutated (mods persisted,
//     generic checks recorded) but ActionIDs in all returned triggers are
//     cleared, preventing actual action execution.
//
// Java mapping:
//
//	CustomPayloadListener.onPluginMessageReceived → handleLunarApollo branch
//	LunarApolloHandshakeParser.parseMods(data).ifPresent(...)
//	LunarHandshakeProcessor.process(hackedPlayer, mods)
func HandleLunarPluginMessage(
	player *DetectedPlayer,
	channelID string,
	data []byte,
	cfg LunarConfig,
	bypass bool,
) LunarRouteResult {
	// Only handle the lunar:apollo channel.
	if !equalFold(channelID, lunarApolloChannel) {
		return LunarRouteResult{}
	}

	// Decode the protobuf payload.
	mods, ok := ParseLunarHandshake(data)
	if !ok {
		return LunarRouteResult{}
	}

	// Run the intelligence processor.
	result := ProcessLunarHandshake(player, mods, cfg)
	if !result.HasTriggers() {
		return LunarRouteResult{}
	}

	triggers := result.Triggers
	if bypass {
		// Clear action IDs but preserve trigger metadata (name, etc.) so the
		// caller can still observe that detection fired.
		cleared := make([]LunarActionTrigger, len(triggers))
		for i, t := range triggers {
			cleared[i] = LunarActionTrigger{Name: t.Name, ActionIDs: nil}
		}
		triggers = cleared
	}

	return LunarRouteResult{Triggers: triggers}
}

// equalFold is a case-insensitive ASCII string comparison helper used for
// channel ID matching (channel IDs are always ASCII in Minecraft).
func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca == cb {
			continue
		}
		// Fold ASCII letters only.
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
