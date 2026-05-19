package cli

import (
	"github.com/systemframe/k3ctx/internal/domain"
)

// mockTunnelManager implements application.TunnelManager.
type mockTunnelManager struct {
	killFn func(contextName string) error
}

func (m *mockTunnelManager) KillTunnel(contextName string) error {
	if m.killFn != nil {
		return m.killFn(contextName)
	}
	return nil
}

// mockCatalog implements application.InventoryCatalog.
type mockCatalog struct {
	targets []domain.ClusterTarget
	err     error
}

func (m *mockCatalog) ListTargets(_ string) ([]domain.ClusterTarget, error) {
	return m.targets, m.err
}
