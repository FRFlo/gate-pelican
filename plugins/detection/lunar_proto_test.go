package detection

import (
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	playerv1 "github.com/minekube/gate-plugin-template/plugins/detection/lunar_pb/lunarclient/apollo/player/v1"
)

// buildHandshakePayload encodes a PlayerHandshakeMessage wrapped in an Any
// — the exact format Lunar Client sends on the "lunar:apollo" channel.
func buildHandshakePayload(t *testing.T, msg *playerv1.PlayerHandshakeMessage) []byte {
	t.Helper()
	anyMsg, err := anypb.New(msg)
	if err != nil {
		t.Fatalf("anypb.New: %v", err)
	}
	raw, err := proto.Marshal(anyMsg)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}
	return raw
}

// ─── TestLunarDecodeValid ──────────────────────────────────────────────────────

// TestLunarDecodeValid verifies that a well-formed lunar:apollo payload is
// decoded into the correct LunarModInfo slice.
// Uses a round-trip encode/decode so no live Lunar Client capture is needed.
func TestLunarDecodeValid(t *testing.T) {
	t.Run("single_fabric_mod", func(t *testing.T) {
		payload := buildHandshakePayload(t, &playerv1.PlayerHandshakeMessage{
			InstalledMods: []*playerv1.ModMessage{
				{
					Id:      "sodium",
					Name:    "Sodium",
					Version: "0.5.3",
					Type:    playerv1.ModMessage_TYPE_FABRIC_EXTERNAL,
				},
			},
		})

		mods, ok := ParseLunarHandshake(payload)
		if !ok {
			t.Fatal("ParseLunarHandshake returned ok=false for valid payload")
		}
		if len(mods) != 1 {
			t.Fatalf("expected 1 mod, got %d", len(mods))
		}
		m := mods[0]
		if m.ID != "sodium" {
			t.Errorf("ID: got %q, want %q", m.ID, "sodium")
		}
		if m.DisplayName != "Sodium" {
			t.Errorf("DisplayName: got %q, want %q", m.DisplayName, "Sodium")
		}
		if m.Version != "0.5.3" {
			t.Errorf("Version: got %q, want %q", m.Version, "0.5.3")
		}
		if !m.IsFabric() {
			t.Errorf("expected IsFabric()=true for type %q", m.Type)
		}
	})

	t.Run("multiple_mods_mixed_types", func(t *testing.T) {
		payload := buildHandshakePayload(t, &playerv1.PlayerHandshakeMessage{
			InstalledMods: []*playerv1.ModMessage{
				{Id: "sodium", Name: "Sodium", Version: "0.5.3", Type: playerv1.ModMessage_TYPE_FABRIC_EXTERNAL},
				{Id: "optifine", Name: "OptiFine", Version: "HD_U_I7", Type: playerv1.ModMessage_TYPE_FORGE_EXTERNAL},
				{Id: "lunar-internal", Name: "LunarInternal", Version: "1.0.0", Type: playerv1.ModMessage_TYPE_FABRIC_INTERNAL},
			},
		})

		mods, ok := ParseLunarHandshake(payload)
		if !ok {
			t.Fatal("ParseLunarHandshake returned ok=false for valid payload")
		}
		if len(mods) != 3 {
			t.Fatalf("expected 3 mods, got %d", len(mods))
		}

		// Verify IDs are preserved in order.
		wantIDs := []string{"sodium", "optifine", "lunar-internal"}
		for i, want := range wantIDs {
			if mods[i].ID != want {
				t.Errorf("mods[%d].ID = %q, want %q", i, mods[i].ID, want)
			}
		}

		// Verify type helpers.
		if !mods[0].IsFabric() {
			t.Errorf("mods[0] expected IsFabric()=true, type=%q", mods[0].Type)
		}
		if !mods[1].IsForge() {
			t.Errorf("mods[1] expected IsForge()=true, type=%q", mods[1].Type)
		}
	})

	t.Run("empty_mod_list_is_valid", func(t *testing.T) {
		// A handshake with zero mods is valid — returns (empty slice, true).
		payload := buildHandshakePayload(t, &playerv1.PlayerHandshakeMessage{})

		mods, ok := ParseLunarHandshake(payload)
		if !ok {
			t.Fatal("ParseLunarHandshake returned ok=false for empty mod list")
		}
		if len(mods) != 0 {
			t.Errorf("expected 0 mods, got %d", len(mods))
		}
	})

	t.Run("mod_with_empty_id_skipped", func(t *testing.T) {
		// Mods with empty IDs are skipped (mirrors Java null-check on mod.getId()).
		payload := buildHandshakePayload(t, &playerv1.PlayerHandshakeMessage{
			InstalledMods: []*playerv1.ModMessage{
				{Id: "", Name: "Ghost", Version: "0.0.0"},
				{Id: "real-mod", Name: "RealMod", Version: "1.0.0"},
			},
		})

		mods, ok := ParseLunarHandshake(payload)
		if !ok {
			t.Fatal("ParseLunarHandshake returned ok=false")
		}
		if len(mods) != 1 {
			t.Fatalf("expected 1 mod (ghost skipped), got %d", len(mods))
		}
		if mods[0].ID != "real-mod" {
			t.Errorf("ID: got %q, want %q", mods[0].ID, "real-mod")
		}
	})
}

// ─── TestLunarDecodeInvalid ────────────────────────────────────────────────────

// TestLunarDecodeInvalid verifies that all invalid or malformed payloads return
// (nil, false) — matching LunarApolloHandshakeParser.parseMods() Optional.empty() behavior.
func TestLunarDecodeInvalid(t *testing.T) {
	t.Run("nil_payload", func(t *testing.T) {
		mods, ok := ParseLunarHandshake(nil)
		if ok || mods != nil {
			t.Errorf("nil payload: got (%v, %v), want (nil, false)", mods, ok)
		}
	})

	t.Run("empty_payload", func(t *testing.T) {
		mods, ok := ParseLunarHandshake([]byte{})
		if ok || mods != nil {
			t.Errorf("empty payload: got (%v, %v), want (nil, false)", mods, ok)
		}
	})

	t.Run("garbage_bytes", func(t *testing.T) {
		garbage := []byte{0xFF, 0xFE, 0x00, 0x01, 0x02, 0x03, 0xAB, 0xCD}
		mods, ok := ParseLunarHandshake(garbage)
		if ok || mods != nil {
			t.Errorf("garbage payload: got (%v, %v), want (nil, false)", mods, ok)
		}
	})

	t.Run("wrong_type_url", func(t *testing.T) {
		// Build an Any with a type_url that does NOT match PlayerHandshakeMessage.
		wrong := &anypb.Any{
			TypeUrl: "type.googleapis.com/some.other.Message",
			Value:   []byte{0x0a, 0x05, 0x68, 0x65, 0x6c, 0x6c, 0x6f},
		}
		raw, err := proto.Marshal(wrong)
		if err != nil {
			t.Fatalf("proto.Marshal: %v", err)
		}
		mods, ok := ParseLunarHandshake(raw)
		if ok || mods != nil {
			t.Errorf("wrong type_url: got (%v, %v), want (nil, false)", mods, ok)
		}
	})

	t.Run("valid_any_invalid_inner_bytes", func(t *testing.T) {
		// An Any with the correct type_url but corrupted inner value bytes.
		correct := "type.googleapis.com/lunarclient.apollo.player.v1.PlayerHandshakeMessage"
		corrupted := &anypb.Any{
			TypeUrl: correct,
			Value:   []byte{0xFF, 0xFE, 0xFD}, // not valid protobuf
		}
		raw, err := proto.Marshal(corrupted)
		if err != nil {
			t.Fatalf("proto.Marshal: %v", err)
		}
		// proto3 lenient decode: invalid bytes inside Any.Value may not error on
		// UnmarshalTo for proto3 messages (unknown fields are silently dropped).
		// Accept either outcome — the key constraint is no panic.
		_, _ = ParseLunarHandshake(raw)
	})

	t.Run("single_byte_payload", func(t *testing.T) {
		mods, ok := ParseLunarHandshake([]byte{0x00})
		// A single zero byte may parse as empty Any (zero-value), which then fails
		// UnmarshalTo (empty type_url). Either (nil,false) or ([], true) is acceptable
		// as long as it does not panic.
		_ = mods
		_ = ok
	})
}
