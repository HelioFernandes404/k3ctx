## 1. All-Hosts Connect — Use Case

- [x] 1.1 Add `FindTargetsByClient(client, inventoryPath string, catalog application.InventoryCatalog) ([]domain.ClusterTarget, error)` to `internal/application/usecases/inventory.go` — calls `ListClusterTargets` and filters by `target.Company() == client`
- [x] 1.2 Unit test `FindTargetsByClient`: no-match returns empty slice, client filter returns only matching targets

## 2. All-Hosts Connect — CLI

- [x] 2.1 Add `connectAllHosts string` flag variable and `--all-hosts` flag to `cli/connect.go`
- [x] 2.2 Add mutual-exclusion guard in `runConnect`: if `--all-hosts` is set alongside any other host-filter flags or positional args, return a usage error
- [x] 2.3 Add `runConnectAllHosts` function in `cli/connect.go`: calls `FindTargetsByClient`, returns `NO_MATCH` when empty, calls `usecases.ConnectMultiple`, outputs JSON array or per-line text
- [x] 2.4 Dispatch to `runConnectAllHosts` at the top of `runConnect` when `connectAllHosts != ""`
- [x] 2.5 Unit tests for `runConnectAllHosts`: no-match exits with `NO_MATCH`, multi-result JSON array contains all entries including failures

## 3. Cross-Cluster Exec — Port and Adapter

- [x] 3.1 Add `ExecResult` struct to `internal/application/ports.go` (fields: `Context`, `OK`, `Stdout`, `Stderr`, `ExitCode`)
- [x] 3.2 Add `ClusterExec` interface to `internal/application/ports.go` with `ExecOnContext(ctx context.Context, contextName string, args []string) ExecResult`
- [x] 3.3 Implement `LocalClusterExec` in `internal/infrastructure/exec.go` using `exec.CommandContext`; set `--context <name>` as first kubectl args; capture stdout/stderr; record exit code
- [x] 3.4 Add `Exec application.ClusterExec` to `ServiceContainer` in `internal/bootstrap/bootstrap.go`; wire `infrastructure.LocalClusterExec{}` in `Build`

## 4. Cross-Cluster Exec — Use Case and CLI

- [x] 4.1 Add `ExecOnContexts(contextNames []string, args []string, timeout time.Duration, executer application.ClusterExec) []application.ExecResult` use case in `internal/application/usecases/exec.go`; run in parallel goroutines with `context.WithTimeout`
- [x] 4.2 Add `cli/exec.go` with `exec` Cobra command: reads `StatusReader.ListContextStatus()`, filters for `liveness == "live"`, calls `ExecOnContexts`, outputs JSON array or text with `=== <context> ===` headers
- [x] 4.3 Add `--timeout` flag (default 30) to `exec` command; no-live-tunnels case prints `No live tunnels.` (text) or empty array (JSON)
- [x] 4.4 Require at least one argument after `--`; exit with usage error when no kubectl args provided

## 5. Tests

- [x] 5.1 Unit test `ExecOnContexts` use case: empty context list returns empty slice; stub executer records calls; all results returned regardless of per-context failure
- [x] 5.2 Unit test `LocalClusterExec.ExecOnContext` using `echo` as the command (verifies stdout capture and `ok: true`)
