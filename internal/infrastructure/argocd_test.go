package infrastructure

import (
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/systemframe/k3ctx/internal/domain"
	"github.com/systemframe/k3ctx/internal/tunnel"
)

func argocdSSHArgs() (hostname, username string, keyfile *string, port int, proxycmd *string, internalIP string) {
	return "203.0.113.10", "helio", nil, 22, nil, "10.0.0.1"
}

func argocdEnabled(nodePort int) domain.ArgocdConfig {
	return domain.ArgocdConfig{Enabled: true, Namespace: "argocd", NodePort: &nodePort}
}

func argocdPlaintext(nodePort int) domain.ArgocdConfig {
	return domain.ArgocdConfig{Enabled: true, Namespace: "argocd", NodePort: &nodePort, Plaintext: true}
}

func servicesJSON(items string) string {
	return fmt.Sprintf(`{"items":[%s]}`, items)
}

func serviceJSON(namespace, name, labels, svcType, ports string) string {
	return fmt.Sprintf(`{
		"metadata":{"namespace":%q,"name":%q,"labels":%s},
		"spec":{"type":%q,"ports":[%s]}
	}`, namespace, name, labels, svcType, ports)
}

func nodePortJSON(name string, port, nodePort int) string {
	return fmt.Sprintf(`{"name":%q,"port":%d,"nodePort":%d}`, name, port, nodePort)
}

// --- Skipped ---

func TestLocalArgocdConnector_SkipsWhenDisabled(t *testing.T) {
	conn := NewLocalArgocdConnector()
	h, u, kf, p, pc, ip := argocdSSHArgs()
	result, err := conn.Setup("acme-prod", domain.DisabledArgocdConfig(), h, u, kf, p, pc, ip)
	require.NoError(t, err)
	assert.True(t, result.Skipped)
	assert.Nil(t, result.LocalPort)
}

func TestLocalArgocdConnector_SkipsWhenNodePortMissing(t *testing.T) {
	conn := NewLocalArgocdConnector()
	h, u, kf, p, pc, ip := argocdSSHArgs()
	cfg := domain.ArgocdConfig{Enabled: true, Namespace: "argocd", NodePort: nil}
	result, err := conn.Setup("acme-prod", cfg, h, u, kf, p, pc, ip)
	require.NoError(t, err)
	assert.True(t, result.Skipped)
	assert.Nil(t, result.LocalPort)
}

// --- Discovery ---

func TestDiscoverArgocdService_SelectsLabeledNodePortService(t *testing.T) {
	out := servicesJSON(serviceJSON(
		"argocd-system", "server", `{"app.kubernetes.io/name":"argocd-server"}`, "NodePort",
		nodePortJSON("https", 443, 30443),
	))

	result := discoverArgocdService("acme-prod", func(args []string) (string, error) {
		assert.Equal(t, []string{"kubectl", "get", "svc", "-A", "-o", "json", "--context", "acme-prod"}, args)
		return out, nil
	})

	require.NotNil(t, result)
	assert.Equal(t, "argocd-system", result.Namespace)
	assert.Equal(t, 30443, result.NodePort)
}

func TestDiscoverArgocdService_RanksCandidates(t *testing.T) {
	out := servicesJSON(fmt.Sprintf("%s,%s,%s,%s,%s",
		serviceJSON("tools", "argocd-helper", `{}`, "NodePort", nodePortJSON("https", 443, 30001)),
		serviceJSON("argocd", "web", `{}`, "NodePort", nodePortJSON("https", 443, 30002)),
		serviceJSON("other", "argocd-server", `{}`, "NodePort", nodePortJSON("https", 443, 30003)),
		serviceJSON("other", "anything", `{"app.kubernetes.io/name":"argocd-server"}`, "NodePort", nodePortJSON("https", 443, 30004)),
		serviceJSON("argocd-prod", "web", `{}`, "NodePort", nodePortJSON("https", 443, 30005)),
	))

	result := discoverArgocdService("acme-prod", func(_ []string) (string, error) { return out, nil })

	require.NotNil(t, result)
	assert.Equal(t, "other", result.Namespace)
	assert.Equal(t, 30004, result.NodePort)
}

func TestDiscoverArgocdService_RanksPorts(t *testing.T) {
	cases := []struct {
		name     string
		ports    string
		expected int
	}{
		{"https name", fmt.Sprintf("%s,%s", nodePortJSON("http", 80, 30080), nodePortJSON("https", 8443, 30443)), 30443},
		{"port 443", fmt.Sprintf("%s,%s", nodePortJSON("admin", 8080, 30080), nodePortJSON("web", 443, 30443)), 30443},
		{"http name", fmt.Sprintf("%s,%s", nodePortJSON("grpc", 8080, 30081), nodePortJSON("http", 8081, 30080)), 30080},
		{"port 80", fmt.Sprintf("%s,%s", nodePortJSON("grpc", 8080, 30081), nodePortJSON("web", 80, 30080)), 30080},
		{"first nodeport", fmt.Sprintf("%s,%s", nodePortJSON("grpc", 8080, 30081), nodePortJSON("web", 8081, 30080)), 30081},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := servicesJSON(serviceJSON("argocd", "argocd-server", `{}`, "NodePort", tc.ports))

			result := discoverArgocdService("acme-prod", func(_ []string) (string, error) { return out, nil })

			require.NotNil(t, result)
			assert.Equal(t, tc.expected, result.NodePort)
		})
	}
}

func TestDiscoverArgocdService_SkipsWhenKubectlFails(t *testing.T) {
	result := discoverArgocdService("acme-prod", func(_ []string) (string, error) {
		return "", fmt.Errorf("forbidden")
	})

	assert.Nil(t, result)
}

func TestDiscoverArgocdService_SkipsWhenNoCandidateExists(t *testing.T) {
	out := servicesJSON(serviceJSON("default", "web", `{}`, "NodePort", nodePortJSON("https", 443, 30443)))

	result := discoverArgocdService("acme-prod", func(_ []string) (string, error) { return out, nil })

	assert.Nil(t, result)
}

func TestDiscoverArgocdService_SkipsClusterIPService(t *testing.T) {
	out := servicesJSON(serviceJSON("argocd", "argocd-server", `{}`, "ClusterIP", `{"name":"https","port":443}`))

	result := discoverArgocdService("acme-prod", func(_ []string) (string, error) { return out, nil })

	assert.Nil(t, result)
}

// --- Tunnel management ---

func TestLocalArgocdConnector_OpensTunnelWhenNotRunning(t *testing.T) {
	var createCalled bool
	var saveCalled bool

	conn := NewLocalArgocdConnector()
	conn.isTunnelRunning = func(_, _ string) bool { return false }
	conn.createTunnel = func(_, _ string, _, _ int, _ tunnel.CreateTunnelOptions) (*int, error) {
		createCalled = true
		pid := 12345
		return &pid, nil
	}
	conn.saveTunnelPID = func(_ string, _ *int, _ string) { saveCalled = true }
	conn.which = func(_ string) string { return "" }

	h, u, kf, p, pc, ip := argocdSSHArgs()
	result, err := conn.Setup("acme-prod", argocdEnabled(30080), h, u, kf, p, pc, ip)

	require.NoError(t, err)
	assert.True(t, createCalled)
	assert.True(t, saveCalled)
	assert.NotNil(t, result.LocalPort)
}

func TestLocalArgocdConnector_ReusesExistingTunnel(t *testing.T) {
	var createCalled bool

	conn := NewLocalArgocdConnector()
	conn.isTunnelRunning = func(_, _ string) bool { return true }
	conn.createTunnel = func(_, _ string, _, _ int, _ tunnel.CreateTunnelOptions) (*int, error) {
		createCalled = true
		pid := 99
		return &pid, nil
	}
	conn.which = func(_ string) string { return "" }

	h, u, kf, p, pc, ip := argocdSSHArgs()
	_, err := conn.Setup("acme-prod", argocdEnabled(30080), h, u, kf, p, pc, ip)

	require.NoError(t, err)
	assert.False(t, createCalled)
}

// --- Login cases ---

func TestLocalArgocdConnector_FailsWhenArgocdCLIAbsent(t *testing.T) {
	conn := NewLocalArgocdConnector()
	conn.isTunnelRunning = func(_, _ string) bool { return true }
	conn.which = func(_ string) string { return "" }

	h, u, kf, p, pc, ip := argocdSSHArgs()
	result, err := conn.Setup("acme-prod", argocdEnabled(30080), h, u, kf, p, pc, ip)

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.False(t, result.Skipped)
	assert.NotNil(t, result.LocalPort)
	assert.Contains(t, result.Message, "argocd CLI not found")
}

func TestLocalArgocdConnector_FailsWhenPasswordUnavailable(t *testing.T) {
	conn := NewLocalArgocdConnector()
	conn.isTunnelRunning = func(_, _ string) bool { return true }
	conn.which = func(_ string) string { return "/usr/bin/argocd" }
	conn.fetchPassword = func(_, _ string) *string { return nil }

	h, u, kf, p, pc, ip := argocdSSHArgs()
	result, err := conn.Setup("acme-prod", argocdEnabled(30080), h, u, kf, p, pc, ip)

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.NotNil(t, result.LocalPort)
	assert.Contains(t, result.Message, "argocd login --insecure")
}

func TestLocalArgocdConnector_RunsArgocdLoginWithInsecureFlag(t *testing.T) {
	var capturedArgs []string

	conn := NewLocalArgocdConnector()
	conn.isTunnelRunning = func(_, _ string) bool { return true }
	conn.which = func(_ string) string { return "/usr/bin/argocd" }
	pwd := "s3cr3t"
	conn.fetchPassword = func(_, _ string) *string { return &pwd }
	conn.runArgocd = func(args []string, _ time.Duration) error {
		capturedArgs = args
		return nil
	}

	h, u, kf, p, pc, ip := argocdSSHArgs()
	result, err := conn.Setup("acme-prod", argocdEnabled(30080), h, u, kf, p, pc, ip)

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, capturedArgs, "admin")
	assert.Contains(t, capturedArgs, "s3cr3t")
	assert.Contains(t, capturedArgs, "--insecure")
	assert.NotContains(t, capturedArgs, "--plaintext")
}

func TestLocalArgocdConnector_UsesPlaintextFlagWhenConfigured(t *testing.T) {
	var capturedArgs []string

	conn := NewLocalArgocdConnector()
	conn.isTunnelRunning = func(_, _ string) bool { return true }
	conn.which = func(_ string) string { return "/usr/bin/argocd" }
	pwd := "s3cr3t"
	conn.fetchPassword = func(_, _ string) *string { return &pwd }
	conn.runArgocd = func(args []string, _ time.Duration) error {
		capturedArgs = args
		return nil
	}

	h, u, kf, p, pc, ip := argocdSSHArgs()
	_, err := conn.Setup("acme-prod", argocdPlaintext(30080), h, u, kf, p, pc, ip)

	require.NoError(t, err)
	assert.Contains(t, capturedArgs, "--plaintext")
	assert.NotContains(t, capturedArgs, "--insecure")
}

func TestLocalArgocdConnector_FailsOnLoginError(t *testing.T) {
	conn := NewLocalArgocdConnector()
	conn.isTunnelRunning = func(_, _ string) bool { return true }
	conn.which = func(_ string) string { return "/usr/bin/argocd" }
	pwd := "s3cr3t"
	conn.fetchPassword = func(_, _ string) *string { return &pwd }
	conn.runArgocd = func(_ []string, _ time.Duration) error {
		return fmt.Errorf("argocd login exited with code 1")
	}

	h, u, kf, p, pc, ip := argocdSSHArgs()
	result, err := conn.Setup("acme-prod", argocdEnabled(30080), h, u, kf, p, pc, ip)

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "argocd login failed")
}

// --- fetchArgocdPassword ---

func TestFetchArgocdPassword_ReturnsNilOnError(t *testing.T) {
	result := fetchArgocdPassword("acme-prod", "argocd", func(_ []string) (string, error) {
		return "", fmt.Errorf("kubectl not found")
	})
	assert.Nil(t, result)
}

func TestFetchArgocdPassword_ReturnsNilWhenOutputEmpty(t *testing.T) {
	result := fetchArgocdPassword("acme-prod", "argocd", func(_ []string) (string, error) {
		return "", nil
	})
	assert.Nil(t, result)
}

func TestFetchArgocdPassword_DecodesBase64(t *testing.T) {
	raw := "supersecret"
	encoded := base64.StdEncoding.EncodeToString([]byte(raw))

	result := fetchArgocdPassword("acme-prod", "argocd", func(_ []string) (string, error) {
		return encoded, nil
	})
	require.NotNil(t, result)
	assert.Equal(t, raw, *result)
}

func TestFetchArgocdPassword_ReturnsNilOnInvalidBase64(t *testing.T) {
	result := fetchArgocdPassword("acme-prod", "argocd", func(_ []string) (string, error) {
		return "not-valid-base64!!!", nil
	})
	assert.Nil(t, result)
}
