## 1. Domain Layer

- [x] 1.1 Create `internal/domain/victoriametrics.go` with `VictoriaMetricsConfig` struct (`Enabled`, `Discovery`, `Namespace`, `NodePort *int`) and `DisabledVictoriaMetricsConfig()` / `AutoDiscoverVictoriaMetricsConfig()` constructors
- [x] 1.2 Add `victoriaMetricsLocalPort *int` field to `ConnectResultParams` and `ConnectResult` in `internal/domain/models.go`
- [x] 1.3 Add `VictoriaMetricsLocalPort() *int` accessor to `ConnectResult`
- [x] 1.4 Add `victoriametrics_local_port` key to `ConnectResult.ToPublicDict()`

## 2. Application Ports

- [x] 2.1 Add `VictoriaMetricsResult` struct to `internal/application/ports.go` (fields: `LocalPort *int`, `Skipped bool`, `Message string`)
- [x] 2.2 Add `VictoriaMetricsConnector` interface to `internal/application/ports.go` with `Setup(contextName string, cfg domain.VictoriaMetricsConfig, hostname, username string, keyfile *string, port int, proxycmd *string, internalIP string) (VictoriaMetricsResult, error)`
- [x] 2.3 Add `VictoriaMetricsLocalPort *int` field to `ConnectionArtifacts` in `internal/application/ports.go`

## 3. Infrastructure — VictoriaMetrics Connector

- [x] 3.1 Create `internal/infrastructure/victoriametrics.go` with `LocalVictoriaMetricsConnector` struct (injectable fields: `isTunnelRunning`, `createTunnel`, `saveTunnelPID`, `discoverService`; port range constants `48000`/`10000`)
- [x] 3.2 Implement `NewLocalVictoriaMetricsConnector()` wiring real tunnel and kubectl dependencies
- [x] 3.3 Implement `Setup()` on `LocalVictoriaMetricsConnector` matching `VictoriaMetricsConnector` interface: check enabled, run discovery, skip gracefully if not found, open or reuse tunnel
- [x] 3.4 Implement `discoverVictoriaMetricsService(contextName string, kubectlRun func) *discoveredVictoriaMetricsService` using `kubectl get svc -A -o json`
- [x] 3.5 Implement `victoriaMetricsServiceRank(item serviceItem) int` with the 6-level heuristic (label `app.kubernetes.io/name`, label `app`, name prefix, namespace exact, name contains, namespace contains)
- [x] 3.6 Implement `bestVictoriaMetricsNodePort(ports []servicePort) (servicePort, int, bool)` preferring port 8428, then 8481, then name `http`, then any NodePort
- [x] 3.7 Create `internal/infrastructure/victoriametrics_test.go` with unit tests for `victoriaMetricsServiceRank` and `bestVictoriaMetricsNodePort` covering all rank levels and edge cases (no NodePort, tie-breaking, multiple candidates)

## 4. Wire Infrastructure

- [x] 4.1 Add `VictoriaMetricsAdapter application.VictoriaMetricsConnector` field to `LocalClusterConnector` in `internal/infrastructure/cluster.go`
- [x] 4.2 Update `NewLocalClusterConnector()` signature to accept a `VictoriaMetricsConnector` parameter and assign it
- [x] 4.3 Call `c.VictoriaMetricsAdapter.Setup(...)` in `Connect()` after ArgoCD and Alertmanager setup; store result port in `ConnectionArtifacts.VictoriaMetricsLocalPort`
- [x] 4.4 Update `internal/bootstrap/bootstrap.go` to instantiate `NewLocalVictoriaMetricsConnector()` and pass it to `NewLocalClusterConnector()`

## 5. CLI Output

- [x] 5.1 In `cli/connect.go`, after the `Connected:` line, print `ArgoCD: http://127.0.0.1:<port>` when `result.ArgocdLocalPort()` is non-nil
- [x] 5.2 In `cli/connect.go`, print `VictoriaMetrics: http://127.0.0.1:<port>` when `result.VictoriaMetricsLocalPort()` is non-nil

## 6. Tests and Validation

- [x] 6.1 Run `go test ./...` and confirm all tests pass
- [x] 6.2 Run `go build ./...` to confirm the build is clean
- [x] 6.3 SSH to `helio@76.13.67.14` (or via sshuttle if needed) and confirm VictoriaMetrics service exists: `kubectl get svc -A | grep -i victoria`
- [x] 6.4 Run `k3ctx connect` against the test cluster and verify output shows VictoriaMetrics and ArgoCD URLs
- [x] 6.5 Run `k3ctx connect --json` and verify `victoriametrics_local_port` and `argocd_local_port` are present in JSON output
