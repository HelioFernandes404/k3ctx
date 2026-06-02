package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestRunStatus_JSON_IncludesCurrentContext(t *testing.T) {
	jsonOutput = true
	t.Cleanup(func() { jsonOutput = false })

	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	kubeDir := filepath.Join(tmp, ".kube")
	require.NoError(t, os.Mkdir(kubeDir, 0o700))
	cfg := map[string]any{"apiVersion": "v1", "current-context": "acme-prod"}
	data, _ := yaml.Marshal(cfg)
	require.NoError(t, os.WriteFile(filepath.Join(kubeDir, "config"), data, 0o600))

	svcs.Status = &mockStatusReader{items: []map[string]any{
		{"context_name": "acme-prod", "tunnel_running": true, "liveness": "live"},
	}}

	var buf bytes.Buffer
	statusCmd.SetOut(&buf)
	t.Cleanup(func() { statusCmd.SetOut(nil) })

	err := runStatus(statusCmd, nil)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	assert.Equal(t, "acme-prod", got["current_context"])
	tunnels := got["tunnels"].([]any)
	assert.Len(t, tunnels, 1)
}

func TestRunStatus_JSON_CurrentContextEmptyWhenNoKubeconfig(t *testing.T) {
	jsonOutput = true
	t.Cleanup(func() { jsonOutput = false })

	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	svcs.Status = &mockStatusReader{items: nil}

	var buf bytes.Buffer
	statusCmd.SetOut(&buf)
	t.Cleanup(func() { statusCmd.SetOut(nil) })

	err := runStatus(statusCmd, nil)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	assert.Equal(t, "", got["current_context"])
}
