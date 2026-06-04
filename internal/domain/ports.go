package domain

import "context"

// ExecResult holds the outcome of running a kubectl command against one context.
type ExecResult struct {
	Context  string
	OK       bool
	Stdout   string
	Stderr   string
	ExitCode int
}

// ClusterExec runs an arbitrary kubectl command against a named context.
type ClusterExec interface {
	ExecOnContext(ctx context.Context, contextName string, args []string) ExecResult
}

// ConnectionArtifacts holds the outputs of a successful cluster connection.
type ConnectionArtifacts struct {
	LocalPort                int
	InternalIP               string
	TunnelPID                *int
	UsedCache                bool
	AlreadyConnected         bool // true when tunnel was already live and kubeconfig existed; full connect was skipped
	ArgocdLocalPort          *int
	ArgocdLoginSuccess       bool
	ArgocdLoginMessage       string
	AlertmanagerLocalPort    *int
	VictoriaMetricsLocalPort *int
}

// ArgocdLoginResult describes the outcome of an ArgoCD login attempt.
type ArgocdLoginResult struct {
	Success   bool
	LocalPort *int
	Skipped   bool
	Message   string
}

// AlertmanagerResult describes the outcome of an Alertmanager tunnel attempt.
type AlertmanagerResult struct {
	LocalPort *int
	Skipped   bool
	Message   string
}

// VictoriaMetricsResult describes the outcome of a VictoriaMetrics tunnel attempt.
type VictoriaMetricsResult struct {
	LocalPort *int
	Skipped   bool
	Message   string
}

// ClusterConnector opens a tunnel and prepares a cluster context.
type ClusterConnector interface {
	Connect(target ClusterTarget, config EffectiveConfig, req NetworkRequirement) (ConnectionArtifacts, error)
}

// InventoryCatalog lists cluster targets from the configured source.
type InventoryCatalog interface {
	ListTargets() ([]ClusterTarget, error)
}

// ContextSwitcher sets the active kubectl context.
type ContextSwitcher interface {
	SwitchContext(contextName string) *OperationError
}

// StatusReader provides runtime cluster and tunnel state.
type StatusReader interface {
	ListContextStatus() ([]map[string]any, error)
	ValidateContextNetwork(contextName string) (map[string]any, error)
}

// TunnelManager manages tunnel lifecycle.
type TunnelManager interface {
	KillTunnel(contextName string) error
}

// TunnelReconnector re-establishes a broken managed tunnel using persisted SSH parameters.
type TunnelReconnector interface {
	ReconnectTunnel(contextName string) (int, error)
}

// ArgocdConnector sets up the ArgoCD tunnel and login.
type ArgocdConnector interface {
	Setup(contextName string, cfg ArgocdConfig, hostname, username string, keyfile *string, port int, proxycmd *string, internalIP string) (ArgocdLoginResult, error)
}

// AlertmanagerConnector sets up the Alertmanager tunnel.
type AlertmanagerConnector interface {
	Setup(contextName string, cfg AlertmanagerConfig, hostname, username string, keyfile *string, port int, proxycmd *string, internalIP string) (AlertmanagerResult, error)
}

// VictoriaMetricsConnector sets up the VictoriaMetrics tunnel.
type VictoriaMetricsConnector interface {
	Setup(contextName string, cfg VictoriaMetricsConfig, hostname, username string, keyfile *string, port int, proxycmd *string, internalIP string) (VictoriaMetricsResult, error)
}

// NetBirdPreflightChecker validates NetBird daemon and peer readiness before SSH.
type NetBirdPreflightChecker interface {
	CheckDaemonReady(skipCheck bool) error
	CheckPeerReady(fqdn string, skipCheck bool) error
}
