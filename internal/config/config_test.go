package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_MissingFileReturnsDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.ContextPairs) != 0 {
		t.Fatalf("expected no context pairs, got %v", cfg.ContextPairs)
	}
	if cfg.TeamLabelKey != "" {
		t.Fatalf("expected empty team label key, got %q", cfg.TeamLabelKey)
	}
}

func TestLoad_ValidFile(t *testing.T) {
	content := []byte(`
context_pairs:
  - name: "env-pair-1"
    contexts: ["ctx-a", "ctx-b"]
team_label_key: "example.org/team"
`)
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.TeamLabelKey != "example.org/team" {
		t.Fatalf("unexpected team label key: %q", cfg.TeamLabelKey)
	}
	if len(cfg.ContextPairs) != 1 || cfg.ContextPairs[0].Name != "env-pair-1" {
		t.Fatalf("unexpected context pairs: %v", cfg.ContextPairs)
	}
}
