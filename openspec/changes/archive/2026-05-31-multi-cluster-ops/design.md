## Context

`ConnectMultiple` already exists in `internal/application/usecases/connect.go` and accepts a `[]domain.ClusterTarget` slice. The `connect` CLI currently resolves exactly one target and errors on ambiguous matches. For multi-cluster, the resolution step is skipped entirely — all hosts for a client are fetched directly via `InventoryCatalog.ListTargets` filtered by client name.

For cross-cluster exec, the active context list comes from `StatusReader.ListContextStatus()`, which after the `tunnel-resilience` change includes a `liveness` field. Only `live` tunnels should receive the exec.

## Goals / Non-Goals

**Goals:**
- `k3ctx connect --all-hosts <client>` connects to every host in the named client, continues on per-host failure, and returns a JSON array of per-context results.
- `k3ctx exec -- <kubectl args...>` runs the given kubectl command against all live k3ctx-managed contexts in parallel, with per-context stdout/stderr in the output.

**Non-Goals:**
- Interactive progress reporting (CLI is consumed by Claude Code).
- Streaming output per context as it completes (all output arrives at end).
- `--all-hosts` with filters other than `--client` (exactly one client name required).
- `exec` against a user-chosen subset of contexts (always all live tunnels).

## Decisions

### D1 — `--all-hosts` bypasses resolution, uses `ListTargets` directly

`ResolveHost` is designed to return exactly one match. For `--all-hosts`, we skip it entirely: call `catalog.ListTargets(inventoryPath)`, filter by `target.Client() == allHostsClient`, and pass the slice to `ConnectMultiple`.

**Rationale:** Reusing the resolution path would require relaxing its single-match invariant. Direct `ListTargets` is already the backing store; filtering by client is trivial.

**Alternative considered:** Adding a `ResolveAllHosts(client)` use case. Unnecessary indirection — `ListTargets` + filter is two lines.

### D2 — `--all-hosts` continues on per-host failure, returns full result array

`ConnectMultiple` already returns `[]domain.ConnectResult`. Each `ConnectResult` carries `Success()` and `Err()`. The CLI iterates the slice and emits a JSON array regardless of individual failures.

**Rationale:** Partial connectivity is the normal case in multi-cluster (some hosts may be offline). The agent must see all results to decide next steps, not just the first failure.

### D3 — `exec` reads live contexts from `StatusReader`, execs via `kubectl --context`

`exec` fetches `StatusReader.ListContextStatus()`, filters for `liveness == "live"`, then runs `kubectl --context <name> <args...>` for each in parallel goroutines with a per-context timeout (default 30s, overridable with `--timeout`).

**Rationale:** `StatusReader` is the authoritative source of k3ctx-managed tunnels. Using `liveness == "live"` avoids sending kubectl to broken tunnels and getting confusing timeout errors.

**Alternative considered:** Reading kubeconfig contexts directly. Rejected — kubeconfig may contain non-k3ctx contexts; StatusReader already scopes to k3ctx-managed tunnels.

### D4 — `exec` output: JSON array of `{context, ok, stdout, stderr, exit_code}`

Each entry always present regardless of success. Text mode prints a `=== <context> ===` header followed by stdout/stderr.

**Rationale:** Agent-consumable format. Consistent structure regardless of per-context outcome.

### D5 — New `ClusterExec` port interface, implemented by `LocalClusterExec`

The interface is `ExecOnContext(contextName string, args []string, timeout time.Duration) (stdout, stderr string, exitCode int, err error)`. It shells out to `kubectl`.

**Rationale:** Keeps kubectl invocation injectable for testing without requiring a real cluster.

## Risks / Trade-offs

- **`--all-hosts` with large clients** → Sequential connect per host; 10+ hosts may take minutes. Acceptable since the agent sets its own timeouts. Parallel connect is a future optimization.
- **`exec` goroutine leak on process hang** → Each goroutine runs `exec.CommandContext` with the per-context timeout, so the process is killed on deadline. No leak.
- **`liveness` filter too strict** → If a tunnel is `stale` (PID alive, port closed) exec would skip it. Correct behaviour: the tunnel is broken; agent should call `tunnel-reconnect` first.
