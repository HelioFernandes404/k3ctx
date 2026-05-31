package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systemframe/k3ctx/internal/domain"
)

func TestEffectiveConfig_ExposesExpectedFields(t *testing.T) {
	cfg := domain.EffectiveConfig{
		SSHConfigPath:       "~/.ssh/config",
		SSHKeyPath:          "~/.ssh/id_ed25519",
		RemoteK3sConfigPath: "/etc/rancher/k3s/k3s.yaml",
		K3sAPIPort:          6443,
		PortRangeStart:      16443,
		PortRangeSize:       10000,
	}

	assert.Equal(t, "~/.ssh/config", cfg.SSHConfigPath)
	assert.Equal(t, "~/.ssh/id_ed25519", cfg.SSHKeyPath)
	assert.Equal(t, "/etc/rancher/k3s/k3s.yaml", cfg.RemoteK3sConfigPath)
	assert.Equal(t, 6443, cfg.K3sAPIPort)
	assert.Equal(t, 16443, cfg.PortRangeStart)
	assert.Equal(t, 10000, cfg.PortRangeSize)
}

func TestNetworkRequirement_NoneFactoryReturnsEmptyRequirement(t *testing.T) {
	req := domain.NoNetworkRequirement()

	assert.Nil(t, req.Type)
	assert.Nil(t, req.NetworkRange)
	assert.False(t, req.NeedsVPN)
}

func TestClusterTarget_ExposesContextName(t *testing.T) {
	target := domain.NewClusterTarget("acme", "prod", "k3s_cluster", nil, nil)

	assert.Equal(t, "acme-prod", target.ContextName())
}

func TestClusterTarget_HostConfigIsProtectedFromMutation(t *testing.T) {
	hostConfig := map[string]any{
		"addr":    "10.0.0.10",
		"network": map[string]any{"dns": "10.96.0.10"},
	}
	groupVars := map[string]any{
		"proxy": map[string]any{"enabled": true},
	}
	target := domain.NewClusterTarget("acme", "prod", "k3s_cluster", hostConfig, groupVars)

	// mutate originals
	hostConfig["addr"] = "10.0.0.20"
	hostConfig["network"].(map[string]any)["dns"] = "10.96.0.20"
	groupVars["proxy"].(map[string]any)["enabled"] = false

	got := target.HostConfig()
	assert.Equal(t, "10.0.0.10", got["addr"])
	assert.Equal(t, "10.96.0.10", got["network"].(map[string]any)["dns"])
	assert.Equal(t, true, target.GroupVars()["proxy"].(map[string]any)["enabled"])
}

func TestOperationError_ToPublicDictRedactsDetail(t *testing.T) {
	err := domain.OperationError{
		Code:      "ssh_connect_failed",
		Message:   "SSH connection failed",
		Hint:      "Check your SSH key",
		Detail:    "token=secret",
		Retryable: true,
	}

	pub := err.ToPublicDict()

	assert.Equal(t, "ssh_connect_failed", pub["code"])
	assert.Equal(t, "SSH connection failed", pub["message"])
	assert.Equal(t, "Check your SSH key", pub["hint"])
	assert.Equal(t, true, pub["retryable"])
	_, hasDetail := pub["detail"]
	assert.False(t, hasDetail)
}

func TestConnectResult_RequiresErrorToMatchSuccessState(t *testing.T) {
	opErr := domain.OperationError{Code: "unexpected", Message: "unexpected"}
	req := domain.NoNetworkRequirement()

	_, err := domain.NewConnectResult(domain.ConnectResultParams{
		Success:            true,
		ContextName:        "acme-prod",
		LocalPort:          intPtr(16443),
		InternalIP:         strPtr("10.0.0.10"),
		TunnelPID:          intPtr(4242),
		UsedCache:          false,
		NetworkRequirement: req,
		Error:              &opErr,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "success=true requires error=nil")

	_, err = domain.NewConnectResult(domain.ConnectResultParams{
		Success:            false,
		ContextName:        "acme-prod",
		NetworkRequirement: req,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "success=false requires error to be set")
}

func TestConnectResult_ToPublicDictIncludesSafeSummary(t *testing.T) {
	req := domain.NoNetworkRequirement()
	result, err := domain.NewConnectResult(domain.ConnectResultParams{
		Success:            true,
		ContextName:        "acme-prod",
		LocalPort:          intPtr(16443),
		InternalIP:         strPtr("10.0.0.10"),
		TunnelPID:          intPtr(4242),
		UsedCache:          false,
		NetworkRequirement: req,
	})
	require.NoError(t, err)

	pub := result.ToPublicDict()

	assert.Equal(t, true, pub["success"])
	assert.Equal(t, "acme-prod", pub["context_name"])
	assert.Equal(t, 16443, pub["local_port"])
	assert.Equal(t, "10.0.0.10", pub["internal_ip"])
	assert.Equal(t, 4242, pub["tunnel_pid"])
	assert.Equal(t, false, pub["used_cache"])
	assert.Nil(t, pub["error"])
	netReq := pub["network_requirement"].(map[string]any)
	assert.Nil(t, netReq["type"])
	assert.Nil(t, netReq["network_range"])
	assert.Equal(t, false, netReq["needs_vpn"])
}

func TestConnectResult_ToPublicDictIncludesErrorPayload(t *testing.T) {
	opErr := domain.OperationError{
		Code:      "ssh_connect_failed",
		Message:   "SSH connection failed",
		Hint:      "Verify SSH access",
		Detail:    "token=secret",
		Retryable: true,
	}
	req := domain.NoNetworkRequirement()
	result, err := domain.NewConnectResult(domain.ConnectResultParams{
		Success:            false,
		ContextName:        "acme-prod",
		NetworkRequirement: req,
		Error:              &opErr,
	})
	require.NoError(t, err)

	pub := result.ToPublicDict()

	assert.Equal(t, false, pub["success"])
	errPub := pub["error"].(map[string]any)
	assert.Equal(t, "ssh_connect_failed", errPub["code"])
	assert.Equal(t, "SSH connection failed", errPub["message"])
	assert.Equal(t, "Verify SSH access", errPub["hint"])
	assert.Equal(t, true, errPub["retryable"])
}

func intPtr(v int) *int       { return &v }
func strPtr(v string) *string { return &v }
