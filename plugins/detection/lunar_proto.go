package detection

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	playerv1 "github.com/minekube/gate-plugin-template/plugins/detection/lunar_pb/lunarclient/apollo/player/v1"
)

// ParseLunarHandshake decodes a lunar:apollo channel payload into a list of
// LunarModInfo entries.
//
// The Lunar Client sends a serialized google.protobuf.Any wrapping a
// PlayerHandshakeMessage on the "lunar:apollo" plugin channel during login.
// This function mirrors LunarApolloHandshakeParser.parseMods() in Java:
//   - Returns (nil, false) for nil, empty, or malformed payloads.
//   - Returns (nil, false) when the Any.type_url does not match PlayerHandshakeMessage.
//   - Returns (mods, true) — possibly an empty slice — on successful decode.
//
// Callers should use the returned mods to call player.SetLunarMods().
func ParseLunarHandshake(payload []byte) ([]LunarModInfo, bool) {
	if len(payload) == 0 {
		return nil, false
	}

	// The payload is a serialized google.protobuf.Any.
	var anyMsg anypb.Any
	if err := proto.Unmarshal(payload, &anyMsg); err != nil {
		return nil, false
	}

	// Unpack into PlayerHandshakeMessage — fails if type_url does not match.
	var handshake playerv1.PlayerHandshakeMessage
	if err := anyMsg.UnmarshalTo(&handshake); err != nil {
		return nil, false
	}

	mods := make([]LunarModInfo, 0, len(handshake.GetInstalledMods()))
	for _, mod := range handshake.GetInstalledMods() {
		id := mod.GetId()
		if id == "" {
			continue
		}
		mods = append(mods, LunarModInfo{
			ID:          id,
			DisplayName: mod.GetName(),
			Version:     mod.GetVersion(),
			Type:        mod.GetType().String(),
		})
	}
	return mods, true
}
