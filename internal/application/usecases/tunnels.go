package usecases

import "github.com/systemframe/k3ctx/internal/application"

// KillTunnel delegates tunnel termination to the TunnelManager.
func KillTunnel(contextName string, manager application.TunnelManager) error {
	return manager.KillTunnel(contextName)
}

// ReconnectTunnel re-establishes a broken managed tunnel and returns the local port.
func ReconnectTunnel(contextName string, reconnector application.TunnelReconnector) (int, error) {
	return reconnector.ReconnectTunnel(contextName)
}
