package cli

import (
	"github.com/systemframe/k3ctx/internal/application"
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

// mockConnector implements application.ClusterConnector.
type mockConnector struct {
	artifacts application.ConnectionArtifacts
	err       error
}

func (m *mockConnector) Connect(_ domain.ClusterTarget, _ domain.EffectiveConfig, _ domain.NetworkRequirement) (application.ConnectionArtifacts, error) {
	return m.artifacts, m.err
}

// mockStatusReader implements application.StatusReader.
type mockStatusReader struct {
	items []map[string]any
}

func (m *mockStatusReader) ListContextStatus() ([]map[string]any, error) {
	return m.items, nil
}

func (m *mockStatusReader) ValidateContextNetwork(_ string) (map[string]any, error) {
	return map[string]any{}, nil
}
