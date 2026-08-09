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
	if len(cfg.Contexts) != 0 || len(cfg.Envs) != 0 {
		t.Fatalf("expected no contexts/envs, got %+v", cfg)
	}
}

func TestLoad_ValidFile(t *testing.T) {
	content := []byte(`
contexts:
  - "internal"
  - "external"
default_context: "internal"
envs:
  - "beta"
  - "prod"
aws_region: "eu-central-1"
aws_account_id: "123456789012"
secret_name_template: "tf-{namespace}-{env}-secrets"
k8s_secret_name_template: "{namespace}-common-secrets"
context_template: "arn:aws:eks:{region}:{account_id}:cluster/tf-{env}-{context}-1"
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
	if len(cfg.Contexts) != 2 || len(cfg.Envs) != 2 {
		t.Fatalf("unexpected contexts/envs: %+v", cfg)
	}
	if cfg.K8sSecretNameTemplate != "{namespace}-common-secrets" {
		t.Fatalf("unexpected k8s secret name template: %q", cfg.K8sSecretNameTemplate)
	}
}

func TestEffectiveDefaultContext(t *testing.T) {
	cfg := Config{Contexts: []string{"internal", "external"}}
	if got := cfg.EffectiveDefaultContext(); got != "internal" {
		t.Fatalf("expected first context as fallback default, got %q", got)
	}

	cfg.DefaultContext = "external"
	if got := cfg.EffectiveDefaultContext(); got != "external" {
		t.Fatalf("expected explicit default_context to win, got %q", got)
	}

	empty := Config{}
	if got := empty.EffectiveDefaultContext(); got != "" {
		t.Fatalf("expected empty string when no contexts configured, got %q", got)
	}
}

func TestResolveContext(t *testing.T) {
	cfg := Config{
		AWSRegion:       "eu-central-1",
		AWSAccountID:    "123456789012",
		ContextTemplate: "arn:aws:eks:{region}:{account_id}:cluster/tf-{env}-{context}-1",
	}
	got := cfg.ResolveContext("beta", "internal")
	want := "arn:aws:eks:eu-central-1:123456789012:cluster/tf-beta-internal-1"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestResolveSecretName(t *testing.T) {
	cfg := Config{SecretNameTemplate: "tf-{namespace}-{env}-secrets"}
	got := cfg.ResolveSecretName("example-ns", "beta")
	want := "tf-example-ns-beta-secrets"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestResolveK8sSecretName_ExplicitTemplate(t *testing.T) {
	cfg := Config{K8sSecretNameTemplate: "{namespace}-common-secrets"}
	got := cfg.ResolveK8sSecretName("example-ns")
	want := "example-ns-common-secrets"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestResolveK8sSecretName_FallsBackToSecretNameTemplate(t *testing.T) {
	cfg := Config{SecretNameTemplate: "tf-{namespace}-{env}-secrets"}
	got := cfg.ResolveK8sSecretName("example-ns")
	want := "tf-example-ns-{env}-secrets" // {env} intentionally left unresolved here
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestLoginCommand_DefaultsWhenUnset(t *testing.T) {
	cfg := Config{}
	if got := cfg.LoginCommand(); got != DefaultAWSSSOLoginCommand {
		t.Fatalf("got %q, want default %q", got, DefaultAWSSSOLoginCommand)
	}

	cfg.AWSSSOLoginCommand = "aws sso login --profile custom"
	if got := cfg.LoginCommand(); got != "aws sso login --profile custom" {
		t.Fatalf("expected custom login command to be used, got %q", got)
	}
}
