package infrastructure

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/systemframe/k3ctx/internal/domain"
	"github.com/systemframe/k3ctx/internal/tunnel"
)

func alertmanagerSSHArgs() (hostname, username string, keyfile *string, port int, proxycmd *string, internalIP string) {
	return "203.0.113.10", "helio", nil, 22, nil, "10.0.0.1"
}

// --- Service discovery ---

func TestDiscoverAlertmanagerService_SelectsLabeledNodePortService(t *testing.T) {
	out := servicesJSON(serviceJSON(
		"monitoring", "alertmanager", `{"app.kubernetes.io/name":"alertmanager"}`, "NodePort",
		nodePortJSON("http", 9093, 30093),
	))

	result := discoverAlertmanagerService("acme-prod", func(args []string) (string, error) {
		assert.Equal(t, []string{"kubectl", "get", "svc", "-A", "-o", "json", "--context", "acme-prod"}, args)
		return out, nil
	})

	require.NotNil(t, result)
	assert.Equal(t, "monitoring", result.Namespace)
	assert.Equal(t, 30093, result.NodePort)
}

func TestDiscoverAlertmanagerService_SelectsByName(t *testing.T) {
	out := servicesJSON(serviceJSON(
		"ops", "alertmanager", `{}`, "NodePort",
		nodePortJSON("http", 9093, 30093),
	))

	result := discoverAlertmanagerService("acme-prod", func(_ []string) (string, error) { return out, nil })

	require.NotNil(t, result)
	assert.Equal(t, "ops", result.Namespace)
	assert.Equal(t, 30093, result.NodePort)
}

func TestDiscoverAlertmanagerService_PrefersMonitoringNamespace(t *testing.T) {
	out := servicesJSON(fmt.Sprintf("%s,%s",
		serviceJSON("other", "alertmanager", `{}`, "NodePort", nodePortJSON("http", 9093, 30001)),
		serviceJSON("monitoring", "alertmanager", `{}`, "NodePort", nodePortJSON("http", 9093, 30002)),
	))

	result := discoverAlertmanagerService("acme-prod", func(_ []string) (string, error) { return out, nil })

	require.NotNil(t, result)
	assert.Equal(t, 30002, result.NodePort)
}

func TestDiscoverAlertmanagerService_PrefersPort9093(t *testing.T) {
	out := servicesJSON(serviceJSON(
		"monitoring", "alertmanager", `{}`, "NodePort",
		fmt.Sprintf("%s,%s",
			nodePortJSON("metrics", 8080, 30080),
			nodePortJSON("http", 9093, 30093),
		),
	))

	result := discoverAlertmanagerService("acme-prod", func(_ []string) (string, error) { return out, nil })

	require.NotNil(t, result)
	assert.Equal(t, 30093, result.NodePort)
}

func TestDiscoverAlertmanagerService_SelectsClusterIPAsFallback(t *testing.T) {
	out := servicesJSON(serviceJSON(
		"monitoring", "alertmanager", `{}`, "ClusterIP",
		`{"name":"http","port":9093}`,
	))

	result := discoverAlertmanagerService("acme-prod", func(_ []string) (string, error) { return out, nil })

	require.NotNil(t, result)
	assert.Equal(t, "monitoring", result.Namespace)
	assert.Equal(t, "alertmanager", result.ServiceName)
	assert.Equal(t, 9093, result.NodePort)
	assert.True(t, result.UseKubectl)
}

func TestDiscoverAlertmanagerService_PrefersNodePortOverClusterIP(t *testing.T) {
	out := servicesJSON(fmt.Sprintf("%s,%s",
		serviceJSON("monitoring", "alertmanager", `{}`, "ClusterIP", `{"name":"http","port":9093}`),
		serviceJSON("monitoring", "alertmanager", `{}`, "NodePort", nodePortJSON("http", 9093, 30093)),
	))

	result := discoverAlertmanagerService("acme-prod", func(_ []string) (string, error) { return out, nil })

	require.NotNil(t, result)
	assert.Equal(t, 30093, result.NodePort)
	assert.False(t, result.UseKubectl)
}

func TestDiscoverAlertmanagerService_SkipsClusterIPWithNoMatchingPort(t *testing.T) {
	out := servicesJSON(serviceJSON(
		"monitoring", "alertmanager", `{}`, "ClusterIP",
		`{"name":"metrics","port":8080}`,
	))

	result := discoverAlertmanagerService("acme-prod", func(_ []string) (string, error) { return out, nil })

	assert.Nil(t, result)
}

func TestDiscoverAlertmanagerService_ReturnsNilOnKubectlFailure(t *testing.T) {
	result := discoverAlertmanagerService("acme-prod", func(_ []string) (string, error) {
		return "", fmt.Errorf("forbidden")
	})

	assert.Nil(t, result)
}

func TestDiscoverAlertmanagerService_ReturnsNilWhenNoCandidateExists(t *testing.T) {
	out := servicesJSON(serviceJSON("default", "prometheus", `{}`, "NodePort", nodePortJSON("http", 9090, 30090)))

	result := discoverAlertmanagerService("acme-prod", func(_ []string) (string, error) { return out, nil })

	assert.Nil(t, result)
}

// --- Setup ---

func TestLocalAlertmanagerConnector_SkipsWhenDisabled(t *testing.T) {
	conn := NewLocalAlertmanagerConnector()
	h, u, kf, p, pc, ip := alertmanagerSSHArgs()
	result, err := conn.Setup("acme-prod", domain.DisabledAlertmanagerConfig(), h, u, kf, p, pc, ip)
	require.NoError(t, err)
	assert.True(t, result.Skipped)
	assert.Nil(t, result.LocalPort)
}

func TestLocalAlertmanagerConnector_SkipsWhenNotDiscovered(t *testing.T) {
	conn := NewLocalAlertmanagerConnector()
	conn.discoverService = func(_ string) *discoveredAlertmanagerService { return nil }
	h, u, kf, p, pc, ip := alertmanagerSSHArgs()
	result, err := conn.Setup("acme-prod", domain.AutoDiscoverAlertmanagerConfig(), h, u, kf, p, pc, ip)
	require.NoError(t, err)
	assert.True(t, result.Skipped)
	assert.Nil(t, result.LocalPort)
}

func TestLocalAlertmanagerConnector_OpensTunnelWhenNotRunning(t *testing.T) {
	var createCalled bool
	var saveCalled bool

	conn := NewLocalAlertmanagerConnector()
	conn.isTunnelRunning = func(_, _ string) bool { return false }
	conn.createTunnel = func(_, _ string, _, _ int, _ tunnel.CreateTunnelOptions) (*int, error) {
		createCalled = true
		pid := 55555
		return &pid, nil
	}
	conn.saveTunnelPID = func(_ string, _ *int, _ string) { saveCalled = true }
	conn.discoverService = func(_ string) *discoveredAlertmanagerService {
		port := 30093
		return &discoveredAlertmanagerService{Namespace: "monitoring", NodePort: port}
	}

	h, u, kf, p, pc, ip := alertmanagerSSHArgs()
	result, err := conn.Setup("acme-prod", domain.AutoDiscoverAlertmanagerConfig(), h, u, kf, p, pc, ip)

	require.NoError(t, err)
	assert.True(t, createCalled)
	assert.True(t, saveCalled)
	assert.NotNil(t, result.LocalPort)
}

func TestLocalAlertmanagerConnector_ReusesExistingTunnel(t *testing.T) {
	var createCalled bool

	conn := NewLocalAlertmanagerConnector()
	conn.isTunnelRunning = func(_, _ string) bool { return true }
	conn.createTunnel = func(_, _ string, _, _ int, _ tunnel.CreateTunnelOptions) (*int, error) {
		createCalled = true
		pid := 99
		return &pid, nil
	}
	conn.discoverService = func(_ string) *discoveredAlertmanagerService {
		port := 30093
		return &discoveredAlertmanagerService{Namespace: "monitoring", NodePort: port}
	}

	h, u, kf, p, pc, ip := alertmanagerSSHArgs()
	result, err := conn.Setup("acme-prod", domain.AutoDiscoverAlertmanagerConfig(), h, u, kf, p, pc, ip)

	require.NoError(t, err)
	assert.False(t, createCalled)
	assert.NotNil(t, result.LocalPort)
}

func TestLocalAlertmanagerConnector_OpensKubectlPortForwardForClusterIP(t *testing.T) {
	var kubectlCalled bool
	var tunnelCalled bool

	conn := NewLocalAlertmanagerConnector()
	conn.isTunnelRunning = func(_, _ string) bool { return false }
	conn.createKubectlPortForward = func(ctx, ns, svc string, _ int, remote int) (*int, error) {
		kubectlCalled = true
		assert.Equal(t, "acme-prod", ctx)
		assert.Equal(t, "monitoring", ns)
		assert.Equal(t, "alertmanager", svc)
		assert.Equal(t, 9093, remote)
		pid := 77777
		return &pid, nil
	}
	conn.createTunnel = func(_, _ string, _, _ int, _ tunnel.CreateTunnelOptions) (*int, error) {
		tunnelCalled = true
		return nil, nil
	}
	conn.saveTunnelPID = func(_ string, _ *int, _ string) {}
	conn.discoverService = func(_ string) *discoveredAlertmanagerService {
		port := 9093
		return &discoveredAlertmanagerService{
			Namespace:   "monitoring",
			ServiceName: "alertmanager",
			NodePort:    port,
			UseKubectl:  true,
		}
	}

	h, u, kf, p, pc, ip := alertmanagerSSHArgs()
	result, err := conn.Setup("acme-prod", domain.AutoDiscoverAlertmanagerConfig(), h, u, kf, p, pc, ip)

	require.NoError(t, err)
	assert.True(t, kubectlCalled, "createKubectlPortForward should be called for ClusterIP")
	assert.False(t, tunnelCalled, "createTunnel should NOT be called for ClusterIP")
	assert.NotNil(t, result.LocalPort)
}

func TestLocalAlertmanagerConnector_KubectlErrorReturnsMessageNotError(t *testing.T) {
	conn := NewLocalAlertmanagerConnector()
	conn.isTunnelRunning = func(_, _ string) bool { return false }
	conn.createKubectlPortForward = func(_, _, _ string, _, _ int) (*int, error) {
		return nil, fmt.Errorf("kubectl: connection refused")
	}
	conn.saveTunnelPID = func(_ string, _ *int, _ string) {}
	conn.discoverService = func(_ string) *discoveredAlertmanagerService {
		port := 9093
		return &discoveredAlertmanagerService{
			Namespace:   "monitoring",
			ServiceName: "alertmanager",
			NodePort:    port,
			UseKubectl:  true,
		}
	}

	h, u, kf, p, pc, ip := alertmanagerSSHArgs()
	result, err := conn.Setup("acme-prod", domain.AutoDiscoverAlertmanagerConfig(), h, u, kf, p, pc, ip)

	require.NoError(t, err)
	assert.Nil(t, result.LocalPort)
	assert.Contains(t, result.Message, "alertmanager tunnel failed")
}

func TestLocalAlertmanagerConnector_TunnelErrorReturnsMessageNotError(t *testing.T) {
	conn := NewLocalAlertmanagerConnector()
	conn.isTunnelRunning = func(_, _ string) bool { return false }
	conn.createTunnel = func(_, _ string, _, _ int, _ tunnel.CreateTunnelOptions) (*int, error) {
		return nil, fmt.Errorf("ssh: connection refused")
	}
	conn.saveTunnelPID = func(_ string, _ *int, _ string) {}
	conn.discoverService = func(_ string) *discoveredAlertmanagerService {
		port := 30093
		return &discoveredAlertmanagerService{Namespace: "monitoring", NodePort: port}
	}

	h, u, kf, p, pc, ip := alertmanagerSSHArgs()
	result, err := conn.Setup("acme-prod", domain.AutoDiscoverAlertmanagerConfig(), h, u, kf, p, pc, ip)

	require.NoError(t, err)
	assert.Nil(t, result.LocalPort)
	assert.Contains(t, result.Message, "alertmanager tunnel failed")
}
