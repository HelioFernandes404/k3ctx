package infrastructure

import (
	"os"
	"path/filepath"

	"github.com/systemframe/k3ctx/internal/tunnel"
)

// LocalTunnelManager manages SSH tunnel PID files in the default state dir.
type LocalTunnelManager struct {
	StateDir string
}

func (m LocalTunnelManager) stateDir() string {
	if m.StateDir != "" {
		return m.StateDir
	}
	return filepath.Join(os.Getenv("HOME"), ".local", "state", "k9s-tunnels")
}

func (m LocalTunnelManager) KillTunnel(contextName string) error {
	tunnel.KillTunnel(contextName, m.stateDir())
	return nil
}
