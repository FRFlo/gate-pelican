package detection

import (
	"strings"
	"sync"

	"github.com/google/uuid"
)

// messagePayload is the value-type key used for per-player duplicate detection.
// It mirrors Java's MessagePayload (channel + message equality).
type messagePayload struct {
	channel string
	message string
}

// MessageHistory tracks previously-seen channel/message payloads per player UUID.
// It mirrors HackedServer's messageHistory ConcurrentHashMap<HackedPlayer, Set<MessagePayload>>.
//
// Thread-safe: all mutations and reads are guarded by a single RWMutex.
type MessageHistory struct {
	mu      sync.RWMutex
	history map[uuid.UUID]map[messagePayload]struct{}
}

// NewMessageHistory creates and returns an empty MessageHistory.
func NewMessageHistory() *MessageHistory {
	return &MessageHistory{
		history: make(map[uuid.UUID]map[messagePayload]struct{}),
	}
}

// IsDuplicate returns true if the (channel, message) pair has already been seen
// for the given player UUID. If it has not been seen, the pair is recorded and
// false is returned.
//
// Semantics mirror Java's isMessageDuplicate:
//
//	Set.add() returns true when the element is new → NOT a duplicate.
//	Set.add() returns false when the element was already present → IS a duplicate.
func (h *MessageHistory) IsDuplicate(id uuid.UUID, channel, message string) bool {
	payload := messagePayload{channel: channel, message: message}

	h.mu.Lock()
	set, ok := h.history[id]
	if !ok {
		set = make(map[messagePayload]struct{})
		h.history[id] = set
	}
	_, seen := set[payload]
	if !seen {
		set[payload] = struct{}{}
	}
	h.mu.Unlock()

	return seen
}

// Remove removes all recorded message history for the given player UUID.
// Call this when a player disconnects (mirrors HackedServer.removePlayer).
func (h *MessageHistory) Remove(id uuid.UUID) {
	h.mu.Lock()
	delete(h.history, id)
	h.mu.Unlock()
}

// Cleanup removes all recorded history for all players.
func (h *MessageHistory) Cleanup() {
	h.mu.Lock()
	h.history = make(map[uuid.UUID]map[messagePayload]struct{})
	h.mu.Unlock()
}

// ─── Matcher ────────────────────────────────────────────────────────────────

// GenericMatchInput carries all inputs required by the generic matcher.
// Keeping them in a struct makes the function signature extensible without
// breaking callers when new fields are added.
type GenericMatchInput struct {
	// Player is the player being evaluated. Must not be nil.
	Player *DetectedPlayer
	// History is the per-session message history used for duplicate suppression.
	// Must not be nil when SkipDuplicates is true.
	History *MessageHistory
	// Check is the GenericCheck definition to evaluate.
	Check GenericCheck
	// Channel is the plugin-message channel identifier from the packet.
	Channel string
	// Message is the decoded string payload from the packet.
	Message string
	// SkipDuplicates, when true, suppresses duplicate (channel,message) pairs per player.
	// Mirrors Config.SKIP_DUPLICATES.toBool() from Java.
	SkipDuplicates bool
}

// GenericMatch evaluates a single GenericCheck against an incoming plugin-message
// payload and returns true if the check passes (i.e. the detection fires).
//
// Ported from Java GenericCheck.pass():
//
//	if channel not in check.Channels → false
//	if messageHas != "" && !containsFold(msg, messageHas) → false
//	if messageNotHas != "" && containsFold(msg, messageNotHas) → false
//	if skipDuplicates && history.IsDuplicate(player, channel, message) → false
//	return true
//
// This function is pure-ish (no side effects on the player or history when
// returning false for the channel/message filters), but it does mutate the
// MessageHistory to record a new payload on the first successful match.
// The player's genericChecks set is NOT mutated here; callers are responsible
// for calling player.AddGenericCheck after acting on a true result.
func GenericMatch(in GenericMatchInput) bool {
	// 1. Channel filter: the packet channel must appear in the check's channel list.
	if !channelMatches(in.Check.Channels, in.Channel) {
		return false
	}

	// 2. message_has filter: payload must contain the required substring (case-insensitive).
	if in.Check.MessageHas != "" && !strings.Contains(strings.ToLower(in.Message), strings.ToLower(in.Check.MessageHas)) {
		return false
	}

	// 3. message_not_has filter: payload must NOT contain the forbidden substring (case-insensitive).
	if in.Check.MessageNotHas != "" && strings.Contains(strings.ToLower(in.Message), strings.ToLower(in.Check.MessageNotHas)) {
		return false
	}

	// 4. Duplicate suppression: if skip_duplicates is enabled, suppress repeated identical payloads.
	if in.SkipDuplicates && in.History != nil && in.History.IsDuplicate(in.Player.UUID, in.Channel, in.Message) {
		return false
	}

	return true
}

// channelMatches reports whether the given channel string appears in the channels list.
// Comparison is exact (not case-insensitive) to match Java's List.contains() semantics.
func channelMatches(channels []string, channel string) bool {
	for _, c := range channels {
		if c == channel {
			return true
		}
	}
	return false
}
