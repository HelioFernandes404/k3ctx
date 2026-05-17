package infrastructure

import (
	"os"
	"path/filepath"

	"github.com/systemframe/k3ctx/internal/network"
	"github.com/systemframe/k3ctx/internal/tunnel"
)

// LocalStatusReader reads context and tunnel status from the local filesystem.
type LocalStatusReader struct {
	StateDir       string
	KubeconfigPath string
}

func (r LocalStatusReader) stateDir() string {
	if r.StateDir != "" {
		return r.StateDir
	}
	return filepath.Join(os.Getenv("HOME"), ".local", "state", "k3ctx-tunnels")
}

func (r LocalStatusReader) ListContextStatus() ([]map[string]any, error) {
	stateDir := r.stateDir()
	entries, _ := filepath.Glob(filepath.Join(stateDir, "*.pid"))
	var items []map[string]any
	for _, pidFile := range entries {
		base := filepath.Base(pidFile)
		contextName := base[:len(base)-4] // strip .pid
		running := tunnel.IsTunnelRunning(contextName, stateDir)
		items = append(items, map[string]any{
			"context_name":   contextName,
			"tunnel_running": running,
		})
	}
	return items, nil
}

func (r LocalStatusReader) ValidateContextNetwork(contextName string) (map[string]any, error) {
	result := network.ValidateContextNetworkDetails(contextName, r.stateDir(), nil)
	return result, nil
}
