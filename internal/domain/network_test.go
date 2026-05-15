package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/systemframe/k3ctx/internal/domain"
)

func TestIsPrivateNetwork_ClassC(t *testing.T) {
	assert.True(t, domain.IsPrivateNetwork("192.168.1.100"))
}

func TestIsPrivateNetwork_ClassA(t *testing.T) {
	assert.True(t, domain.IsPrivateNetwork("10.0.0.1"))
}

func TestIsPrivateNetwork_ClassB(t *testing.T) {
	assert.True(t, domain.IsPrivateNetwork("172.16.0.1"))
	assert.True(t, domain.IsPrivateNetwork("172.31.255.254"))
}

func TestIsPrivateNetwork_RejectsPublicIPs(t *testing.T) {
	assert.False(t, domain.IsPrivateNetwork("8.8.8.8"))
	assert.False(t, domain.IsPrivateNetwork("1.1.1.1"))
}

func TestIsPrivateNetwork_RejectsInvalidHostname(t *testing.T) {
	assert.False(t, domain.IsPrivateNetwork("example.com"))
	assert.False(t, domain.IsPrivateNetwork("invalid"))
}

func TestIsPrivateNetwork_RejectsEmptyString(t *testing.T) {
	assert.False(t, domain.IsPrivateNetwork(""))
}

func TestCheckVPNRequirement_WhenFlagTrue(t *testing.T) {
	inv := map[string]any{
		"all": map[string]any{
			"children": map[string]any{
				"k3s_cluster": map[string]any{
					"vars": map[string]any{"k3s_use_socks5_proxy": true},
				},
			},
		},
	}
	assert.True(t, domain.CheckVPNRequirement(inv, "k3s_cluster", "testhost"))
}

func TestCheckVPNRequirement_ReturnsFalseWhenFlagMissing(t *testing.T) {
	inv := map[string]any{
		"all": map[string]any{
			"children": map[string]any{
				"k3s_cluster": map[string]any{"vars": map[string]any{}},
			},
		},
	}
	assert.False(t, domain.CheckVPNRequirement(inv, "k3s_cluster", "testhost"))
}

func TestCheckVPNRequirement_ReturnsFalseWhenFlagFalse(t *testing.T) {
	inv := map[string]any{
		"all": map[string]any{
			"children": map[string]any{
				"k3s_cluster": map[string]any{
					"vars": map[string]any{"k3s_use_socks5_proxy": false},
				},
			},
		},
	}
	assert.False(t, domain.CheckVPNRequirement(inv, "k3s_cluster", "testhost"))
}

func TestCheckVPNRequirement_LegacyFlag(t *testing.T) {
	inv := map[string]any{
		"all": map[string]any{
			"children": map[string]any{
				"k3s_cluster": map[string]any{
					"vars": map[string]any{"argocd_use_socks5_proxy": true},
				},
			},
		},
	}
	assert.True(t, domain.CheckVPNRequirement(inv, "k3s_cluster", "testhost"))
}

func TestCheckVPNRequirement_InvalidInventory(t *testing.T) {
	assert.False(t, domain.CheckVPNRequirement(map[string]any{}, "group", "host"))
	assert.False(t, domain.CheckVPNRequirement(map[string]any{"all": map[string]any{}}, "group", "host"))
	assert.False(t, domain.CheckVPNRequirement(nil, "group", "host"))
}

func TestCheckNetworkRequirement_DetectsSshuttleForPrivateIP(t *testing.T) {
	hostInfo := map[string]any{
		"config": map[string]any{"ansible_host": "192.168.90.100"},
	}
	netType, netRange := domain.CheckNetworkRequirement("testhost", hostInfo)
	assert.Equal(t, "sshuttle", netType)
	assert.Contains(t, netRange, "192.168.90.0/24")
}

func TestCheckNetworkRequirement_ReturnsNilForPublicIP(t *testing.T) {
	hostInfo := map[string]any{
		"config": map[string]any{"ansible_host": "8.8.8.8"},
	}
	netType, netRange := domain.CheckNetworkRequirement("testhost", hostInfo)
	assert.Empty(t, netType)
	assert.Empty(t, netRange)
}

func TestCheckNetworkRequirement_ReturnsNilWhenAnsibleHostMissing(t *testing.T) {
	hostInfo := map[string]any{"config": map[string]any{}}
	netType, netRange := domain.CheckNetworkRequirement("testhost", hostInfo)
	assert.Empty(t, netType)
	assert.Empty(t, netRange)
}

func TestCheckNetworkRequirement_ReturnsNilForEmptyConfig(t *testing.T) {
	netType, netRange := domain.CheckNetworkRequirement("testhost", map[string]any{})
	assert.Empty(t, netType)
	assert.Empty(t, netRange)
}

func TestCheckNetworkRequirement_CalculatesCorrectNetworkRange(t *testing.T) {
	cases := []struct {
		ip       string
		expected string
	}{
		{"10.0.0.1", "10.0.0.0/24"},
		{"172.16.5.50", "172.16.5.0/24"},
		{"192.168.1.255", "192.168.1.0/24"},
	}
	for _, tc := range cases {
		hostInfo := map[string]any{"config": map[string]any{"ansible_host": tc.ip}}
		_, netRange := domain.CheckNetworkRequirement("testhost", hostInfo)
		assert.Equal(t, tc.expected, netRange, "ip=%s", tc.ip)
	}
}
