package kubeconfig_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/systemframe/k3ctx/internal/kubeconfig"
)

// --- UpdateKubeconfigServer ---

func TestUpdateKubeconfigServer_UpdatesServerWithDirectIP(t *testing.T) {
	input := `apiVersion: v1
clusters:
- cluster:
    server: https://127.0.0.1:6443
  name: default
`
	result, err := kubeconfig.UpdateKubeconfigServer(input, "10.0.0.1", 6443, false, 0)
	require.NoError(t, err)
	var data map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(result), &data))
	clusters := data["clusters"].([]any)
	cluster := clusters[0].(map[string]any)["cluster"].(map[string]any)
	assert.Equal(t, "https://10.0.0.1:6443", cluster["server"])
}

func TestUpdateKubeconfigServer_UpdatesServerWithLocalhostTunnel(t *testing.T) {
	input := `apiVersion: v1
clusters:
- cluster:
    server: https://192.168.1.100:6443
  name: default
`
	result, err := kubeconfig.UpdateKubeconfigServer(input, "192.168.1.100", 6443, true, 16443)
	require.NoError(t, err)
	var data map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(result), &data))
	clusters := data["clusters"].([]any)
	server := clusters[0].(map[string]any)["cluster"].(map[string]any)["server"]
	assert.Equal(t, "https://127.0.0.1:16443", server)
}

func TestUpdateKubeconfigServer_RaisesErrorForInvalidKubeconfig(t *testing.T) {
	input := `apiVersion: v1
kind: Config
`
	_, err := kubeconfig.UpdateKubeconfigServer(input, "10.0.0.1", 6443, false, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no 'clusters' key")
}

func TestUpdateKubeconfigServer_PreservesOtherFields(t *testing.T) {
	input := `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: CERTDATA
    server: https://old:6443
  name: default
contexts:
- context:
    cluster: default
    user: default
  name: default
`
	result, err := kubeconfig.UpdateKubeconfigServer(input, "10.0.0.1", 6443, false, 0)
	require.NoError(t, err)
	var data map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(result), &data))
	cluster := data["clusters"].([]any)[0].(map[string]any)["cluster"].(map[string]any)
	assert.Equal(t, "CERTDATA", cluster["certificate-authority-data"])
	assert.Len(t, data["contexts"], 1)
}

// --- MergeKubeconfig ---

func TestMergeKubeconfig_CreatesNewWhenMissing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	newCfg := `apiVersion: v1
clusters:
- cluster:
    server: https://127.0.0.1:16443
  name: default
contexts:
- context:
    cluster: default
    user: default
  name: default
users:
- name: default
  user:
    token: test-token
`
	p, err := kubeconfig.MergeKubeconfig(newCfg, "test-context")
	require.NoError(t, err)
	assert.FileExists(t, p)

	data := loadYAML(t, p)
	assert.Equal(t, "test-context", data["current-context"])
	clusters := data["clusters"].([]any)
	assert.Equal(t, "test-context", clusters[0].(map[string]any)["name"])
}

func TestMergeKubeconfig_MergesIntoExisting(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	kubeDir := filepath.Join(tmp, ".kube")
	require.NoError(t, os.Mkdir(kubeDir, 0o700))

	existing := map[string]any{
		"apiVersion":      "v1",
		"clusters":        []any{map[string]any{"name": "existing-cluster", "cluster": map[string]any{"server": "https://old:6443"}}},
		"contexts":        []any{map[string]any{"name": "existing-context", "context": map[string]any{"cluster": "existing-cluster", "user": "existing-user"}}},
		"users":           []any{map[string]any{"name": "existing-user", "user": map[string]any{"token": "old-token"}}},
		"current-context": "existing-context",
	}
	writeYAML(t, filepath.Join(kubeDir, "config"), existing)

	newCfg := `apiVersion: v1
clusters:
- cluster:
    server: https://127.0.0.1:16443
  name: default
contexts:
- context:
    cluster: default
    user: default
  name: default
users:
- name: default
  user:
    token: new-token
`
	p, err := kubeconfig.MergeKubeconfig(newCfg, "new-context")
	require.NoError(t, err)

	data := loadYAML(t, p)
	assert.Len(t, data["clusters"], 2)
	assert.Len(t, data["contexts"], 2)
	assert.Equal(t, "new-context", data["current-context"])
}

func TestMergeKubeconfig_ReplacesExistingContextWithSameName(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	kubeDir := filepath.Join(tmp, ".kube")
	require.NoError(t, os.Mkdir(kubeDir, 0o700))

	existing := map[string]any{
		"apiVersion": "v1",
		"clusters":   []any{map[string]any{"name": "test-context", "cluster": map[string]any{"server": "https://old:6443"}}},
		"contexts":   []any{map[string]any{"name": "test-context", "context": map[string]any{"cluster": "test-context", "user": "test-context"}}},
		"users":      []any{map[string]any{"name": "test-context", "user": map[string]any{"token": "old-token"}}},
	}
	writeYAML(t, filepath.Join(kubeDir, "config"), existing)

	newCfg := `apiVersion: v1
clusters:
- cluster:
    server: https://127.0.0.1:16443
  name: default
contexts:
- context:
    cluster: default
    user: default
  name: default
users:
- name: default
  user:
    token: new-token
`
	p, err := kubeconfig.MergeKubeconfig(newCfg, "test-context")
	require.NoError(t, err)

	data := loadYAML(t, p)
	clusters := data["clusters"].([]any)
	require.Len(t, clusters, 1)
	srv := clusters[0].(map[string]any)["cluster"].(map[string]any)["server"]
	assert.Equal(t, "https://127.0.0.1:16443", srv)
}

func TestMergeKubeconfig_CreatesBackupOfExistingConfig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	kubeDir := filepath.Join(tmp, ".kube")
	require.NoError(t, os.Mkdir(kubeDir, 0o700))

	existing := map[string]any{"apiVersion": "v1", "clusters": []any{}, "contexts": []any{}, "users": []any{}}
	writeYAML(t, filepath.Join(kubeDir, "config"), existing)

	newCfg := `apiVersion: v1
clusters:
- cluster:
    server: https://127.0.0.1:16443
  name: default
contexts:
- context:
    cluster: default
    user: default
  name: default
users:
- name: default
  user:
    token: test
`
	_, err := kubeconfig.MergeKubeconfig(newCfg, "test-context")
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(kubeDir, "config.bak"))
}

// --- GetCurrentContext ---

func TestGetCurrentContext_ReturnsCurrent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	kubeDir := filepath.Join(tmp, ".kube")
	require.NoError(t, os.Mkdir(kubeDir, 0o700))
	writeYAML(t, filepath.Join(kubeDir, "config"), map[string]any{
		"apiVersion":      "v1",
		"current-context": "acme-prod",
	})

	ctx, err := kubeconfig.GetCurrentContext()
	require.NoError(t, err)
	assert.Equal(t, "acme-prod", ctx)
}

func TestGetCurrentContext_EmptyWhenFileMissing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	ctx, err := kubeconfig.GetCurrentContext()
	require.NoError(t, err)
	assert.Equal(t, "", ctx)
}

func TestGetCurrentContext_EmptyWhenFieldAbsent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	kubeDir := filepath.Join(tmp, ".kube")
	require.NoError(t, os.Mkdir(kubeDir, 0o700))
	writeYAML(t, filepath.Join(kubeDir, "config"), map[string]any{"apiVersion": "v1"})

	ctx, err := kubeconfig.GetCurrentContext()
	require.NoError(t, err)
	assert.Equal(t, "", ctx)
}

// --- ContextExists ---

func TestContextExists_ReturnsTrueWhenContextPresent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, ".kube"), 0o700))
	cfg := map[string]any{
		"apiVersion": "v1",
		"contexts": []any{
			map[string]any{"name": "acme-prod"},
			map[string]any{"name": "acme-staging"},
		},
	}
	writeYAML(t, filepath.Join(tmp, ".kube", "config"), cfg)

	assert.True(t, kubeconfig.ContextExists("acme-prod"))
	assert.True(t, kubeconfig.ContextExists("acme-staging"))
}

func TestContextExists_ReturnsFalseWhenContextAbsent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, ".kube"), 0o700))
	cfg := map[string]any{
		"apiVersion": "v1",
		"contexts":   []any{map[string]any{"name": "acme-prod"}},
	}
	writeYAML(t, filepath.Join(tmp, ".kube", "config"), cfg)

	assert.False(t, kubeconfig.ContextExists("acme-staging"))
}

func TestContextExists_ReturnsFalseWhenFileAbsent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	assert.False(t, kubeconfig.ContextExists("any-context"))
}

func loadYAML(t *testing.T, p string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(p) //nolint:gosec // test reads its own tempdir
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, yaml.Unmarshal(data, &out))
	return out
}

func writeYAML(t *testing.T, p string, v any) {
	t.Helper()
	data, err := yaml.Marshal(v)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(p, data, 0o600))
}
