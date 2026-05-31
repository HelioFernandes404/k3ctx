## Why

`k3ctx connect` resolves to exactly one host, requiring the caller to loop over `k3ctx hosts <client>` output to connect to all clusters in a client. Since the CLI is consumed by Claude Code (an AI agent), this loop belongs in the CLI itself to avoid multi-step orchestration and partial-failure bookkeeping by the agent. Cross-cluster kubectl exec has the same problem: the agent would have to shell-loop and collate output manually.

## What Changes

- `k3ctx connect` gains `--all-hosts <client>` flag that connects to every host in a client in sequence and returns a JSON array of per-context results.
- A new `k3ctx exec -- <kubectl args...>` command runs an arbitrary kubectl command against all active k3ctx contexts in parallel and collates output per context.
- The single-host connect flow is unchanged when `--all-hosts` is not set.

## Capabilities

### New Capabilities

- `multi-cluster-exec`: Run a kubectl command against all active k3ctx contexts in parallel, returning per-context stdout/stderr and exit status.

### Modified Capabilities

- `cluster-connection`: Add `--all-hosts <client>` flag to `connect`; when set, the system resolves all hosts for the client and connects to each in sequence, returning a JSON array of per-context `ConnectResult` entries.

## Impact

- `cli/connect.go`: add `--all-hosts` flag and multi-result output path.
- `internal/application/usecases/connect.go`: `ConnectMultiple` already exists; wire to client-scoped target list.
- New `cli/exec.go` + `internal/application/usecases/exec.go` for cross-cluster exec.
- New port interface `ClusterExec` in `internal/application/ports.go`.
- `internal/bootstrap/bootstrap.go`: wire new adapter.
- No external dependency changes; uses `os/exec` + `kubeconfig` context switching convention.
