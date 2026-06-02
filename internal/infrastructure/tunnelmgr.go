package infrastructure

import (
	"fmt"
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
	return filepath.Join(os.Getenv("HOME"), ".local", "state", "k3ctx-tunnels")
}

// KillTunnel terminates the managed tunnel process for the given context.
func (m LocalTunnelManager) KillTunnel(contextName string) error {
	tunnel.KillTunnel(contextName, m.stateDir())
	return nil
}

// ReconnectTunnel kills the existing tunnel and restarts it from persisted connection params.
func (m LocalTunnelManager) ReconnectTunnel(contextName string) (int, error) {
	stateDir := m.stateDir()
	params, err := tunnel.LoadConnParams(contextName, stateDir)
	if err != nil {
		return 0, err
	}
	tunnel.KillTunnel(contextName, stateDir)
	opts := tunnel.CreateTunnelOptions{
		Username: params.Username,
		Port:     params.SSHPort,
	}
	if params.KeyFile != "" {
		opts.KeyFilename = params.KeyFile
	}
	if params.ProxyCmd != "" {
		opts.ProxyCmd = params.ProxyCmd
	}
	pid, err := tunnel.CreateTunnel(params.SSHHost, params.InternalIP, params.LocalPort, params.RemotePort, opts)
	if err != nil {
		return 0, fmt.Errorf("reconnect tunnel: %w", err)
	}
	tunnel.SaveTunnelPID(contextName, pid, stateDir)
	return params.LocalPort, nil
}
