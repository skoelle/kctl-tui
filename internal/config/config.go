// Package config loads the user-specific, non-versioned kctl-tui
// configuration (context pairs, team label key) from a YAML file.
package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/skoelle/kctl-tui/internal/kctl"
)

// Config is the root structure of ~/.kctl-tui/config.yaml
type Config struct {
	ContextPairs []kctl.ContextPair `yaml:"context_pairs"`
	TeamLabelKey string              `yaml:"team_label_key"`
}

// DefaultPath returns the default config file location: ~/.kctl-tui/config.yaml
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".kctl-tui", "config.yaml"), nil
}

// Load reads and parses the config file at path. If the file does not
// exist, it returns a zero-value Config (no pairs, empty label key) and no
// error, so the tool can run with sane defaults before the user has set up
// a config file.
func Load(path string) (Config, error) {
	cfg := Config{}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}
