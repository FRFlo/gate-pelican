package detection

import (
	"errors"
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
)

// genericRaw is the raw decoded shape of generic.toml — a flat map of all
// top-level keys so we can separate the "enabled" flag from check tables.
type genericRaw map[string]interface{}

// actionsRaw is the raw decoded shape of actions.toml — a flat map of all
// top-level action IDs.
type actionsRaw map[string]interface{}

// LoadDetectionConfig loads all six TOML files from the given base directory
// (typically the submodule resources path). Each file path is constructed as
// baseDir/<name>.toml. Returns an explicit, path-contextualised error on any
// failure — including when a file is missing.
func LoadDetectionConfig(baseDir string) (*DetectionConfig, error) {
	cfg := &DetectionConfig{}

	if err := loadTOML(baseDir, "config", &cfg.Main); err != nil {
		return nil, err
	}
	if err := loadGeneric(baseDir, cfg); err != nil {
		return nil, err
	}
	if err := loadActions(baseDir, cfg); err != nil {
		return nil, err
	}
	if err := loadTOML(baseDir, "forge", &cfg.Forge); err != nil {
		return nil, err
	}
	if err := loadTOML(baseDir, "lunar", &cfg.Lunar); err != nil {
		return nil, err
	}
	if err := loadTOML(baseDir, "bedrock", &cfg.Bedrock); err != nil {
		return nil, err
	}

	return cfg, nil
}

// loadTOML reads <baseDir>/<name>.toml and decodes it into dst.
// A missing file returns a descriptive error including the full path.
func loadTOML(baseDir, name string, dst interface{}) error {
	path := tomlPath(baseDir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("detection: required TOML file not found: %q (run 'git submodule update --init' if the HackedServer submodule is missing)", path)
		}
		return fmt.Errorf("detection: reading %q: %w", path, err)
	}
	if err := toml.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("detection: decoding %q: %w", path, err)
	}
	return nil
}

// loadGeneric handles generic.toml specially because the check tables share the
// top-level namespace with the "enabled" key.
func loadGeneric(baseDir string, cfg *DetectionConfig) error {
	path := tomlPath(baseDir, "generic")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("detection: required TOML file not found: %q (run 'git submodule update --init' if the HackedServer submodule is missing)", path)
		}
		return fmt.Errorf("detection: reading %q: %w", path, err)
	}

	var raw genericRaw
	if err := toml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("detection: decoding %q: %w", path, err)
	}

	cfg.Generic.Enabled = true // default; overwritten below if present
	cfg.Generic.Checks = make(map[string]GenericCheck)

	for key, val := range raw {
		switch key {
		case "enabled":
			if b, ok := val.(bool); ok {
				cfg.Generic.Enabled = b
			}
		default:
			// Re-encode the table value back to TOML bytes and decode into GenericCheck.
			checkBytes, err := toml.Marshal(val)
			if err != nil {
				return fmt.Errorf("detection: re-encoding generic check %q from %q: %w", key, path, err)
			}
			var check GenericCheck
			if err := toml.Unmarshal(checkBytes, &check); err != nil {
				return fmt.Errorf("detection: decoding generic check %q from %q: %w", key, path, err)
			}
			cfg.Generic.Checks[key] = check
		}
	}

	return nil
}

// loadActions handles actions.toml specially because each action is a top-level
// table keyed by action ID.
func loadActions(baseDir string, cfg *DetectionConfig) error {
	path := tomlPath(baseDir, "actions")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("detection: required TOML file not found: %q (run 'git submodule update --init' if the HackedServer submodule is missing)", path)
		}
		return fmt.Errorf("detection: reading %q: %w", path, err)
	}

	var raw actionsRaw
	if err := toml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("detection: decoding %q: %w", path, err)
	}

	cfg.Actions.Actions = make(map[string]ActionDef)
	for id, val := range raw {
		// Re-encode and decode into ActionDef.
		actionBytes, err := toml.Marshal(val)
		if err != nil {
			return fmt.Errorf("detection: re-encoding action %q from %q: %w", id, path, err)
		}
		var def ActionDef
		if err := toml.Unmarshal(actionBytes, &def); err != nil {
			return fmt.Errorf("detection: decoding action %q from %q: %w", id, path, err)
		}
		cfg.Actions.Actions[id] = def
	}

	return nil
}

// tomlPath joins baseDir and name to produce a .toml file path.
func tomlPath(baseDir, name string) string {
	return baseDir + "/" + name + ".toml"
}
