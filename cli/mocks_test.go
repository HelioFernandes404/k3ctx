package cli

import (
	"context"

	"github.com/systemframe/k3ctx/internal/domain"
)

// mockTunnelManager implements domain.TunnelManager.
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

func (m *mockCatalog) ListTargets() ([]domain.ClusterTarget, error) {
	return m.targets, m.err
}

// mockConnector implements application.ClusterConnector.
type mockConnector struct {
	artifacts domain.ConnectionArtifacts
	err       error
}

func (m *mockConnector) Connect(_ domain.ClusterTarget, _ domain.EffectiveConfig, _ domain.NetworkRequirement) (domain.ConnectionArtifacts, error) {
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

// mockReconnector implements domain.TunnelReconnector.
type mockReconnector struct {
	port int
	err  error
}

func (m *mockReconnector) ReconnectTunnel(_ string) (int, error) {
	return m.port, m.err
}

// mockClusterExec implements domain.ClusterExec.
type mockClusterExec struct {
	results map[string]domain.ExecResult
}

func (m *mockClusterExec) ExecOnContext(_ context.Context, contextName string, _ []string) domain.ExecResult {
	if r, ok := m.results[contextName]; ok {
		return r
	}
	return domain.ExecResult{Context: contextName, OK: true}
}
