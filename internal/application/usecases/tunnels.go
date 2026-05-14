package usecases

import "github.com/systemframe/k3ctx/internal/application"

// KillTunnel delegates tunnel termination to the TunnelManager.
func KillTunnel(contextName string, manager application.TunnelManager) error {
	return manager.KillTunnel(contextName)
}
