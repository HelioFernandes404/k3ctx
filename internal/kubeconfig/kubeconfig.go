package kubeconfig

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func defaultKubeconfigPath() string {
	return filepath.Join(os.Getenv("HOME"), ".kube", "config")
}

// GetCurrentContext returns the active kubectl context name from ~/.kube/config.
// Returns ("", nil) when the file is absent or the field is missing.
func GetCurrentContext() (string, error) {
	data, err := os.ReadFile(defaultKubeconfigPath())
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read kubeconfig: %w", err)
	}
	var cfg map[string]any
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("parse kubeconfig: %w", err)
	}
	ctx, _ := cfg["current-context"].(string)
	return ctx, nil
}

// UpdateKubeconfigServer rewrites the first cluster's server URL.
// When useLocalhost is true, uses 127.0.0.1:localPort instead of ip:port.
func UpdateKubeconfigServer(yamlText, ip string, port int, useLocalhost bool, localPort int) (string, error) {
	var data map[string]any
	if err := yaml.Unmarshal([]byte(yamlText), &data); err != nil {
		return "", fmt.Errorf("failed to parse kubeconfig: %w", err)
	}
	clusters, ok := data["clusters"]
	if !ok {
		return "", fmt.Errorf("fetched file doesn't look like a kubeconfig (no 'clusters' key)")
	}

	clusterList, ok := clusters.([]any)
	if !ok || len(clusterList) == 0 {
		return "", fmt.Errorf("no 'clusters' key")
	}

	clusterEntry, _ := clusterList[0].(map[string]any)
	clusterData, _ := clusterEntry["cluster"].(map[string]any)
	if clusterData == nil {
		return "", fmt.Errorf("malformed cluster entry")
	}

	if useLocalhost && localPort != 0 {
		clusterData["server"] = fmt.Sprintf("https://127.0.0.1:%d", localPort)
	} else {
		clusterData["server"] = fmt.Sprintf("https://%s:%d", ip, port)
	}

	out, err := marshalYAML(data)
	if err != nil {
		return "", err
	}
	return out, nil
}

// MergeKubeconfig merges newConfigText into ~/.kube/config, renaming entries to contextName.
func MergeKubeconfig(newConfigText, contextName string) (string, error) {
	kubeconfigPath := defaultKubeconfigPath()
	if err := os.MkdirAll(filepath.Dir(kubeconfigPath), 0o700); err != nil {
		return "", err
	}

	var newCfg map[string]any
	if err := yaml.Unmarshal([]byte(newConfigText), &newCfg); err != nil {
		return "", fmt.Errorf("failed to parse new kubeconfig: %w", err)
	}

	// Load or create existing
	existing := map[string]any{}
	if data, err := os.ReadFile(kubeconfigPath); err == nil { //nolint:gosec // kubeconfigPath resolved from user home
		_ = yaml.Unmarshal(data, &existing)
	}
	for _, key := range []string{"clusters", "contexts", "users"} {
		if existing[key] == nil {
			existing[key] = []any{}
		}
	}

	// Rename new entries to contextName
	rename := func(list []any, _ string) []any {
		if len(list) == 0 {
			return list
		}
		entry, _ := list[0].(map[string]any)
		if entry != nil {
			entry["name"] = contextName
		}
		return list
	}

	if cl, ok := newCfg["clusters"].([]any); ok {
		newCfg["clusters"] = rename(cl, "name")
	}
	if ul, ok := newCfg["users"].([]any); ok {
		newCfg["users"] = rename(ul, "name")
	}
	if cl, ok := newCfg["contexts"].([]any); ok {
		if len(cl) > 0 {
			entry, _ := cl[0].(map[string]any)
			if entry != nil {
				entry["name"] = contextName
				if ctx, ok := entry["context"].(map[string]any); ok {
					ctx["cluster"] = contextName
					ctx["user"] = contextName
				}
			}
		}
		newCfg["contexts"] = cl
	}

	// Remove existing entries with the same name
	for _, key := range []string{"clusters", "contexts", "users"} {
		existing[key] = filterByName(existing[key], contextName)
	}

	// Append new entries
	for _, key := range []string{"clusters", "contexts", "users"} {
		if newList, ok := newCfg[key].([]any); ok && len(newList) > 0 {
			existing[key] = append(existing[key].([]any), newList...)
		}
	}

	existing["current-context"] = contextName

	// Backup
	if _, err := os.Stat(kubeconfigPath); err == nil {
		backupPath := kubeconfigPath + ".bak"
		if err := copyFile(kubeconfigPath, backupPath); err != nil {
			return "", err
		}
	}

	out, err := marshalYAML(existing)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(kubeconfigPath, []byte(out), 0o600); err != nil {
		return "", err
	}
	return kubeconfigPath, nil
}

func filterByName(list any, name string) []any {
	items, _ := list.([]any)
	var out []any
	for _, item := range items {
		m, _ := item.(map[string]any)
		if m == nil || m["name"] == name {
			continue
		}
		out = append(out, item)
	}
	if out == nil {
		return []any{}
	}
	return out
}

func marshalYAML(v any) (string, error) {
	data, err := yaml.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("failed to marshal kubeconfig: %w", err)
	}
	return string(data), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src) //nolint:gosec // src is the resolved kubeconfig path
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.Create(dst) //nolint:gosec // dst is the resolved kubeconfig backup path
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
