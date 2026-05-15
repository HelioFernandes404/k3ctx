package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/systemframe/k3ctx/internal/config"
)

func writeYAML(t *testing.T, dir string, data map[string]any) string {
	t.Helper()
	f, err := os.CreateTemp(dir, "*.yaml")
	require.NoError(t, err)
	require.NoError(t, yaml.NewEncoder(f).Encode(data))
	require.NoError(t, f.Close())
	return f.Name()
}

func TestLoadConfig_LoadsValidConfigFile(t *testing.T) {
	p := writeYAML(t, t.TempDir(), map[string]any{
		"remote_k3s_config_path": "/custom/path/k3s.yaml",
		"k3s_api_port":           7443,
		"ssh_key_path":           "~/.ssh/custom_key",
		"port_range_start":       20000,
		"port_range_size":        5000,
	})
	cfg, err := config.LoadConfig(p)
	require.NoError(t, err)
	assert.Equal(t, "/custom/path/k3s.yaml", cfg["remote_k3s_config_path"])
	assert.Equal(t, 7443, cfg["k3s_api_port"])
	assert.Equal(t, 20000, cfg["port_range_start"])
	assert.Equal(t, 5000, cfg["port_range_size"])
}

func TestLoadConfig_ReturnsEmptyForMissingFile(t *testing.T) {
	cfg, err := config.LoadConfig("/nonexistent/path/config.yaml")
	require.NoError(t, err)
	assert.Empty(t, cfg)
}

func TestLoadConfig_EnvVarOverridesFileValue(t *testing.T) {
	p := writeYAML(t, t.TempDir(), map[string]any{
		"k3s_api_port":           7443,
		"remote_k3s_config_path": "/file/path",
	})
	t.Setenv("K3S_API_PORT", "8443")
	cfg, err := config.LoadConfig(p)
	require.NoError(t, err)
	assert.Equal(t, 8443, cfg["k3s_api_port"])
	assert.Equal(t, "/file/path", cfg["remote_k3s_config_path"])
}

func TestLoadConfig_InventoryPathOverriddenByEnvVar(t *testing.T) {
	p := writeYAML(t, t.TempDir(), map[string]any{"inventory_path": "/file/inventory"})
	t.Setenv("INVENTORY_PATH", "/env/inventory/path")
	cfg, err := config.LoadConfig(p)
	require.NoError(t, err)
	assert.Equal(t, "/env/inventory/path", cfg["inventory_path"])
}

func TestLoadConfig_NumericStringsInYAMLNormalized(t *testing.T) {
	p := writeYAML(t, t.TempDir(), map[string]any{
		"k3s_api_port":    "7443",
		"port_range_start": "20000",
		"port_range_size": "5000",
	})
	cfg, err := config.LoadConfig(p)
	require.NoError(t, err)
	assert.IsType(t, 0, cfg["k3s_api_port"])
	assert.Equal(t, 7443, cfg["k3s_api_port"])
}

func TestLoadConfig_MalformedYAMLReturnsEmpty(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "*.yaml")
	require.NoError(t, err)
	_, _ = f.WriteString("invalid: yaml: [")
	require.NoError(t, f.Close())
	cfg, err := config.LoadConfig(f.Name())
	require.NoError(t, err)
	assert.Empty(t, cfg)
}

func TestLoadConfig_NonNumericEnvVarForPortKeptAsString(t *testing.T) {
	t.Setenv("K3S_API_PORT", "not_a_number")
	cfg, err := config.LoadConfig("/nonexistent")
	require.NoError(t, err)
	assert.Equal(t, "not_a_number", cfg["k3s_api_port"])
}

func TestLoadConfig_NegativeEnvVarConverted(t *testing.T) {
	t.Setenv("PORT_RANGE_START", "-1000")
	cfg, err := config.LoadConfig("/nonexistent")
	require.NoError(t, err)
	assert.Equal(t, -1000, cfg["port_range_start"])
}

func TestGetConfigValue_ReturnsValueOrDefault(t *testing.T) {
	cfg := map[string]any{"key": "value"}
	assert.Equal(t, "value", config.GetConfigValue(cfg, "key", "default"))
	assert.Equal(t, "default", config.GetConfigValue(cfg, "missing", "default"))
	assert.Nil(t, config.GetConfigValue(cfg, "missing", nil))
}

func TestResolveInventoryPath_UsesExistingConfiguredPath(t *testing.T) {
	tmp := t.TempDir()
	inv := filepath.Join(tmp, "inventory")
	require.NoError(t, os.Mkdir(inv, 0o755))

	resolved := config.ResolveInventoryPath(map[string]any{"inventory_path": inv}, tmp)
	assert.Equal(t, inv, resolved)
}

func TestResolveInventoryPath_FallsBackToAncestorAnsibleInventory(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, ".custom-tools", "k3s-context-tunnel-manager")
	require.NoError(t, os.MkdirAll(projectDir, 0o755))
	ancestorInv := filepath.Join(root, "ansible", "inventory")
	require.NoError(t, os.MkdirAll(ancestorInv, 0o755))

	resolved := config.ResolveInventoryPath(
		map[string]any{"inventory_path": filepath.Join(root, "missing", "inventory")},
		projectDir,
	)
	assert.Equal(t, ancestorInv, resolved)
}

func TestLoadEffectiveConfig_BuildsCanonicalConfig(t *testing.T) {
	tmp := t.TempDir()
	inv := filepath.Join(tmp, "inventory")
	require.NoError(t, os.Mkdir(inv, 0o755))
	cfgFile := filepath.Join(tmp, "config.yaml")
	require.NoError(t, os.WriteFile(cfgFile, mustYAML(map[string]any{
		"inventory_path":          inv,
		"remote_k3s_config_path": "/file/path/k3s.yaml",
		"ssh_key_path":            "~/.ssh/from-file",
		"k3s_api_port":            7443,
		"port_range_start":        20000,
		"port_range_size":         5000,
	}), 0o644))

	t.Setenv("K3S_API_PORT", "8443")
	t.Setenv("SSH_KEY_PATH", "~/.ssh/from-env")

	cfg, err := config.LoadEffectiveConfig(tmp, cfgFile)
	require.NoError(t, err)
	assert.Equal(t, inv, cfg.InventoryPath)
	assert.Equal(t, "/file/path/k3s.yaml", cfg.RemoteK3sConfigPath)
	assert.Contains(t, cfg.SSHKeyPath, ".ssh/from-env")
	assert.Contains(t, cfg.SSHConfigPath, ".ssh/config")
	assert.Equal(t, 8443, cfg.K3sAPIPort)
	assert.Equal(t, 20000, cfg.PortRangeStart)
	assert.Equal(t, 5000, cfg.PortRangeSize)
}

func TestLoadEffectiveConfig_InvalidNumericEnvUsesDefault(t *testing.T) {
	tmp := t.TempDir()
	cfgFile := filepath.Join(tmp, "config.yaml")
	require.NoError(t, os.WriteFile(cfgFile, mustYAML(map[string]any{
		"k3s_api_port":    7443,
		"port_range_start": 20000,
		"port_range_size": 5000,
	}), 0o644))
	t.Setenv("K3S_API_PORT", "not-a-number")

	cfg, err := config.LoadEffectiveConfig(tmp, cfgFile)
	require.NoError(t, err)
	assert.Equal(t, 6443, cfg.K3sAPIPort)
	assert.Equal(t, 20000, cfg.PortRangeStart)
	assert.Equal(t, 5000, cfg.PortRangeSize)
}

func TestLoadEffectiveConfig_InvalidNumericFileUsesDefault(t *testing.T) {
	tmp := t.TempDir()
	cfgFile := filepath.Join(tmp, "config.yaml")
	require.NoError(t, os.WriteFile(cfgFile, mustYAML(map[string]any{
		"k3s_api_port":    "broken",
		"port_range_start": "also-broken",
		"port_range_size": 5000,
	}), 0o644))

	cfg, err := config.LoadEffectiveConfig(tmp, cfgFile)
	require.NoError(t, err)
	assert.Equal(t, 6443, cfg.K3sAPIPort)
	assert.Equal(t, 16443, cfg.PortRangeStart)
	assert.Equal(t, 5000, cfg.PortRangeSize)
}

func mustYAML(v map[string]any) []byte {
	b, err := yaml.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
