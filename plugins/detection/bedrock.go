package detection

import "strings"

// bedrockBrandTokens are the lowercase substrings in a client brand that
// identify a Bedrock player connecting through a geyser bridge.
// Matching is case-insensitive contains-check against each token.
var bedrockBrandTokens = []string{"geyser"}

// IsBedrock reports whether the given client brand string identifies a
// Bedrock client. Detection is brand-string-only (no geyser/floodgate APIs).
//
// A brand matches if it contains any of the bedrockBrandTokens
// (case-insensitive). An empty brand never matches.
func IsBedrock(brand string) bool {
	if brand == "" {
		return false
	}
	lower := strings.ToLower(brand)
	for _, token := range bedrockBrandTokens {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

// ApplyBedrockBrand runs Bedrock brand detection for a player.
// It returns true and mutates the player's state if the brand matches and
// bedrock detection is enabled in cfg.
//
// Callers (event handlers) pass the BedrockConfig from the loaded
// DetectionConfig. The returned action IDs come from cfg.Actions and
// cfg.Label is available for placeholder expansion downstream.
//
// No external APIs are consulted; detection is purely based on brand string.
func ApplyBedrockBrand(player *DetectedPlayer, brand string, cfg BedrockConfig) (detected bool, label string, actionIDs []string) {
	if !cfg.Enabled {
		return false, "", nil
	}
	if !IsBedrock(brand) {
		return false, "", nil
	}
	player.SetBedrockDetected(true)
	return true, cfg.Label, cfg.Actions
}
