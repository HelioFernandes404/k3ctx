## Context

k3ctx already auto-discovers ArgoCD and Alertmanager during `connect` using a consistent pattern: a domain config type, an infrastructure connector with kubectl-based service discovery, an application port interface, and wiring through `cluster.go` and `bootstrap.go`. VictoriaMetrics is a common addition to the same monitoring stack but has no equivalent support.

Additionally, ArgoCD's local port is computed and tunnelled correctly but never printed to the user in human-readable output — a gap that makes it invisible in practice.

Current state:
- `cluster.go` calls `ArgocdAdapter.Setup()` and `AlertmanagerAdapter.Setup()` and returns both ports in `ConnectionArtifacts`.
- `connect.go` prints Alertmanager port but not ArgoCD port.
- No VictoriaMetrics connector exists anywhere.

## Goals / Non-Goals

**Goals:**
- Add VictoriaMetrics auto-discovery following the identical pattern to Alertmanager.
- Print ArgoCD local URL after a successful connect (fix silent output).
- Print VictoriaMetrics local URL after a successful connect.
- All three services remain best-effort: failure does not fail the cluster connection.

**Non-Goals:**
- VictoriaMetrics authentication (it exposes an unauthenticated HTTP endpoint by default).
- VictoriaLogs discovery (separate service, separate change).
- Inventory-configured (manual) NodePort for VictoriaMetrics — auto-discovery only for now.
- Changing how ArgoCD discovery or tunnelling works.

## Decisions

### 1. Mirror Alertmanager exactly — no new abstractions

The Alertmanager connector is the cleanest reference. VictoriaMetrics will follow it field-for-field:

| Alertmanager | VictoriaMetrics |
|---|---|
| `AlertmanagerConfig` | `VictoriaMetricsConfig` |
| `LocalAlertmanagerConnector` | `LocalVictoriaMetricsConnector` |
| `AlertmanagerResult` | `VictoriaMetricsResult` |
| `AlertmanagerConnector` interface | `VictoriaMetricsConnector` interface |
| `alertmanagerLocalPort *int` | `victoriaMetricsLocalPort *int` |
| port range 38000–47999 | port range 48000–57999 |

Alternative considered: a generic `ServiceTunnelConnector` parameterised by ranking function. Rejected — premature abstraction, three services don't justify it, and the concrete types are easier to test independently.

### 2. Service fingerprinting by ranked heuristics

VictoriaMetrics is deployed in several forms (single-node binary, Helm chart, VM Operator). No single label or name covers all cases, so a ranked scoring approach (matching Alertmanager's pattern) is used:

| Signal | Rank |
|---|---|
| `app.kubernetes.io/name == "victoria-metrics"` | 6 |
| `app == "vmsingle"` | 5 |
| `name == "victoria-metrics"` | 4 |
| `name` has prefix `"vmsingle"` | 4 |
| namespace is `"victoriametrics"` or `"vm"` | 3 |
| `name` contains `"victoria"` | 2 |
| namespace contains `"victoria"` | 1 |

Port ranking:

| Signal | Rank |
|---|---|
| port 8428 (VictoriaMetrics HTTP default) | 5 |
| port 8481 (vmselect query endpoint) | 3 |
| port name `"http"` | 2 |
| any other NodePort | 1 |

Alternative considered: only matching port 8428. Rejected — operator-managed deployments sometimes remap ports; name/label matching is more reliable as primary signal.

### 3. ArgoCD CLI output fix is in-scope

The fix is a one-line addition to `connect.go`. Including it here avoids a separate trivial change and makes the three-service output consistent from day one.

### 4. Port range 48000–57999

ArgoCD: 28000–37999. Alertmanager: 38000–47999. VictoriaMetrics: 48000–57999. Consistent 10k-slot windows, no overlap.

## Risks / Trade-offs

- **False positive on port 8428**: Any NodePort service with port 8428 will score rank 5 for the port even if the service isn't VictoriaMetrics. The service-name and label ranking above it mitigates this — port rank only breaks ties between services with the same name score.
- **vmselect vs vmsingle ambiguity**: In a VM Operator cluster deployment both may exist. `vmsingle` prefix rank (4) and label rank (5/6) will prefer the correct single-node or operator-managed write endpoint over vmselect (which is a query router). Acceptable for the common case.
- **Tunnel churn on reconnect**: Same behaviour as ArgoCD and Alertmanager — existing tunnel is reused if the PID file is valid.

## Migration Plan

No migration required. All new fields are optional pointers; JSON output gains new nullable keys. Existing consumers of `ToPublicDict` are unaffected.

Deployment: ship as a normal release. No feature flag needed.
