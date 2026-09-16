// Copyright (c) 2026 Stefan Koelle (https://stefankoelle.de)
// Licensed under the MIT License. See LICENSE file in project root for details.

// Package config loads the user-specific, non-versioned kctl-tui
// configuration (contexts, envs, templates) from a YAML file.
package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/skoelle/kctl-tui/internal/kctl"
)

// DefaultAWSSSOLoginCommand is used when the user has not configured a
// custom login command in their config.yaml.
const DefaultAWSSSOLoginCommand = "aws sso login"

// Config is the root structure of ~/.kctl-tui/config.yaml
type Config struct {
	// Contexts are the top-level groupings the tool starts from, e.g.
	// "internal"/"external". This is the outermost navigation level.
	Contexts []string `yaml:"contexts"`

	// DefaultContext is pre-selected on startup so the tool can jump
	// straight to team selection; falls back to the first entry of
	// Contexts if empty.
	DefaultContext string `yaml:"default_context"`

	// Envs are the environments switchable from the control panel, e.g.
	// "beta"/"prod". The first two entries are used for the two k9s
	// status panes.
	Envs []string `yaml:"envs"`

	// AWSRegion is used for all AWS Secrets Manager calls.
	AWSRegion string `yaml:"aws_region"`

	// AWSAccountID fills the {account_id} placeholder in ContextTemplate.
	AWSAccountID string `yaml:"aws_account_id"`

	// SecretNameTemplate builds the AWS Secrets Manager secret ID from a
	// namespace and env, e.g. "tf-{namespace}-{env}-secrets".
	SecretNameTemplate string `yaml:"secret_name_template"`

	// K8sSecretNameTemplate builds the Kubernetes secret name from a
	// namespace, e.g. "{namespace}-common-secrets". Kept separate from
	// SecretNameTemplate because the two sides commonly follow different
	// naming conventions.
	K8sSecretNameTemplate string `yaml:"k8s_secret_name_template"`

	// ContextTemplate builds the actual kubectl context name/ARN from
	// region, account_id, env, and context, e.g.
	// "arn:aws:eks:{region}:{account_id}:cluster/tf-{env}-{context}-1".
	ContextTemplate string `yaml:"context_template"`

	// TeamLabelKey is the namespace label used to group namespaces by
	// team/ownership in the team-selection screen.
	TeamLabelKey string `yaml:"team_label_key"`

	// AWSSSOLoginCommand is run interactively if an AWS auth check fails
	// before the secrets workflow (e.g. an expired SSO session).
	AWSSSOLoginCommand string `yaml:"aws_sso_login_command"`

	// AutoUpdateCheck controls whether kctl-tui checks for updates on
	// startup. Defaults to true when omitted.
	AutoUpdateCheck *bool `yaml:"auto_update_check"`
}

// IsAutoUpdateCheckEnabled returns true unless the user has explicitly set
// auto_update_check to false in their config.
func (c Config) IsAutoUpdateCheckEnabled() bool {
	if c.AutoUpdateCheck == nil {
		return true
	}
	return *c.AutoUpdateCheck
}

// LoginCommand returns the configured AWS SSO login command, falling back
// to DefaultAWSSSOLoginCommand if none is set.
func (c Config) LoginCommand() string {
	if c.AWSSSOLoginCommand == "" {
		return DefaultAWSSSOLoginCommand
	}
	return c.AWSSSOLoginCommand
}

// EffectiveDefaultContext returns DefaultContext if set, otherwise the
// first entry of Contexts, otherwise an empty string.
func (c Config) EffectiveDefaultContext() string {
	if c.DefaultContext != "" {
		return c.DefaultContext
	}
	if len(c.Contexts) > 0 {
		return c.Contexts[0]
	}
	return ""
}

// ResolveContext builds the actual kubectl context name/ARN for a given
// env + context (e.g. "beta" + "internal") using ContextTemplate.
func (c Config) ResolveContext(env, context string) string {
	return kctl.ResolveTemplate(c.ContextTemplate, map[string]string{
		"region":     c.AWSRegion,
		"account_id": c.AWSAccountID,
		"env":        env,
		"context":    context,
	})
}

// ResolveSecretName builds the AWS Secrets Manager secret ID for a given
// namespace + env using SecretNameTemplate.
func (c Config) ResolveSecretName(namespace, env string) string {
	return kctl.ResolveTemplate(c.SecretNameTemplate, map[string]string{
		"namespace": namespace,
		"env":       env,
	})
}

// ResolveK8sSecretName builds the Kubernetes secret name for a given
// namespace using K8sSecretNameTemplate. Falls back to
// SecretNameTemplate resolved without an env placeholder if
// K8sSecretNameTemplate is not configured, so existing configs keep
// working, though setting it explicitly is recommended since the two
// naming conventions usually differ.
func (c Config) ResolveK8sSecretName(namespace string) string {
	template := c.K8sSecretNameTemplate
	if template == "" {
		template = c.SecretNameTemplate
	}
	return kctl.ResolveTemplate(template, map[string]string{
		"namespace": namespace,
	})
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
// exist, it returns a zero-value Config and no error, so the tool can
// still start (with an explanatory error surfaced later where a required
// field turns out to be missing) before the user has set up a config
// file.
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
