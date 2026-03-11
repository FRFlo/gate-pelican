// Package detection implements HackedServer client/mod detection for Gate.
package detection

import (
	"fmt"
	"os"
	"path/filepath"
)

// submoduleResourcesDir is the relative path (from repo root) to the TOML resources
// inside the HackedServer submodule.
const submoduleResourcesDir = "HackedServer/hackedserver-core/src/main/resources"

// tomlFileNames lists all six TOML files that must be present in the submodule
// resources directory.
var tomlFileNames = [6]string{
	"config.toml",
	"generic.toml",
	"actions.toml",
	"forge.toml",
	"lunar.toml",
	"bedrock.toml",
}

// TOMLPaths holds the fully-resolved absolute paths to every required TOML
// configuration file read from the HackedServer submodule.
// All fields are guaranteed non-empty when returned by ResolveSubmodulePaths
// without error.
type TOMLPaths struct {
	Config  string
	Generic string
	Actions string
	Forge   string
	Lunar   string
	Bedrock string
}

// ResolveSubmodulePaths resolves and validates all six TOML paths that live
// inside the HackedServer git submodule.
//
// baseDir must be the repository root directory (the directory that contains
// the HackedServer submodule folder). In production, pass the executable's
// working directory or the path discovered at startup.
//
// Errors are deterministic and non-silent. When the submodule is missing or
// uninitialised the returned error contains explicit remediation instructions:
//
//	git submodule init
//	git submodule update --recursive
func ResolveSubmodulePaths(baseDir string) (TOMLPaths, error) {
	resourcesDir := filepath.Join(baseDir, filepath.FromSlash(submoduleResourcesDir))

	// Validate the submodule resources directory itself first so we can give a
	// targeted remediation message (instead of a per-file missing error).
	if _, err := os.Stat(resourcesDir); os.IsNotExist(err) {
		return TOMLPaths{}, submoduleMissingError(resourcesDir)
	} else if err != nil {
		return TOMLPaths{}, fmt.Errorf(
			"detection: cannot access submodule resources directory %q: %w", resourcesDir, err,
		)
	}

	paths := [6]string{}
	for i, name := range tomlFileNames {
		p := filepath.Join(resourcesDir, name)
		if err := assertReadable(p); err != nil {
			return TOMLPaths{}, err
		}
		paths[i] = p
	}

	return TOMLPaths{
		Config:  paths[0],
		Generic: paths[1],
		Actions: paths[2],
		Forge:   paths[3],
		Lunar:   paths[4],
		Bedrock: paths[5],
	}, nil
}

// assertReadable verifies that the file at path exists and can be opened for
// reading. It does not consume any file contents.
func assertReadable(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf(
				"detection: required TOML file missing: %q\n"+
					"Ensure the HackedServer submodule is initialised:\n"+
					"  git submodule init\n"+
					"  git submodule update --recursive",
				path,
			)
		}
		return fmt.Errorf("detection: cannot open TOML file %q: %w", path, err)
	}
	_ = f.Close()
	return nil
}

// submoduleMissingError returns a descriptive error with remediation steps
// when the submodule resources directory is not present.
func submoduleMissingError(resourcesDir string) error {
	return fmt.Errorf(
		"detection: HackedServer submodule resources not found at %q\n"+
			"The HackedServer git submodule must be initialised before starting Gate.\n"+
			"Run the following commands from the repository root:\n"+
			"  git submodule init\n"+
			"  git submodule update --recursive",
		resourcesDir,
	)
}
