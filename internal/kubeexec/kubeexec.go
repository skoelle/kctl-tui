// Package kubeexec wraps kubectl/aws-cli invocations used by kctl-tui.
// All functions here have side effects (they run external processes) and
// are therefore not covered by unit tests; the pure logic they depend on
// lives in the kctl package instead.
package kubeexec

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

func runOutput(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s %s failed: %w\n%s", name, strings.Join(args, " "), err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

// GetContexts returns all configured kubectl context names.
func GetContexts() ([]string, error) {
	out, err := runOutput("kubectl", "config", "get-contexts", "-o", "name")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return []string{}, nil
	}
	return strings.Split(out, "\n"), nil
}

// GetCurrentContext returns the currently active kubectl context, or an
// empty string if none is set.
func GetCurrentContext() string {
	out, _ := runOutput("kubectl", "config", "current-context")
	return out
}

// GetCurrentNamespace returns the namespace bound to the current context,
// defaulting to "default" if unset.
func GetCurrentNamespace() string {
	out, _ := runOutput("kubectl", "config", "view", "--minify", "-o", "jsonpath={..namespace}")
	if out == "" {
		return "default"
	}
	return out
}

// UseContext switches the active kubectl context.
func UseContext(ctx string) error {
	_, err := runOutput("kubectl", "config", "use-context", ctx)
	return err
}

// SetNamespace binds a namespace to the current kubectl context.
func SetNamespace(ns string) error {
	_, err := runOutput("kubectl", "config", "set-context", "--current", "--namespace="+ns)
	return err
}

type nsItem struct {
	Metadata struct {
		Name   string            `json:"name"`
		Labels map[string]string `json:"labels"`
	} `json:"metadata"`
}

type nsList struct {
	Items []nsItem `json:"items"`
}

// GetNamespacesWithLabels returns a map of namespace name -> labels for all
// namespaces visible in the current context.
func GetNamespacesWithLabels() (map[string]map[string]string, error) {
	out, err := runOutput("kubectl", "get", "ns", "-o", "json")
	if err != nil {
		return nil, err
	}
	var list nsList
	if err := json.Unmarshal([]byte(out), &list); err != nil {
		return nil, err
	}
	result := make(map[string]map[string]string, len(list.Items))
	for _, item := range list.Items {
		result[item.Metadata.Name] = item.Metadata.Labels
	}
	return result, nil
}

// GetDeployments lists deployment names in the given namespace.
func GetDeployments(namespace string) ([]string, error) {
	out, err := runOutput("kubectl", "-n", namespace, "get", "deploy",
		"-o", `jsonpath={range .items[*]}{.metadata.name}{"\n"}{end}`)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return []string{}, nil
	}
	return strings.Split(out, "\n"), nil
}

// RolloutRestart triggers a rolling restart of a deployment.
func RolloutRestart(namespace, deployment string) (string, error) {
	return runOutput("kubectl", "-n", namespace, "rollout", "restart", "deploy/"+deployment)
}

// RolloutStatus waits for and returns the rollout status of a deployment.
func RolloutStatus(namespace, deployment string) (string, error) {
	return runOutput("kubectl", "-n", namespace, "rollout", "status", "deploy/"+deployment)
}

// GetSecretValueBase64 returns the raw (still base64-encoded) value of a
// single field in a Kubernetes secret.
func GetSecretValueBase64(namespace, secretName, field string) (string, error) {
	path := fmt.Sprintf("jsonpath={.data.%s}", field)
	return runOutput("kubectl", "-n", namespace, "get", "secret", secretName, "-o", path)
}

// GetSecretAllFields returns all fields of a Kubernetes secret, already
// base64-decoded into plain values.
func GetSecretAllFields(namespace, secretName string) (map[string]string, error) {
	out, err := runOutput("kubectl", "-n", namespace, "get", "secret", secretName, "-o", "json")
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		return nil, err
	}
	result := make(map[string]string, len(parsed.Data))
	for k, v := range parsed.Data {
		decoded, err := DecodeBase64(v)
		if err != nil {
			return nil, fmt.Errorf("failed to decode field %q: %w", k, err)
		}
		result[k] = decoded
	}
	return result, nil
}

// DecodeBase64 decodes a base64-encoded Kubernetes secret value.
func DecodeBase64(value string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

// AnnotateForceSync sets the force-sync annotation on an ExternalSecret
// object to trigger an immediate re-sync from the upstream secret store.
func AnnotateForceSync(namespace, externalSecretName string, unixTimestamp int64) (string, error) {
	annotation := fmt.Sprintf("force-sync=%d", unixTimestamp)
	return runOutput("kubectl", "-n", namespace, "annotate", "externalsecret",
		externalSecretName, annotation, "--overwrite")
}

// ListAWSSecrets returns all AWS Secrets Manager secret names/IDs visible
// in the given region (subject to the caller's IAM permissions).
func ListAWSSecrets(region string) ([]string, error) {
	out, err := runOutput("aws", "secretsmanager", "list-secrets",
		"--region", region, "--query", "SecretList[].Name", "--output", "json")
	if err != nil {
		return nil, err
	}
	var names []string
	if err := json.Unmarshal([]byte(out), &names); err != nil {
		return nil, err
	}
	return names, nil
}

// GetAWSSecretString fetches the SecretString of an AWS Secrets Manager
// secret via the aws-cli.
func GetAWSSecretString(secretID, region string) (string, error) {
	return runOutput("aws", "secretsmanager", "get-secret-value",
		"--secret-id", secretID, "--region", region,
		"--query", "SecretString", "--output", "text")
}
