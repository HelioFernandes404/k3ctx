package infrastructure

import (
	"crypto/tls"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/systemframe/k3ctx/internal/application"
	"github.com/systemframe/k3ctx/internal/domain"
)

func makeTarget() domain.ClusterTarget {
	return domain.NewClusterTarget("acme", "prod", "k3s_cluster",
		map[string]any{"addr": "203.0.113.10"}, nil)
}

func makeEffectiveConfig(t *testing.T) domain.EffectiveConfig {
	return domain.EffectiveConfig{
		SSHConfigPath:       "~/.ssh/config",
		SSHKeyPath:          "~/.ssh/id_ed25519",
		RemoteK3sConfigPath: "/etc/rancher/k3s/k3s.yaml",
		K3sAPIPort:          6443,
		PortRangeStart:      16443,
		PortRangeSize:       10000,
	}
}

func stubSSHConnect(target domain.ClusterTarget, config domain.EffectiveConfig) (string, string, *string, int, *string, func(string) (string, error), error) {
	runner := func(_ string) (string, error) { return "", nil }
	return "resolved.example.internal", "ec2-user", nil, 22, nil, runner, nil
}

func stubPrepareKubeconfig(_ func(string) (string, error), _ domain.ClusterTarget, _ domain.EffectiveConfig) (string, int, bool, string, error) {
	return "10.0.0.10", 16443, false, "apiVersion: v1\n", nil
}

func stubEnsureTunnel(pid int, reused bool) func(string, string, string, string, *string, int, int, int, *string, string) (*int, bool, error) {
	return func(_, _, _, _ string, _ *string, _, _, _ int, _ *string, _ string) (*int, bool, error) {
		return &pid, reused, nil
	}
}

func noopMerge(content, contextName string) (string, error) { return "/tmp/.kube/config", nil }

type stubArgocdConnector struct {
	calls []domain.ArgocdConfig
	err   error
	port  *int
}

func (s *stubArgocdConnector) Setup(_ string, cfg domain.ArgocdConfig, _, _ string, _ *string, _ int, _ *string, _ string) (application.ArgocdLoginResult, error) {
	s.calls = append(s.calls, cfg)
	return application.ArgocdLoginResult{LocalPort: s.port}, s.err
}

// --- Orchestration tests ---

func TestLocalClusterConnector_AbortsWhenAPICheckFailsOnFreshTunnel(t *testing.T) {
	var killCalled []string
	mergeCalled := false

	conn := NewLocalClusterConnector(nil, nil, nil)
	conn.StateDir = t.TempDir()
	conn.sshConnect = stubSSHConnect
	conn.prepareKubeconfig = stubPrepareKubeconfig
	conn.ensureTunnel = stubEnsureTunnel(4242, false) // fresh tunnel
	conn.pollAPIReady = func(_ int, _ string, _, _ time.Duration) error {
		return &domain.OperationError{
			Code:    "kubernetes_api_unreachable",
			Message: "Kubernetes API did not become ready on https://127.0.0.1:16443",
		}
	}
	conn.killTunnel = func(contextName, _ string) { killCalled = append(killCalled, contextName) }
	conn.mergeKubeconfig = func(content, ctx string) (string, error) {
		mergeCalled = true
		return noopMerge(content, ctx)
	}

	_, err := conn.Connect(makeTarget(), makeEffectiveConfig(t), domain.NoNetworkRequirement())

	require.Error(t, err)
	assert.Equal(t, []string{"acme-prod"}, killCalled)
	assert.False(t, mergeCalled)
}

func TestLocalClusterConnector_RecreatesStaleReusedTunnelOnAPIFailure(t *testing.T) {
	var killCalled []string
	var ensureCalls int
	var apiCalls int

	ensureSideEffects := []struct {
		pid    int
		reused bool
	}{{4242, true}, {4343, false}}

	conn := NewLocalClusterConnector(nil, nil, nil)
	conn.StateDir = t.TempDir()
	conn.sshConnect = stubSSHConnect
	conn.prepareKubeconfig = stubPrepareKubeconfig
	conn.ensureTunnel = func(_, _, _, _ string, _ *string, _, _, _ int, _ *string, _ string) (*int, bool, error) {
		se := ensureSideEffects[ensureCalls]
		ensureCalls++
		return &se.pid, se.reused, nil
	}
	conn.pollAPIReady = func(_ int, _ string, _, _ time.Duration) error {
		apiCalls++
		if apiCalls == 1 {
			return &domain.OperationError{Code: "kubernetes_api_unreachable", Message: "timeout"}
		}
		return nil
	}
	conn.killTunnel = func(ctx, _ string) { killCalled = append(killCalled, ctx) }
	conn.mergeKubeconfig = noopMerge

	result, err := conn.Connect(makeTarget(), makeEffectiveConfig(t), domain.NoNetworkRequirement())

	require.NoError(t, err)
	assert.Equal(t, 2, ensureCalls)
	assert.Equal(t, 2, apiCalls)
	assert.Equal(t, []string{"acme-prod"}, killCalled)
	assert.Equal(t, 4343, *result.TunnelPID)
}

func TestLocalClusterConnector_MergesKubeconfigOnSuccess(t *testing.T) {
	var mergedContent, mergedContext string

	conn := NewLocalClusterConnector(nil, nil, nil)
	conn.StateDir = t.TempDir()
	conn.sshConnect = stubSSHConnect
	conn.prepareKubeconfig = stubPrepareKubeconfig
	conn.ensureTunnel = stubEnsureTunnel(1234, false)
	conn.pollAPIReady = func(_ int, _ string, _, _ time.Duration) error { return nil }
	conn.killTunnel = func(_, _ string) {}
	conn.mergeKubeconfig = func(content, ctx string) (string, error) {
		mergedContent = content
		mergedContext = ctx
		return "/tmp/.kube/config", nil
	}

	_, err := conn.Connect(makeTarget(), makeEffectiveConfig(t), domain.NoNetworkRequirement())

	require.NoError(t, err)
	assert.Equal(t, "apiVersion: v1\n", mergedContent)
	assert.Equal(t, "acme-prod", mergedContext)
}

func TestLocalClusterConnector_SkipsAPICheckWhenDisabled(t *testing.T) {
	apiCalled := false
	mergeCalled := false

	conn := NewLocalClusterConnector(nil, nil, nil)
	conn.StateDir = t.TempDir()
	conn.VerifyAPIReady = false
	conn.sshConnect = stubSSHConnect
	conn.prepareKubeconfig = stubPrepareKubeconfig
	conn.ensureTunnel = stubEnsureTunnel(1234, false)
	conn.pollAPIReady = func(_ int, _ string, _, _ time.Duration) error {
		apiCalled = true
		return nil
	}
	conn.killTunnel = func(_, _ string) {}
	conn.mergeKubeconfig = func(content, ctx string) (string, error) {
		mergeCalled = true
		return noopMerge(content, ctx)
	}

	_, err := conn.Connect(makeTarget(), makeEffectiveConfig(t), domain.NoNetworkRequirement())

	require.NoError(t, err)
	assert.False(t, apiCalled)
	assert.True(t, mergeCalled)
}

func TestLocalClusterConnector_UsesAutoDiscoveryArgocdConfig(t *testing.T) {
	argocd := &stubArgocdConnector{}
	target := domain.NewClusterTarget("acme", "prod", "k3s_cluster", map[string]any{
		"addr":               "203.0.113.10",
		"argocd_enabled":     false,
		"argocd_node_port":   30080,
		"argocd_namespace":   "inventory-ns",
		"argocd_plaintext":   true,
		"argocd_extra_noise": true,
	}, nil)

	conn := NewLocalClusterConnector(argocd, nil, nil)
	conn.StateDir = t.TempDir()
	conn.sshConnect = stubSSHConnect
	conn.prepareKubeconfig = stubPrepareKubeconfig
	conn.ensureTunnel = stubEnsureTunnel(1234, false)
	conn.pollAPIReady = func(_ int, _ string, _, _ time.Duration) error { return nil }
	conn.killTunnel = func(_, _ string) {}
	conn.mergeKubeconfig = noopMerge

	_, err := conn.Connect(target, makeEffectiveConfig(t), domain.NoNetworkRequirement())

	require.NoError(t, err)
	require.Len(t, argocd.calls, 1)
	assert.True(t, argocd.calls[0].Enabled)
	assert.True(t, argocd.calls[0].Discovery)
	assert.Nil(t, argocd.calls[0].NodePort)
	assert.Equal(t, "argocd", argocd.calls[0].Namespace)
	assert.False(t, argocd.calls[0].Plaintext)
}

func TestLocalClusterConnector_IgnoresArgocdSetupError(t *testing.T) {
	argocd := &stubArgocdConnector{err: fmt.Errorf("discovery failed")}

	conn := NewLocalClusterConnector(argocd, nil, nil)
	conn.StateDir = t.TempDir()
	conn.sshConnect = stubSSHConnect
	conn.prepareKubeconfig = stubPrepareKubeconfig
	conn.ensureTunnel = stubEnsureTunnel(1234, false)
	conn.pollAPIReady = func(_ int, _ string, _, _ time.Duration) error { return nil }
	conn.killTunnel = func(_, _ string) {}
	conn.mergeKubeconfig = noopMerge

	result, err := conn.Connect(makeTarget(), makeEffectiveConfig(t), domain.NoNetworkRequirement())

	require.NoError(t, err)
	assert.Equal(t, 16443, result.LocalPort)
	require.Len(t, argocd.calls, 1)
}

// --- pollAPIReadyWithDoer ---

func TestPollAPIReadyWithDoer_PerRequestTimeoutAtLeast2s(t *testing.T) {
	var observedTimeouts []time.Duration
	doer := func(_ string, perReq time.Duration, _ *tls.Config) (int, error) {
		observedTimeouts = append(observedTimeouts, perReq)
		return 200, nil
	}

	err := pollAPIReadyWithDoer(16443, 5*time.Second, 250*time.Millisecond, nil, doer)
	require.NoError(t, err)
	require.NotEmpty(t, observedTimeouts, "doer must be called at least once")
	assert.GreaterOrEqual(t, observedTimeouts[0].Seconds(), 2.0,
		"per-request timeout should be at least 2s, got %v", observedTimeouts[0])
}

func TestPollAPIReadyWithDoer_ReturnsNilOn2xx(t *testing.T) {
	doer := func(_ string, _ time.Duration, _ *tls.Config) (int, error) { return 200, nil }
	err := pollAPIReadyWithDoer(16443, 5*time.Second, 100*time.Millisecond, nil, doer)
	require.NoError(t, err)
}

func TestPollAPIReadyWithDoer_ReturnsNilOn4xx(t *testing.T) {
	doer := func(_ string, _ time.Duration, _ *tls.Config) (int, error) { return 401, nil }
	err := pollAPIReadyWithDoer(16443, 5*time.Second, 100*time.Millisecond, nil, doer)
	require.NoError(t, err)
}

func TestPollAPIReadyWithDoer_ReturnsErrorWhenDeadlineExceeded(t *testing.T) {
	doer := func(_ string, _ time.Duration, _ *tls.Config) (int, error) {
		return 0, fmt.Errorf("connection refused")
	}
	err := pollAPIReadyWithDoer(16443, 200*time.Millisecond, 50*time.Millisecond, nil, doer)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kubernetes_api_unreachable")
}
