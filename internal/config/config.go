package config

import (
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"

	"github.com/systemframe/k3ctx/internal/domain"
	"github.com/systemframe/k3ctx/internal/paths"
)

const (
	defaultRemoteK3sConfigPath = "/etc/rancher/k3s/k3s.yaml"
	defaultSSHKeyPath          = "~/.ssh/id_ed25519"
	defaultSSHConfigPath       = "~/.ssh/config"
	defaultK3sAPIPort          = 6443
	defaultPortRangeStart      = 16443
	defaultPortRangeSize       = 10000
)

var numericKeys = []string{"k3s_api_port", "port_range_start", "port_range_size"}

var envMapping = map[string]string{
	"remote_k3s_config_path": "REMOTE_K3S_CONFIG_PATH",
	"ssh_key_path":           "SSH_KEY_PATH",
	"k3s_api_port":           "K3S_API_PORT",
	"port_range_start":       "PORT_RANGE_START",
	"port_range_size":        "PORT_RANGE_SIZE",
	"inventory_path":         "INVENTORY_PATH",
}

// LoadConfig loads config from a YAML file and overlays env vars.
// Returns empty map (no error) when file is missing or YAML is malformed.
func LoadConfig(configPath string) (map[string]any, error) {
	cfg := map[string]any{}

	data, err := os.ReadFile(configPath)
	if err == nil {
		var raw map[string]any
		if yerr := yaml.Unmarshal(data, &raw); yerr == nil && raw != nil {
			for k, v := range normalizeNumeric(raw) {
				cfg[k] = v
			}
		}
		// malformed YAML → return empty (no error surfaced)
	}

	for cfgKey, envVar := range envMapping {
		val, ok := os.LookupEnv(envVar)
		if !ok {
			continue
		}
		if isNumericKey(cfgKey) {
			if n, err := strconv.Atoi(val); err == nil {
				cfg[cfgKey] = n
			} else {
				cfg[cfgKey] = val
			}
		} else {
			cfg[cfgKey] = val
		}
	}

	return cfg, nil
}

// GetConfigValue returns cfg[key] or defaultVal when the key is absent.
func GetConfigValue(cfg map[string]any, key string, defaultVal any) any {
	if v, ok := cfg[key]; ok {
		return v
	}
	return defaultVal
}

// ResolveInventoryPath finds the best inventory directory.
func ResolveInventoryPath(cfg map[string]any, projectDir string) string {
	var configured string
	if raw, ok := cfg["inventory_path"].(string); ok && raw != "" {
		configured = expandHome(raw)
		if stat, err := os.Stat(configured); err == nil && stat.IsDir() {
			return configured
		}
	}

	local := filepath.Join(projectDir, "inventory")
	if stat, err := os.Stat(local); err == nil && stat.IsDir() {
		return local
	}

	// Walk ancestors for ansible/inventory
	dir := projectDir
	for {
		candidate := filepath.Join(dir, "ansible", "inventory")
		if stat, err := os.Stat(candidate); err == nil && stat.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	if configured != "" {
		return configured
	}
	return local
}

// LoadEffectiveConfig resolves and returns the canonical EffectiveConfig.
// If configFilePath is empty, it searches default candidate paths.
func LoadEffectiveConfig(projectDir, configFilePath string) (domain.EffectiveConfig, error) {
	resolved := configFilePath
	if resolved == "" {
		resolved = paths.ConfigFilePathXDG()
		for _, c := range paths.DefaultConfigCandidates(projectDir) {
			if _, err := os.Stat(c); err == nil {
				resolved = c
				break
			}
		}
	}

	cfg, err := LoadConfig(resolved)
	if err != nil {
		return domain.EffectiveConfig{}, err
	}

	return domain.EffectiveConfig{
		InventoryPath:       ResolveInventoryPath(cfg, projectDir),
		SSHConfigPath:       expandHome(defaultSSHConfigPath),
		SSHKeyPath:          expandHome(strVal(GetConfigValue(cfg, "ssh_key_path", defaultSSHKeyPath))),
		RemoteK3sConfigPath: strVal(GetConfigValue(cfg, "remote_k3s_config_path", defaultRemoteK3sConfigPath)),
		K3sAPIPort:          canonicalInt(cfg, "k3s_api_port", defaultK3sAPIPort),
		PortRangeStart:      canonicalInt(cfg, "port_range_start", defaultPortRangeStart),
		PortRangeSize:       canonicalInt(cfg, "port_range_size", defaultPortRangeSize),
	}, nil
}

func normalizeNumeric(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		if isNumericKey(k) {
			switch val := v.(type) {
			case int:
				out[k] = val
			case float64:
				out[k] = int(val)
			case string:
				if n, err := strconv.Atoi(val); err == nil {
					out[k] = n
				} else {
					out[k] = v
				}
			default:
				out[k] = v
			}
		} else {
			out[k] = v
		}
	}
	return out
}

func isNumericKey(k string) bool {
	for _, nk := range numericKeys {
		if nk == k {
			return true
		}
	}
	return false
}

func canonicalInt(cfg map[string]any, key string, def int) int {
	v := GetConfigValue(cfg, key, def)
	switch val := v.(type) {
	case int:
		return val
	case float64:
		return int(val)
	case string:
		if n, err := strconv.Atoi(val); err == nil {
			return n
		}
	}
	return def
}

func expandHome(p string) string {
	if len(p) >= 2 && p[:2] == "~/" {
		return filepath.Join(os.Getenv("HOME"), p[2:])
	}
	return p
}

func strVal(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
