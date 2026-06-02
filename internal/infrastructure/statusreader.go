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
	PortRangeStart int
	PortRangeSize  int
}

func (r LocalStatusReader) stateDir() string {
	if r.StateDir != "" {
		return r.StateDir
	}
	return filepath.Join(os.Getenv("HOME"), ".local", "state", "k3ctx-tunnels")
}

func (r LocalStatusReader) portRangeStart() int {
	if r.PortRangeStart != 0 {
		return r.PortRangeStart
	}
	return 16443
}

func (r LocalStatusReader) portRangeSize() int {
	if r.PortRangeSize != 0 {
		return r.PortRangeSize
	}
	return 10000
}

// ListContextStatus returns one status record per managed tunnel found in the state directory.
func (r LocalStatusReader) ListContextStatus() ([]map[string]any, error) {
	stateDir := r.stateDir()
	entries, _ := filepath.Glob(filepath.Join(stateDir, "*.pid"))
	items := make([]map[string]any, 0, len(entries))
	for _, pidFile := range entries {
		base := filepath.Base(pidFile)
		contextName := base[:len(base)-4] // strip .pid
		localPort := tunnel.GetUniquePort(contextName, r.portRangeStart(), r.portRangeSize())
		liveness := tunnel.Liveness(contextName, stateDir, localPort)
		items = append(items, map[string]any{
			"context_name":   contextName,
			"tunnel_running": liveness != "dead",
			"liveness":       liveness,
		})
	}
	return items, nil
}

// ValidateContextNetwork inspects the persisted network metadata for a context.
func (r LocalStatusReader) ValidateContextNetwork(contextName string) (map[string]any, error) {
	result := network.ValidateContextNetworkDetails(contextName, r.stateDir(), nil)
	return result, nil
}
