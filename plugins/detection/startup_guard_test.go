package detection

import (
	"strings"
	"testing"
)

// TestStartupGuardPresent verifies the plugin startup guard succeeds when the
// HackedServer submodule TOMLs are resolvable.
//
// This exercises the exact call sequence that initDetection uses:
//
//	os.Getwd() → resourcesDirFromBaseDir(baseDir) → LoadDetectionConfig(resourcesDir)
//
// When the submodule is present all three steps must complete without error and
// the resulting DetectionConfig must be non-zero (at least one generic check
// loaded).
func TestStartupGuardPresent(t *testing.T) {
	base := repoRoot(t)

	// Step 1: resolver must succeed — mirrors resourcesDirFromBaseDir in initDetection.
	resourcesDir, err := resourcesDirFromBaseDir(base)
	if err != nil {
		t.Skipf("submodule not available, skipping TestStartupGuardPresent: %v", err)
	}

	// Step 2: config load must succeed — mirrors LoadDetectionConfig in initDetection.
	cfg, err := LoadDetectionConfig(resourcesDir)
	if err != nil {
		t.Fatalf("startup guard: LoadDetectionConfig failed with submodule present: %v", err)
	}

	// Step 3: config must be usable (non-zero generic checks are a good proxy).
	if len(cfg.Generic.Checks) == 0 {
		t.Error("startup guard: loaded config has zero generic checks; expected at least one")
	}

	t.Logf("startup guard present: OK — resourcesDir=%s, generic checks=%d",
		resourcesDir, len(cfg.Generic.Checks))
}

// TestStartupGuardMissing verifies the plugin startup guard fails with a clear,
// actionable error when the HackedServer submodule TOMLs are unavailable.
//
// The error returned by resourcesDirFromBaseDir must:
//   - Be non-nil
//   - Mention "git submodule init"
//   - Mention "git submodule update --recursive"
//   - Mention "HackedServer" to identify the affected submodule
//
// This ensures the operator receives explicit remediation guidance rather than
// a cryptic "file not found" message when Gate cannot start.
func TestStartupGuardMissing(t *testing.T) {
	// Use an empty temp directory as the fake repo root — the HackedServer
	// submodule directory cannot exist there.
	fakeBase := t.TempDir()

	// Mirrors resourcesDirFromBaseDir(baseDir) in initDetection.
	_, err := resourcesDirFromBaseDir(fakeBase)
	if err == nil {
		t.Fatal("startup guard: expected error when submodule TOMLs are missing, got nil")
	}

	msg := err.Error()
	t.Logf("startup guard missing: error = %s", msg)

	// Validate all required remediation phrases are present.
	requiredPhrases := []string{
		"git submodule init",
		"git submodule update --recursive",
		"HackedServer",
	}
	for _, phrase := range requiredPhrases {
		if !strings.Contains(msg, phrase) {
			t.Errorf("startup guard error missing required phrase %q\nfull error: %s", phrase, msg)
		}
	}
}
