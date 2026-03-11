package detection

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// repoRoot returns the absolute path to the repository root by walking up from
// the test file's directory.  It relies on the fact that this file lives in
// plugins/detection/ which is two levels below the repo root.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// file = .../plugins/detection/config_toml_paths_test.go
	// two parents = repo root
	root := filepath.Dir(filepath.Dir(filepath.Dir(file)))
	return root
}

// TestResolveSubmodulePaths verifies that all six TOML paths can be resolved
// and that each resolved path actually points to a readable file.
func TestResolveSubmodulePaths(t *testing.T) {
	base := repoRoot(t)

	paths, err := ResolveSubmodulePaths(base)
	if err != nil {
		t.Fatalf("ResolveSubmodulePaths(%q) returned unexpected error: %v", base, err)
	}

	cases := []struct {
		name string
		path string
	}{
		{"Config", paths.Config},
		{"Generic", paths.Generic},
		{"Actions", paths.Actions},
		{"Forge", paths.Forge},
		{"Lunar", paths.Lunar},
		{"Bedrock", paths.Bedrock},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.path == "" {
				t.Fatalf("%s path is empty", tc.name)
			}

			f, err := os.Open(tc.path)
			if err != nil {
				t.Fatalf("%s path %q is not readable: %v", tc.name, tc.path, err)
			}
			_ = f.Close()

			// Sanity-check: path must end with the expected filename.
			expectedSuffix := strings.ToLower(tc.name) + ".toml"
			if !strings.HasSuffix(filepath.ToSlash(tc.path), expectedSuffix) {
				t.Errorf("%s path %q does not end with %q", tc.name, tc.path, expectedSuffix)
			}

			t.Logf("OK: %s → %s", tc.name, tc.path)
		})
	}
}

// TestResolveSubmodulePathsMissing verifies that when the submodule resources
// directory does not exist the resolver returns an explicit, actionable error
// that includes guidance for git submodule init and git submodule update.
func TestResolveSubmodulePathsMissing(t *testing.T) {
	// Use an empty temporary directory as the fake repo root — the submodule
	// directory cannot exist there.
	emptyBase := t.TempDir()

	_, err := ResolveSubmodulePaths(emptyBase)
	if err == nil {
		t.Fatalf("expected an error when submodule is missing, but got nil")
	}

	msg := err.Error()
	t.Logf("Error message:\n%s", msg)

	requiredPhrases := []string{
		"git submodule init",
		"git submodule update",
	}
	for _, phrase := range requiredPhrases {
		if !strings.Contains(msg, phrase) {
			t.Errorf("error message does not contain remediation phrase %q\nGot: %s", phrase, msg)
		}
	}

	// Must not silently swallow the error — it must mention the missing path.
	if !strings.Contains(msg, "HackedServer") {
		t.Errorf("error message does not mention HackedServer submodule path\nGot: %s", msg)
	}
}

// TestResolveSubmodulePathsMissingFile verifies that when the resources
// directory exists but a single TOML file is absent the error is also
// explicit and includes remediation guidance.
func TestResolveSubmodulePathsMissingFile(t *testing.T) {
	// Create the submodule resources directory tree without any actual files.
	fakeBase := t.TempDir()
	resourcesDir := filepath.Join(fakeBase, filepath.FromSlash(submoduleResourcesDir))
	if err := os.MkdirAll(resourcesDir, 0o755); err != nil {
		t.Fatalf("setup: MkdirAll: %v", err)
	}

	_, err := ResolveSubmodulePaths(fakeBase)
	if err == nil {
		t.Fatal("expected an error for missing TOML files, got nil")
	}

	msg := err.Error()
	t.Logf("Error message:\n%s", msg)

	requiredPhrases := []string{
		"git submodule init",
		"git submodule update",
	}
	for _, phrase := range requiredPhrases {
		if !strings.Contains(msg, phrase) {
			t.Errorf("error message does not contain remediation phrase %q\nGot: %s", phrase, msg)
		}
	}
}
