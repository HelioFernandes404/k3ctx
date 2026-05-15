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
