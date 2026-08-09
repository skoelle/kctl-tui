// Package kubeexec wraps kubectl/aws-cli invocations used by kctl-tui.
// All functions here have side effects (they run external processes) and
// are therefore not covered by unit tests; the pure logic they depend on
// lives in the kctl package instead.
//
// Every kubectl-related function takes an explicit context argument
// (passed as --context) instead of relying on/mutating the globally
// active kubectl context. This lets the panel act on multiple resolved
// contexts (e.g. beta and prod) without switching global state back and
// forth.
package kubeexec

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

func runOutput(name string, args ...string) (string, error) {
	logCmd(name, args...)
	cmd := exec.Command(name, args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		logErr(name, err)
		prefix := fmt.Sprintf("%s %s failed: %v", name, strings.Join(args, " "), err)
		msg := prefix
		if s := strings.TrimSpace(stderr.String()); s != "" {
			msg += "\n" + s
		}
		if s := strings.TrimSpace(stdout.String()); s != "" {
			msg += "\n" + s
		}
		return "", fmt.Errorf("%s", msg)
	}
	out := strings.TrimSpace(stdout.String())
	logOutput(name, out)
	return out, nil
}

// kubectlArgs prepends a --context flag when context is non-empty.
func kubectlArgs(context string, args ...string) []string {
	if context == "" {
		return args
	}
	return append([]string{"--context", context}, args...)
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
// namespaces visible in the given context.
func GetNamespacesWithLabels(context string) (map[string]map[string]string, error) {
	args := kubectlArgs(context, "get", "ns", "-o", "json")
	out, err := runOutput("kubectl", args...)
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

// GetDeployments lists deployment names in the given context/namespace.
func GetDeployments(context, namespace string) ([]string, error) {
	args := kubectlArgs(context, "-n", namespace, "get", "deploy",
		"-o", `jsonpath={range .items[*]}{.metadata.name}{"\n"}{end}`)
	out, err := runOutput("kubectl", args...)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return []string{}, nil
	}
	return strings.Split(out, "\n"), nil
}

// RolloutRestart triggers a rolling restart of a deployment.
func RolloutRestart(context, namespace, deployment string) (string, error) {
	args := kubectlArgs(context, "-n", namespace, "rollout", "restart", "deploy/"+deployment)
	return runOutput("kubectl", args...)
}

// RolloutStatus waits for and returns the rollout status of a deployment.
func RolloutStatus(context, namespace, deployment string) (string, error) {
	args := kubectlArgs(context, "-n", namespace, "rollout", "status", "deploy/"+deployment)
	return runOutput("kubectl", args...)
}

// GetSecretAllFields returns all fields of a Kubernetes secret, already
// base64-decoded into plain values.
func GetSecretAllFields(context, namespace, secretName string) (map[string]string, error) {
	args := kubectlArgs(context, "-n", namespace, "get", "secret", secretName, "-o", "json")
	out, err := runOutput("kubectl", args...)
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
func AnnotateForceSync(context, namespace, externalSecretName string, unixTimestamp int64) (string, error) {
	annotation := fmt.Sprintf("force-sync=%d", unixTimestamp)
	args := kubectlArgs(context, "-n", namespace, "annotate", "externalsecret",
		externalSecretName, annotation, "--overwrite")
	return runOutput("kubectl", args...)
}

// GetAWSSecretString fetches the SecretString of an AWS Secrets Manager
// secret via the aws-cli. The secret ID is computed from config templates
// (see internal/config), not looked up interactively.
func GetAWSSecretString(secretID, region string) (string, error) {
	return runOutput("aws", "secretsmanager", "get-secret-value",
		"--secret-id", secretID, "--region", region,
		"--query", "SecretString", "--output", "text")
}

// CheckAWSAuth performs a cheap, fast call to verify the current AWS
// credentials/SSO session are valid. Returns nil if authenticated, or the
// underlying error (e.g. an expired SSO session) otherwise.
func CheckAWSAuth() error {
	_, err := runOutput("aws", "sts", "get-caller-identity", "--query", "Account", "--output", "text")
	return err
}

// RunAWSLogin returns an *exec.Cmd for the given login command (e.g.
// "aws sso login"), split on whitespace. The caller is responsible for
// running it interactively (e.g. via tea.ExecProcess) since SSO login
// typically requires opening a browser and confirming a device code.
func RunAWSLogin(loginCommand string) *exec.Cmd {
	parts := strings.Fields(loginCommand)
	if len(parts) == 0 {
		parts = []string{"aws", "sso", "login"}
	}
	return exec.Command(parts[0], parts[1:]...)
}

// CheckTool verifies that a named executable is available in PATH.
// Returns nil if found, or a descriptive error if not.
func CheckTool(name string) error {
	_, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("%q not found in PATH — please install it first", name)
	}
	return nil
}
