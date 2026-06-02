package tunnel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ConnParams holds the SSH parameters needed to recreate a managed tunnel.
type ConnParams struct {
	SSHHost    string `json:"ssh_host"`
	InternalIP string `json:"internal_ip"`
	LocalPort  int    `json:"local_port"`
	RemotePort int    `json:"remote_port"`
	Username   string `json:"username"`
	KeyFile    string `json:"key_file"`
	SSHPort    int    `json:"ssh_port"`
	ProxyCmd   string `json:"proxy_cmd"`
}

func connParamsPath(contextName, stateDir string) string {
	_ = os.MkdirAll(stateDir, 0o700)
	return filepath.Join(stateDir, contextName+".conn.json")
}

// SaveConnParams persists SSH connection parameters for later reconnection.
func SaveConnParams(contextName, stateDir string, p ConnParams) error {
	data, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal conn params: %w", err)
	}
	return os.WriteFile(connParamsPath(contextName, stateDir), data, 0o600)
}

// LoadConnParams reads persisted SSH connection parameters.
// Returns an error with a user-facing message when the file is absent.
func LoadConnParams(contextName, stateDir string) (ConnParams, error) {
	data, err := os.ReadFile(connParamsPath(contextName, stateDir))
	if err != nil {
		if os.IsNotExist(err) {
			return ConnParams{}, fmt.Errorf("no connection state for context %s; run k3ctx connect first", contextName)
		}
		return ConnParams{}, fmt.Errorf("read conn params: %w", err)
	}
	var p ConnParams
	if err := json.Unmarshal(data, &p); err != nil {
		return ConnParams{}, fmt.Errorf("parse conn params: %w", err)
	}
	return p, nil
}
