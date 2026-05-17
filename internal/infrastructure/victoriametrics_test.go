package infrastructure

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/systemframe/k3ctx/internal/domain"
	"github.com/systemframe/k3ctx/internal/tunnel"
)

func victoriaMetricsSSHArgs() (hostname, username string, keyfile *string, port int, proxycmd *string, internalIP string) {
	return "203.0.113.10", "helio", nil, 22, nil, "10.0.0.1"
}

// --- victoriaMetricsServiceRank ---

func TestVictoriaMetricsServiceRank_LabelAppKubernetesIOName(t *testing.T) {
	item := serviceItem{
		Metadata: serviceMetadata{
			Name:      "vm",
			Namespace: "default",
			Labels:    map[string]string{"app.kubernetes.io/name": "victoria-metrics"},
		},
	}
	assert.Equal(t, 6, victoriaMetricsServiceRank(item))
}

func TestVictoriaMetricsServiceRank_LabelApp_Vmsingle(t *testing.T) {
	item := serviceItem{
		Metadata: serviceMetadata{
			Name:      "vmsingle-k8s",
			Namespace: "default",
			Labels:    map[string]string{"app": "vmsingle"},
		},
	}
	assert.Equal(t, 5, victoriaMetricsServiceRank(item))
}

func TestVictoriaMetricsServiceRank_NameExact(t *testing.T) {
	item := serviceItem{
		Metadata: serviceMetadata{Name: "victoria-metrics", Namespace: "ops"},
	}
	assert.Equal(t, 4, victoriaMetricsServiceRank(item))
}

func TestVictoriaMetricsServiceRank_NamePrefixVmsingle(t *testing.T) {
	item := serviceItem{
		Metadata: serviceMetadata{Name: "vmsingle-main", Namespace: "ops"},
	}
	assert.Equal(t, 4, victoriaMetricsServiceRank(item))
}

func TestVictoriaMetricsServiceRank_NamespaceVictoriametrics(t *testing.T) {
	item := serviceItem{
		Metadata: serviceMetadata{Name: "svc", Namespace: "victoriametrics"},
	}
	assert.Equal(t, 3, victoriaMetricsServiceRank(item))
}

func TestVictoriaMetricsServiceRank_NamespaceVm(t *testing.T) {
	item := serviceItem{
		Metadata: serviceMetadata{Name: "svc", Namespace: "vm"},
	}
	assert.Equal(t, 3, victoriaMetricsServiceRank(item))
}

func TestVictoriaMetricsServiceRank_NameContainsVictoria(t *testing.T) {
	item := serviceItem{
		Metadata: serviceMetadata{Name: "my-victoria-svc", Namespace: "default"},
	}
	assert.Equal(t, 2, victoriaMetricsServiceRank(item))
}

func TestVictoriaMetricsServiceRank_NamespaceContainsVictoria(t *testing.T) {
	item := serviceItem{
		Metadata: serviceMetadata{Name: "metrics", Namespace: "victoria-stack"},
	}
	assert.Equal(t, 1, victoriaMetricsServiceRank(item))
}

func TestVictoriaMetricsServiceRank_NoMatch(t *testing.T) {
	item := serviceItem{
		Metadata: serviceMetadata{Name: "prometheus", Namespace: "monitoring"},
	}
	assert.Equal(t, 0, victoriaMetricsServiceRank(item))
}

// --- bestVictoriaMetricsNodePort ---

func TestBestVictoriaMetricsNodePort_Prefers8428(t *testing.T) {
	ports := []servicePort{
		{Name: "http", Port: 8481, NodePort: 30481},
		{Name: "metrics", Port: 8428, NodePort: 30428},
	}
	p, rank, ok := bestVictoriaMetricsNodePort(ports)
	require.True(t, ok)
	assert.Equal(t, 30428, p.NodePort)
	assert.Equal(t, 5, rank)
}

func TestBestVictoriaMetricsNodePort_Prefers8481Over_Http(t *testing.T) {
	ports := []servicePort{
		{Name: "http", Port: 9999, NodePort: 30999},
		{Name: "vmselect", Port: 8481, NodePort: 30481},
	}
	p, rank, ok := bestVictoriaMetricsNodePort(ports)
	require.True(t, ok)
	assert.Equal(t, 30481, p.NodePort)
	assert.Equal(t, 3, rank)
}

func TestBestVictoriaMetricsNodePort_PrefersHttpNameOverGeneric(t *testing.T) {
	ports := []servicePort{
		{Name: "other", Port: 7777, NodePort: 30777},
		{Name: "http", Port: 9999, NodePort: 30999},
	}
	p, rank, ok := bestVictoriaMetricsNodePort(ports)
	require.True(t, ok)
	assert.Equal(t, 30999, p.NodePort)
	assert.Equal(t, 2, rank)
}

func TestBestVictoriaMetricsNodePort_ReturnsAnyNodePort(t *testing.T) {
	ports := []servicePort{
		{Name: "custom", Port: 1234, NodePort: 31234},
	}
	p, rank, ok := bestVictoriaMetricsNodePort(ports)
	require.True(t, ok)
	assert.Equal(t, 31234, p.NodePort)
	assert.Equal(t, 1, rank)
}

func TestBestVictoriaMetricsNodePort_SkipsClusterIPPorts(t *testing.T) {
	ports := []servicePort{
		{Name: "http", Port: 8428, NodePort: 0},
	}
	_, _, ok := bestVictoriaMetricsNodePort(ports)
	assert.False(t, ok)
}

func TestBestVictoriaMetricsNodePort_EmptyPorts(t *testing.T) {
	_, _, ok := bestVictoriaMetricsNodePort(nil)
	assert.False(t, ok)
}

// --- Service discovery ---

func TestDiscoverVictoriaMetricsService_SelectsLabeledService(t *testing.T) {
	out := servicesJSON(serviceJSON(
		"monitoring", "vm", `{"app.kubernetes.io/name":"victoria-metrics"}`, "NodePort",
		nodePortJSON("http", 8428, 30428),
	))

	result := discoverVictoriaMetricsService("acme-prod", func(args []string) (string, error) {
		assert.Equal(t, []string{"kubectl", "get", "svc", "-A", "-o", "json", "--context", "acme-prod"}, args)
		return out, nil
	})

	require.NotNil(t, result)
	assert.Equal(t, "monitoring", result.Namespace)
	assert.Equal(t, 30428, result.NodePort)
}

func TestDiscoverVictoriaMetricsService_SelectsByExactName(t *testing.T) {
	out := servicesJSON(serviceJSON(
		"ops", "victoria-metrics", `{}`, "NodePort",
		nodePortJSON("http", 8428, 30428),
	))

	result := discoverVictoriaMetricsService("acme-prod", func(_ []string) (string, error) { return out, nil })

	require.NotNil(t, result)
	assert.Equal(t, "ops", result.Namespace)
	assert.Equal(t, 30428, result.NodePort)
}

func TestDiscoverVictoriaMetricsService_PrefersHigherRankedService(t *testing.T) {
	out := servicesJSON(fmt.Sprintf("%s,%s",
		serviceJSON("monitoring", "my-victoria-svc", `{}`, "NodePort", nodePortJSON("http", 8428, 30001)),
		serviceJSON("monitoring", "victoria-metrics", `{}`, "NodePort", nodePortJSON("http", 8428, 30002)),
	))

	result := discoverVictoriaMetricsService("acme-prod", func(_ []string) (string, error) { return out, nil })

	require.NotNil(t, result)
	assert.Equal(t, 30002, result.NodePort)
}

func TestDiscoverVictoriaMetricsService_SkipsClusterIPOnly(t *testing.T) {
	out := servicesJSON(serviceJSON(
		"monitoring", "victoria-metrics", `{}`, "ClusterIP",
		`{"name":"http","port":8428}`,
	))

	result := discoverVictoriaMetricsService("acme-prod", func(_ []string) (string, error) { return out, nil })

	assert.Nil(t, result)
}

func TestDiscoverVictoriaMetricsService_ReturnsNilOnKubectlFailure(t *testing.T) {
	result := discoverVictoriaMetricsService("acme-prod", func(_ []string) (string, error) {
		return "", fmt.Errorf("forbidden")
	})

	assert.Nil(t, result)
}

func TestDiscoverVictoriaMetricsService_ReturnsNilWhenNoCandidateExists(t *testing.T) {
	out := servicesJSON(serviceJSON("default", "prometheus", `{}`, "NodePort", nodePortJSON("http", 9090, 30090)))

	result := discoverVictoriaMetricsService("acme-prod", func(_ []string) (string, error) { return out, nil })

	assert.Nil(t, result)
}

// --- Setup ---

func TestLocalVictoriaMetricsConnector_SkipsWhenDisabled(t *testing.T) {
	conn := NewLocalVictoriaMetricsConnector()
	h, u, kf, p, pc, ip := victoriaMetricsSSHArgs()
	result, err := conn.Setup("acme-prod", domain.DisabledVictoriaMetricsConfig(), h, u, kf, p, pc, ip)
	require.NoError(t, err)
	assert.True(t, result.Skipped)
	assert.Nil(t, result.LocalPort)
}

func TestLocalVictoriaMetricsConnector_SkipsWhenNotDiscovered(t *testing.T) {
	conn := NewLocalVictoriaMetricsConnector()
	conn.discoverService = func(_ string) *discoveredVictoriaMetricsService { return nil }
	h, u, kf, p, pc, ip := victoriaMetricsSSHArgs()
	result, err := conn.Setup("acme-prod", domain.AutoDiscoverVictoriaMetricsConfig(), h, u, kf, p, pc, ip)
	require.NoError(t, err)
	assert.True(t, result.Skipped)
	assert.Nil(t, result.LocalPort)
}

func TestLocalVictoriaMetricsConnector_OpensTunnelWhenNotRunning(t *testing.T) {
	var createCalled bool
	var saveCalled bool

	conn := NewLocalVictoriaMetricsConnector()
	conn.isTunnelRunning = func(_, _ string) bool { return false }
	conn.createTunnel = func(_, _ string, _, _ int, _ tunnel.CreateTunnelOptions) (*int, error) {
		createCalled = true
		pid := 55555
		return &pid, nil
	}
	conn.saveTunnelPID = func(_ string, _ *int, _ string) { saveCalled = true }
	conn.discoverService = func(_ string) *discoveredVictoriaMetricsService {
		port := 30428
		return &discoveredVictoriaMetricsService{Namespace: "monitoring", NodePort: port}
	}

	h, u, kf, p, pc, ip := victoriaMetricsSSHArgs()
	result, err := conn.Setup("acme-prod", domain.AutoDiscoverVictoriaMetricsConfig(), h, u, kf, p, pc, ip)

	require.NoError(t, err)
	assert.True(t, createCalled)
	assert.True(t, saveCalled)
	assert.NotNil(t, result.LocalPort)
}

func TestLocalVictoriaMetricsConnector_ReusesExistingTunnel(t *testing.T) {
	var createCalled bool

	conn := NewLocalVictoriaMetricsConnector()
	conn.isTunnelRunning = func(_, _ string) bool { return true }
	conn.createTunnel = func(_, _ string, _, _ int, _ tunnel.CreateTunnelOptions) (*int, error) {
		createCalled = true
		pid := 99
		return &pid, nil
	}
	conn.discoverService = func(_ string) *discoveredVictoriaMetricsService {
		port := 30428
		return &discoveredVictoriaMetricsService{Namespace: "monitoring", NodePort: port}
	}

	h, u, kf, p, pc, ip := victoriaMetricsSSHArgs()
	result, err := conn.Setup("acme-prod", domain.AutoDiscoverVictoriaMetricsConfig(), h, u, kf, p, pc, ip)

	require.NoError(t, err)
	assert.False(t, createCalled)
	assert.NotNil(t, result.LocalPort)
}

func TestLocalVictoriaMetricsConnector_TunnelErrorReturnsMessageNotError(t *testing.T) {
	conn := NewLocalVictoriaMetricsConnector()
	conn.isTunnelRunning = func(_, _ string) bool { return false }
	conn.createTunnel = func(_, _ string, _, _ int, _ tunnel.CreateTunnelOptions) (*int, error) {
		return nil, fmt.Errorf("ssh: connection refused")
	}
	conn.saveTunnelPID = func(_ string, _ *int, _ string) {}
	conn.discoverService = func(_ string) *discoveredVictoriaMetricsService {
		port := 30428
		return &discoveredVictoriaMetricsService{Namespace: "monitoring", NodePort: port}
	}

	h, u, kf, p, pc, ip := victoriaMetricsSSHArgs()
	result, err := conn.Setup("acme-prod", domain.AutoDiscoverVictoriaMetricsConfig(), h, u, kf, p, pc, ip)

	require.NoError(t, err)
	assert.Nil(t, result.LocalPort)
	assert.Contains(t, result.Message, "victoriametrics tunnel failed")
}
