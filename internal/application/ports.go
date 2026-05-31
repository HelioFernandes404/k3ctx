package application

import (
	"context"

	"github.com/systemframe/k3ctx/internal/domain"
)

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
	Connect(target domain.ClusterTarget, config domain.EffectiveConfig, req domain.NetworkRequirement) (ConnectionArtifacts, error)
}

// InventoryCatalog lists cluster targets from an inventory path.
type InventoryCatalog interface {
	ListTargets(inventoryPath string) ([]domain.ClusterTarget, error)
}

// InventoryRefresher pulls the latest inventory from its source.
type InventoryRefresher interface {
	Refresh(inventoryPath string) (bool, string)
}

// ContextSwitcher sets the active kubectl context.
type ContextSwitcher interface {
	SwitchContext(contextName string) *domain.OperationError
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
	// ReconnectTunnel kills any existing tunnel for contextName, opens a new one
	// using stored ConnParams, and returns the local port on success.
	ReconnectTunnel(contextName string) (int, error)
}

// ArgocdConnector sets up the ArgoCD tunnel and login.
type ArgocdConnector interface {
	Setup(contextName string, cfg domain.ArgocdConfig, hostname, username string, keyfile *string, port int, proxycmd *string, internalIP string) (ArgocdLoginResult, error)
}

// AlertmanagerConnector sets up the Alertmanager tunnel.
type AlertmanagerConnector interface {
	Setup(contextName string, cfg domain.AlertmanagerConfig, hostname, username string, keyfile *string, port int, proxycmd *string, internalIP string) (AlertmanagerResult, error)
}

// VictoriaMetricsConnector sets up the VictoriaMetrics tunnel.
type VictoriaMetricsConnector interface {
	Setup(contextName string, cfg domain.VictoriaMetricsConfig, hostname, username string, keyfile *string, port int, proxycmd *string, internalIP string) (VictoriaMetricsResult, error)
}

// NetBirdPreflightChecker validates NetBird daemon and peer readiness before SSH.
type NetBirdPreflightChecker interface {
	// CheckDaemonReady verifies the local NetBird daemon is Connected.
	// Returns nil when binary is absent (non-NetBird env) or skipCheck is true.
	CheckDaemonReady(skipCheck bool) error
	// CheckPeerReady verifies that the target FQDN peer is connected.
	// Must be called after CheckDaemonReady; returns nil when skipped.
	CheckPeerReady(fqdn string, skipCheck bool) error
}
