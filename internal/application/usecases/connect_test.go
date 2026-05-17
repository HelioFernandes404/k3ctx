package usecases_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/systemframe/k3ctx/internal/application"
	"github.com/systemframe/k3ctx/internal/application/usecases"
	"github.com/systemframe/k3ctx/internal/domain"
)

// --- Stub connector ---

type stubConnector struct {
	artifacts application.ConnectionArtifacts
	err       error
	calls     []domain.ClusterTarget
}

func (s *stubConnector) Connect(target domain.ClusterTarget, _ domain.EffectiveConfig, _ domain.NetworkRequirement) (application.ConnectionArtifacts, error) {
	s.calls = append(s.calls, target)
	if s.err != nil {
		return application.ConnectionArtifacts{}, s.err
	}
	return s.artifacts, nil
}

func buildConfig(t *testing.T) domain.EffectiveConfig {
	t.Helper()
	return domain.EffectiveConfig{
		InventoryPath:       t.TempDir() + "/inventory",
		SSHConfigPath:       "~/.ssh/config",
		SSHKeyPath:          "~/.ssh/id_ed25519",
		RemoteK3sConfigPath: "/etc/rancher/k3s/k3s.yaml",
		K3sAPIPort:          6443,
		PortRangeStart:      16443,
		PortRangeSize:       10000,
	}
}

func buildTarget(ansibleHost string) domain.ClusterTarget {
	return domain.NewClusterTarget("acme", "prod", "k3s_cluster",
		map[string]any{"ansible_host": ansibleHost}, nil)
}

// --- Tests ---

func TestConnectCluster_DelegatesToConnectorAndReturnsStructuredResult(t *testing.T) {
	pid := 4242
	stub := &stubConnector{artifacts: application.ConnectionArtifacts{
		LocalPort:  16443,
		InternalIP: "10.0.0.10",
		TunnelPID:  &pid,
		UsedCache:  false,
	}}
	cfg := buildConfig(t)
	target := buildTarget("203.0.113.10")

	result, err := usecases.ConnectCluster(target, cfg, stub, true)

	require.NoError(t, err)
	assert.True(t, result.Success())
	assert.Equal(t, "acme-prod", result.ContextName())
	assert.Equal(t, 16443, *result.LocalPort())
	assert.Equal(t, "10.0.0.10", *result.InternalIP())
	assert.Equal(t, 4242, *result.TunnelPID())
	assert.Nil(t, result.Err())
	require.Len(t, stub.calls, 1)
	assert.Equal(t, target.ContextName(), stub.calls[0].ContextName())
}

func TestConnectCluster_BlocksWhenManualNetworkSetupRequired(t *testing.T) {
	stub := &stubConnector{artifacts: application.ConnectionArtifacts{LocalPort: 16443, InternalIP: "10.0.0.10"}}
	cfg := buildConfig(t)
	target := buildTarget("10.0.0.10") // private IP → sshuttle requirement

	result, err := usecases.ConnectCluster(target, cfg, stub, false)

	require.NoError(t, err)
	assert.False(t, result.Success())
	require.NotNil(t, result.Err())
	assert.Equal(t, "network_requirement_unmet", result.Err().Code)
	assert.Empty(t, stub.calls)
}

func TestConnectMultiple_PreservesTargetOrder(t *testing.T) {
	stub := &stubConnector{artifacts: application.ConnectionArtifacts{LocalPort: 16443, InternalIP: "10.0.0.10"}}
	cfg := buildConfig(t)
	first := buildTarget("203.0.113.10")
	second := domain.NewClusterTarget("beta", "staging", "k3s_cluster",
		map[string]any{"ansible_host": "203.0.113.11"}, nil)

	results, err := usecases.ConnectMultiple([]domain.ClusterTarget{first, second}, cfg, stub, true)

	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "acme-prod", results[0].ContextName())
	assert.Equal(t, "beta-staging", results[1].ContextName())
}

func TestConnectCluster_ReturnsSpecificPublicErrorForAPIReadinessFailure(t *testing.T) {
	stub := &stubConnector{err: &domain.OperationError{
		Code:    "kubernetes_api_unreachable",
		Message: "Kubernetes API did not become ready on https://127.0.0.1:16443",
	}}
	cfg := buildConfig(t)
	target := buildTarget("203.0.113.10")

	result, err := usecases.ConnectCluster(target, cfg, stub, true)

	require.NoError(t, err)
	assert.False(t, result.Success())
	require.NotNil(t, result.Err())
	assert.Equal(t, "kubernetes_api_unreachable", result.Err().Code)
	assert.Contains(t, result.Err().Message, "did not become ready")
}
