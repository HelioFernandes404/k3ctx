## Why

k3ctx auto-discovers ArgoCD and Alertmanager during `connect`, but VictoriaMetrics — which is commonly deployed alongside these tools — requires manual port-forwarding. Additionally, ArgoCD's local port is wired internally but never printed to the user, leaving it effectively invisible at runtime.

## What Changes

- Add VictoriaMetrics auto-discovery to `connect`: detect the VictoriaMetrics NodePort service across any namespace and open an SSH tunnel to it automatically.
- Expose the VictoriaMetrics local port in CLI output and JSON result.
- Fix ArgoCD CLI output: print the ArgoCD local URL after connect (currently the port is discovered and tunnelled but never shown to the user).

## Capabilities

### New Capabilities

- `victoriametrics-auto-discovery`: Automatically discover the VictoriaMetrics service (vmsingle, victoria-metrics, or operator-managed variants) in a connected cluster, open an SSH tunnel, and surface the local port in CLI output.

### Modified Capabilities

- `argocd-auto-discovery`: ArgoCD local port must now be printed in CLI output after a successful connect (requirement change: currently silent at the CLI layer).
- `cluster-connection`: `ConnectResult`, `ConnectResultParams`, and `ConnectionArtifacts` gain a `VictoriaMetricsLocalPort` field; `ToPublicDict` includes it.

## Impact

- **New files**: `internal/domain/victoriametrics.go`, `internal/infrastructure/victoriametrics.go`, `internal/infrastructure/victoriametrics_test.go`
- **Modified files**: `internal/application/ports.go`, `internal/domain/models.go`, `internal/infrastructure/cluster.go`, `internal/bootstrap/bootstrap.go`, `cli/connect.go`
- **No breaking changes**: new fields are optional pointers; existing behavior is unchanged when VictoriaMetrics is not present.
