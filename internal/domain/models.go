package domain

import "fmt"

// EffectiveConfig holds resolved configuration for a session.
type EffectiveConfig struct {
	InventoryPath       string // optional; empty activates NetBird catalog
	SSHConfigPath       string
	SSHKeyPath          string
	RemoteK3sConfigPath string
	K3sAPIPort          int
	PortRangeStart      int
	PortRangeSize       int
	NetBirdBinPath      string // path to netbird binary; default "netbird"
	NetBirdHostFilter   string // regex applied to hostname label; empty = no filter
}

// NetworkRequirement describes the network setup needed to reach a cluster.
type NetworkRequirement struct {
	Type         *string
	NetworkRange *string
	NeedsVPN     bool
}

// NoNetworkRequirement returns a zero NetworkRequirement (no VPN, no sshuttle).
func NoNetworkRequirement() NetworkRequirement {
	return NetworkRequirement{}
}

// ClusterTarget is the resolved identity of a single K3s host.
type ClusterTarget struct {
	company    string
	hostAlias  string
	group      string
	hostConfig map[string]any
	groupVars  map[string]any
}

// NewClusterTarget creates a ClusterTarget with deep-copied config maps.
func NewClusterTarget(company, hostAlias, group string, hostConfig, groupVars map[string]any) ClusterTarget {
	return ClusterTarget{
		company:    company,
		hostAlias:  hostAlias,
		group:      group,
		hostConfig: deepCopyMap(hostConfig),
		groupVars:  deepCopyMap(groupVars),
	}
}

func (t ClusterTarget) Company() string   { return t.company }
func (t ClusterTarget) HostAlias() string { return t.hostAlias }
func (t ClusterTarget) Group() string     { return t.group }

// ContextName returns the canonical kubectl context name.
func (t ClusterTarget) ContextName() string { return t.company + "-" + t.hostAlias }

// HostConfig returns a shallow copy of the host config (values are already deep-copied at construction).
func (t ClusterTarget) HostConfig() map[string]any { return shallowCopy(t.hostConfig) }

// GroupVars returns a shallow copy of the group vars.
func (t ClusterTarget) GroupVars() map[string]any { return shallowCopy(t.groupVars) }

// OperationError is a structured error with a safe public representation.
type OperationError struct {
	Code      string
	Message   string
	Hint      string
	Detail    string
	Retryable bool
}

func (e OperationError) Error() string { return fmt.Sprintf("[%s] %s", e.Code, e.Message) }

// ToPublicDict returns the error without the Detail field.
func (e OperationError) ToPublicDict() map[string]any {
	return map[string]any{
		"code":      e.Code,
		"message":   e.Message,
		"hint":      e.Hint,
		"retryable": e.Retryable,
	}
}

// ConnectResult holds the outcome of a single cluster connection attempt.
type ConnectResult struct {
	success                  bool
	contextName              string
	localPort                *int
	internalIP               *string
	tunnelPID                *int
	usedCache                bool
	networkRequirement       NetworkRequirement
	err                      *OperationError
	argocdLocalPort          *int
	argocdLoginSuccess       bool
	argocdLoginMessage       string
	alertmanagerLocalPort    *int
	victoriaMetricsLocalPort *int
}

// ConnectResultParams groups the inputs for NewConnectResult.
type ConnectResultParams struct {
	Success                  bool
	ContextName              string
	LocalPort                *int
	InternalIP               *string
	TunnelPID                *int
	UsedCache                bool
	NetworkRequirement       NetworkRequirement
	Error                    *OperationError
	ArgocdLocalPort          *int
	ArgocdLoginSuccess       bool
	ArgocdLoginMessage       string
	AlertmanagerLocalPort    *int
	VictoriaMetricsLocalPort *int
}

// NewConnectResult validates and constructs a ConnectResult.
func NewConnectResult(p ConnectResultParams) (ConnectResult, error) {
	if p.Success && p.Error != nil {
		return ConnectResult{}, fmt.Errorf("success=true requires error=nil")
	}
	if !p.Success && p.Error == nil {
		return ConnectResult{}, fmt.Errorf("success=false requires error to be set")
	}
	return ConnectResult{
		success:                  p.Success,
		contextName:              p.ContextName,
		localPort:                p.LocalPort,
		internalIP:               p.InternalIP,
		tunnelPID:                p.TunnelPID,
		usedCache:                p.UsedCache,
		networkRequirement:       p.NetworkRequirement,
		err:                      p.Error,
		argocdLocalPort:          p.ArgocdLocalPort,
		argocdLoginSuccess:       p.ArgocdLoginSuccess,
		argocdLoginMessage:       p.ArgocdLoginMessage,
		alertmanagerLocalPort:    p.AlertmanagerLocalPort,
		victoriaMetricsLocalPort: p.VictoriaMetricsLocalPort,
	}, nil
}

func (r ConnectResult) Success() bool                          { return r.success }
func (r ConnectResult) ContextName() string                    { return r.contextName }
func (r ConnectResult) LocalPort() *int                        { return r.localPort }
func (r ConnectResult) InternalIP() *string                    { return r.internalIP }
func (r ConnectResult) TunnelPID() *int                        { return r.tunnelPID }
func (r ConnectResult) UsedCache() bool                        { return r.usedCache }
func (r ConnectResult) NetworkRequirement() NetworkRequirement { return r.networkRequirement }
func (r ConnectResult) Err() *OperationError                   { return r.err }
func (r ConnectResult) ArgocdLocalPort() *int                  { return r.argocdLocalPort }
func (r ConnectResult) ArgocdLoginSuccess() bool               { return r.argocdLoginSuccess }
func (r ConnectResult) ArgocdLoginMessage() string             { return r.argocdLoginMessage }
func (r ConnectResult) AlertmanagerLocalPort() *int            { return r.alertmanagerLocalPort }
func (r ConnectResult) VictoriaMetricsLocalPort() *int         { return r.victoriaMetricsLocalPort }

// ToPublicDict returns a JSON-safe representation.
func (r ConnectResult) ToPublicDict() map[string]any {
	var errDict any
	if r.err != nil {
		errDict = r.err.ToPublicDict()
	}

	var localPort, tunnelPID, argocdLocalPort, alertmanagerLocalPort, victoriaMetricsLocalPort any
	if r.localPort != nil {
		localPort = *r.localPort
	}
	if r.tunnelPID != nil {
		tunnelPID = *r.tunnelPID
	}
	if r.argocdLocalPort != nil {
		argocdLocalPort = *r.argocdLocalPort
	}
	if r.alertmanagerLocalPort != nil {
		alertmanagerLocalPort = *r.alertmanagerLocalPort
	}
	if r.victoriaMetricsLocalPort != nil {
		victoriaMetricsLocalPort = *r.victoriaMetricsLocalPort
	}

	var internalIP any
	if r.internalIP != nil {
		internalIP = *r.internalIP
	}

	return map[string]any{
		"success":      r.success,
		"context_name": r.contextName,
		"local_port":   localPort,
		"internal_ip":  internalIP,
		"tunnel_pid":   tunnelPID,
		"used_cache":   r.usedCache,
		"network_requirement": map[string]any{
			"type":          r.networkRequirement.Type,
			"network_range": r.networkRequirement.NetworkRange,
			"needs_vpn":     r.networkRequirement.NeedsVPN,
		},
		"error":                      errDict,
		"argocd_local_port":          argocdLocalPort,
		"alertmanager_local_port":    alertmanagerLocalPort,
		"victoriametrics_local_port": victoriaMetricsLocalPort,
	}
}

// deepCopyMap creates a recursive deep copy of a map[string]any.
func deepCopyMap(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = deepCopyValue(v)
	}
	return dst
}

func deepCopyValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		return deepCopyMap(val)
	case []any:
		cp := make([]any, len(val))
		for i, item := range val {
			cp[i] = deepCopyValue(item)
		}
		return cp
	default:
		return v
	}
}

func shallowCopy(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
