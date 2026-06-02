package paths

import (
	"os"
	"path/filepath"
)

const (
	appName           = "k3ctx"
	yamlDirname       = "yaml"
	configDirname     = "config"
	kubeconfigDirname = "kubeconfigs"
	legacyConfigDir   = ".k3ctx-config"
)

// DataHome returns the XDG data root ($XDG_DATA_HOME or ~/.local/share).
func DataHome() string {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return filepath.Clean(v)
	}
	return filepath.Join(os.Getenv("HOME"), ".local", "share")
}

// AppDataDir returns the k3ctx application data directory under DataHome.
func AppDataDir() string {
	return filepath.Join(DataHome(), appName)
}

// YamlStorageDir returns the directory holding k3ctx YAML state.
func YamlStorageDir() string {
	return filepath.Join(AppDataDir(), yamlDirname)
}

// UserDataConfigDir returns the directory that holds the user config.yaml.
func UserDataConfigDir() string {
	return filepath.Join(YamlStorageDir(), configDirname)
}

// ConfigFilePathXDG returns the resolved XDG-style config file path.
// Honors K3CTX_CONFIG_DIR when set.
func ConfigFilePathXDG() string {
	if v := os.Getenv("K3CTX_CONFIG_DIR"); v != "" {
		return filepath.Join(filepath.Clean(v), "config.yaml")
	}
	return filepath.Join(UserDataConfigDir(), "config.yaml")
}

// KubeconfigCacheDir returns the directory where per-context kubeconfigs are cached.
func KubeconfigCacheDir() string {
	return filepath.Join(YamlStorageDir(), kubeconfigDirname)
}

// KubeconfigCachePath returns the cache path for a given context name.
func KubeconfigCachePath(contextName string) string {
	return filepath.Join(KubeconfigCacheDir(), contextName+".yml")
}

// LegacyHomeConfigFilePath returns the legacy ~/.k3ctx-config/config.yaml path.
func LegacyHomeConfigFilePath() string {
	return filepath.Join(os.Getenv("HOME"), legacyConfigDir, "config.yaml")
}

// TelemetryDir returns the directory used for telemetry event files.
func TelemetryDir() string {
	return filepath.Join(AppDataDir(), "telemetry")
}

// DefaultConfigCandidates returns config paths in resolution order.
func DefaultConfigCandidates(projectDir string) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range []string{
		ConfigFilePathXDG(),
		UserDataConfigDir() + "/config.yaml",
		filepath.Join(projectDir, "config.yaml"),
		LegacyHomeConfigFilePath(),
	} {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}
