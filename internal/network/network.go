package network

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const corruptedWarning = "Network metadata file exists but could not be read safely"

// GetNetworkMetadata loads the .network metadata file for a context.
// Returns nil when the file does not exist, empty map for empty files,
// and {"corrupted": true, ...} for unreadable or malformed files.
func GetNetworkMetadata(contextName, stateDir string) map[string]any {
	data, corrupted, exists := loadRawMetadata(contextName, stateDir)
	if !exists {
		return nil
	}
	if corrupted {
		return data
	}
	return data
}

// ValidateContextNetwork checks whether required network setup is active.
// Pass a non-nil checkSshuttle to override the default pgrep-based check (useful in tests).
func ValidateContextNetwork(contextName, stateDir string, checkSshuttle func(string) bool) (bool, string) {
	data, corrupted, exists := loadRawMetadata(contextName, stateDir)
	if !exists {
		return true, ""
	}
	if corrupted {
		return false, corruptedWarning
	}
	if len(data) == 0 {
		return true, ""
	}

	if needsVPN, _ := data["needs_vpn"].(bool); needsVPN {
		return false, "This cluster requires VPN connection"
	}

	if netType, _ := data["network_type"].(string); netType == "sshuttle" {
		netRange, _ := data["network_range"].(string)
		if netRange == "" {
			return false, corruptedWarning
		}

		checker := checkSshuttle
		if checker == nil {
			checker = CheckSshuttleActive
		}
		if !checker(netRange) {
			sshuttleCmd, _ := data["sshuttle_command"].(string)
			if sshuttleCmd == "" {
				sshuttleCmd = fmt.Sprintf("sshuttle -v -r <gateway> %s", netRange)
			}
			warning := fmt.Sprintf("This cluster requires sshuttle for %s\n  Run: %s", netRange, sshuttleCmd)
			return false, warning
		}
	}

	return true, ""
}

// ValidateContextNetworkDetails returns a structured validation result.
func ValidateContextNetworkDetails(contextName, stateDir string, checkSshuttle func(string) bool) map[string]any {
	metadata := GetNetworkMetadata(contextName, stateDir)
	ok, warning := ValidateContextNetwork(contextName, stateDir, checkSshuttle)

	var warningAny any
	if warning != "" {
		warningAny = warning
	}

	return map[string]any{
		"context_name":     contextName,
		"ok":               ok,
		"warning":          warningAny,
		"network_metadata": metadata,
	}
}

// CheckSshuttleActive uses pgrep to detect a running sshuttle process.
func CheckSshuttleActive(networkRange string) bool {
	ctx := context.Background()
	specific := exec.CommandContext(ctx, "pgrep", "-f", "sshuttle.*"+networkRange) //nolint:gosec // pgrep with networkRange from config
	if out, err := specific.Output(); err == nil && strings.TrimSpace(string(out)) != "" {
		return true
	}
	generic := exec.CommandContext(ctx, "pgrep", "-f", "sshuttle")
	if out, err := generic.Output(); err == nil && strings.TrimSpace(string(out)) != "" {
		return true
	}
	return false
}

// ValidateNetworkAccess tests TCP connectivity to ip:6443.
func ValidateNetworkAccess(ip string) bool {
	d := net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.DialContext(context.Background(), "tcp", ip+":6443")
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// loadRawMetadata returns (data, corrupted, fileExists).
func loadRawMetadata(contextName, stateDir string) (map[string]any, bool, bool) {
	path := filepath.Join(stateDir, contextName+".network")
	raw, err := os.ReadFile(path) //nolint:gosec // path built from validated stateDir
	if err != nil {
		return nil, false, false
	}

	var parsed any
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		return map[string]any{
			"corrupted": true,
			"warning":   corruptedWarning,
			"error":     err.Error(),
		}, true, true
	}

	if parsed == nil {
		return map[string]any{}, false, true
	}

	m, ok := parsed.(map[string]any)
	if !ok {
		return map[string]any{
			"corrupted": true,
			"warning":   corruptedWarning,
			"error":     "expected mapping metadata structure",
		}, true, true
	}

	return m, false, true
}
