package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunInit_CreatesConfigAndKubeconfigDirs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)
	jsonOutput = false

	var buf bytes.Buffer
	initCmd.SetOut(&buf)
	t.Cleanup(func() { initCmd.SetOut(nil) })

	err := runInit(initCmd, nil)
	require.NoError(t, err)

	configDir := filepath.Join(dir, "k3ctx", "yaml", "config")
	kubeDir := filepath.Join(dir, "k3ctx", "yaml", "kubeconfigs")
	_, errCfg := os.Stat(configDir)
	_, errKube := os.Stat(kubeDir)
	assert.NoError(t, errCfg, "config dir must be created")
	assert.NoError(t, errKube, "kubeconfig dir must be created")
}

func TestRunInit_JSON_EmitsInitializedTrue(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)
	jsonOutput = true
	t.Cleanup(func() { jsonOutput = false })

	var buf bytes.Buffer
	initCmd.SetOut(&buf)
	t.Cleanup(func() { initCmd.SetOut(nil) })

	err := runInit(initCmd, nil)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	assert.Equal(t, true, got["initialized"])
}

func TestRunInit_Idempotent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)
	jsonOutput = false

	var buf bytes.Buffer
	initCmd.SetOut(&buf)
	t.Cleanup(func() { initCmd.SetOut(nil) })

	require.NoError(t, runInit(initCmd, nil))
	require.NoError(t, runInit(initCmd, nil), "running twice must not error")
}
