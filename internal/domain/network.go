package domain

import (
	"fmt"
	"net"
)

const (
	vpnFlag       = "k3s_use_socks5_proxy"
	vpnFlagLegacy = "argocd_use_socks5_proxy"
	hostAddr      = "addr"
)

// IsPrivateNetwork returns true if ip is an RFC 1918 private address.
func IsPrivateNetwork(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	private := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	}
	for _, cidr := range private {
		_, network, _ := net.ParseCIDR(cidr)
		if network.Contains(parsed) {
			return true
		}
	}
	return false
}

// CheckVPNRequirement returns true when the inventory group declares a VPN/socks5 proxy requirement.
func CheckVPNRequirement(invData map[string]any, groupName, _ string) bool {
	vars := extractGroupVars(invData, groupName)
	return toBool(vars[vpnFlag]) || toBool(vars[vpnFlagLegacy])
}

// CheckNetworkRequirement returns (networkType, networkRange) for a host_info map.
// host_info must have a "config" key containing "addr".
func CheckNetworkRequirement(_ string, hostInfo map[string]any) (string, string) {
	config, _ := hostInfo["config"].(map[string]any)
	if config == nil {
		return "", ""
	}
	ah, _ := config[hostAddr].(string)
	if ah == "" {
		return "", ""
	}
	return networkRangeForHost(ah)
}

// DetectNetworkRequirement derives network type, range, and VPN flag from host config + group vars.
func DetectNetworkRequirement(hostConfig, groupVars map[string]any, _ string) (string, string, bool) {
	hostInfo := map[string]any{"config": hostConfig}
	netType, netRange := CheckNetworkRequirement("", hostInfo)

	needsVPN := toBool(hostConfig[vpnFlag]) || toBool(hostConfig[vpnFlagLegacy])
	if vars, ok := hostConfig["vars"].(map[string]any); ok {
		needsVPN = needsVPN || toBool(vars[vpnFlag]) || toBool(vars[vpnFlagLegacy])
	}
	if groupVars != nil {
		needsVPN = needsVPN || toBool(groupVars[vpnFlag]) || toBool(groupVars[vpnFlagLegacy])
	}

	return netType, netRange, needsVPN
}

func networkRangeForHost(ip string) (string, string) {
	if !IsPrivateNetwork(ip) {
		return "", ""
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return "network", ""
	}
	_, network, err := net.ParseCIDR(fmt.Sprintf("%s/24", parsed.String()))
	if err != nil {
		return "network", ""
	}
	return "sshuttle", network.String()
}

func extractGroupVars(invData map[string]any, groupName string) map[string]any {
	if invData == nil {
		return nil
	}
	all, _ := invData["all"].(map[string]any)
	children, _ := all["children"].(map[string]any)
	group, _ := children[groupName].(map[string]any)
	vars, _ := group["vars"].(map[string]any)
	return vars
}
