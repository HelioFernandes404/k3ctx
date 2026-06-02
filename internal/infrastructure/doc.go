// Package infrastructure contains the adapters that implement the ports
// declared in domain: cluster connector, command exec, status reader,
// tunnel manager, observability connectors (ArgoCD, Alertmanager,
// VictoriaMetrics), and inventory catalog. Adapters here own all
// IO concerns and never leak technical details into the use cases.
package infrastructure
