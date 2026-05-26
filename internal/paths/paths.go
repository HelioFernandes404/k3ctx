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

func DataHome() string {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return filepath.Clean(v)
	}
	return filepath.Join(os.Getenv("HOME"), ".local", "share")
}

func AppDataDir() string {
	return filepath.Join(DataHome(), appName)
}

func YamlStorageDir() string {
	return filepath.Join(AppDataDir(), yamlDirname)
}

func UserDataConfigDir() string {
	return filepath.Join(YamlStorageDir(), configDirname)
}

func ConfigFilePathXDG() string {
	if v := os.Getenv("K3CTX_CONFIG_DIR"); v != "" {
		return filepath.Join(filepath.Clean(v), "config.yaml")
	}
	return filepath.Join(UserDataConfigDir(), "config.yaml")
}

func KubeconfigCacheDir() string {
	return filepath.Join(YamlStorageDir(), kubeconfigDirname)
}

func KubeconfigCachePath(contextName string) string {
	return filepath.Join(KubeconfigCacheDir(), contextName+".yml")
}

func LegacyHomeConfigFilePath() string {
	return filepath.Join(os.Getenv("HOME"), legacyConfigDir, "config.yaml")
}

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
