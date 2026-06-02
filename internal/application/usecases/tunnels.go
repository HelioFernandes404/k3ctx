package usecases

import "github.com/systemframe/k3ctx/internal/domain"

// KillTunnel delegates tunnel termination to the TunnelManager.
func KillTunnel(contextName string, manager domain.TunnelManager) error {
	return manager.KillTunnel(contextName)
}

// KillAllTunnels kills every running tunnel and returns the names of killed contexts.
func KillAllTunnels(statusReader domain.StatusReader, manager domain.TunnelManager) ([]string, error) {
	items, err := statusReader.ListContextStatus()
	if err != nil {
		return nil, err
	}
	var killed []string
	for _, item := range items {
		if running, _ := item["tunnel_running"].(bool); !running {
			continue
		}
		name, _ := item["context_name"].(string)
		if name == "" {
			continue
		}
		_ = manager.KillTunnel(name)
		killed = append(killed, name)
	}
	return killed, nil
}

// ReconnectTunnel re-establishes a broken managed tunnel and returns the local port.
func ReconnectTunnel(contextName string, reconnector domain.TunnelReconnector) (int, error) {
	return reconnector.ReconnectTunnel(contextName)
}
