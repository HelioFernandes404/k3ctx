## Context

Currently `IsTunnelRunning` in `internal/tunnel/tunnel.go` checks PID existence via `signal(0)`. The PID can be alive while the SSH port-forward is broken (e.g. network drop, SSH server restart), leaving `k3ctx status` reporting `tunnel_running: true` for a dead forward. Since the CLI is consumed by Claude Code (not a human), silent stale state causes agent failures with no recovery path.

Reconnection today requires a full `k3ctx connect` cycle — inventory lookup, kubeconfig merge, ArgoCD/Alertmanager/VictoriaMetrics discovery — even though the kubeconfig is already merged and only the SSH tunnel needs to be re-established. The SSH parameters needed to recreate the tunnel are not persisted anywhere after the initial `connect`.

## Goals / Non-Goals

**Goals:**
- `k3ctx status` reports actual port liveness, not just PID state.
- A new `k3ctx tunnel-reconnect <context>` command re-establishes a broken tunnel using persisted SSH parameters.
- JSON output gains a `liveness` field (`live`, `dead`, `stale`) for agent consumption.
- Minimal new state: one JSON sidecar file per tunnel, written at connect time.

**Non-Goals:**
- Background daemon or automatic reconnection without explicit invocation.
- Re-running ArgoCD/Alertmanager/VictoriaMetrics discovery on reconnect.
- Reconnecting tunnels opened before this change ships (no `.conn.json` present → graceful skip).

## Decisions

### D1 — TCP liveness probe in `internal/tunnel` package

Add `IsPortLive(localPort int, timeout time.Duration) bool` to `internal/tunnel/tunnel.go`. It attempts `net.DialTimeout("tcp", "localhost:<port>", timeout)` and returns true on success.

**Rationale:** Keeps the probe co-located with PID logic. The local port is already deterministic (`GetUniquePort`) so no new state is needed to compute it. Timeout of 2s is sufficient for a local loopback check.

**Alternative considered:** Probe inside `statusreader.go`. Rejected — leaks network logic into infrastructure layer.

### D2 — Three-state liveness: `live`, `stale`, `dead`

- `live`: PID alive **and** port responds.
- `stale`: PID alive but port does not respond (process hung or SSH keepalive not yet triggered).
- `dead`: PID absent or process gone.

`tunnel_running` (bool) is kept for backward compatibility; `liveness` (string) is added alongside it.

**Rationale:** `stale` is actionable — agent can issue `tunnel-reconnect` or `tunnel-kill` + reconnect. Collapsing stale into dead hides information useful for debugging.

### D3 — Connection sidecar file `<context>.conn.json`

Written to `~/.local/state/k3ctx-tunnels/<context>.conn.json` at the end of a successful `connect`. Contains the minimum SSH parameters needed to recreate the tunnel:

```json
{
  "ssh_host":    "sf-ams-nl-00001.netbird.cloud",
  "internal_ip": "10.28.0.1",
  "local_port":  28042,
  "remote_port": 6443,
  "username":    "ubuntu",
  "key_file":    "/home/user/.ssh/id_ed25519",
  "ssh_port":    22,
  "proxy_cmd":   ""
}
```

`tunnel-reconnect` reads this file, calls `tunnel.CreateTunnel`, and overwrites the PID file.

**Alternative considered:** Re-derive parameters from kubeconfig + config file. Rejected — kubeconfig stores only `localhost:<port>` (no SSH host), and config file does not store per-host SSH params resolved at connect time.

**Risk:** File contains SSH key path (not the key itself). Safe to store; path is already in `~/.ssh/`.

### D4 — `tunnel-reconnect` is explicit, not automatic

`k3ctx tunnel-reconnect <context>` is a new Cobra subcommand under `tunnel`. It does not auto-trigger from `status`. Claude Code reads status JSON, detects `liveness: stale` or `dead`, and explicitly calls `tunnel-reconnect`.

**Rationale:** Keeps the CLI stateless and predictable. Auto-reconnect in a background goroutine would require a daemon and complicates process management.

## Risks / Trade-offs

- **`.conn.json` missing for old tunnels** → `tunnel-reconnect` returns a clear error: `no connection state for context <name>; run k3ctx connect first`. Not a silent failure.
- **Port collision on reconnect** → `GetUniquePort` is deterministic; the old tunnel must be killed first (or be dead). `tunnel-reconnect` calls `KillTunnel` before re-dialing to ensure the port is free.
- **2s TCP timeout on status** → With many tunnels this could slow `k3ctx status`. Probes run sequentially per tunnel entry; acceptable for expected tunnel counts (< 20). Parallel probes can be added later if needed.
- **Backward compat** → `tunnel_running` preserved in JSON output; consumers only need to add handling for `liveness`.

## Migration Plan

1. Ship `IsPortLive` + liveness reporting in status (no new files written yet — purely additive).
2. Ship `.conn.json` write in `connect` infrastructure (`cluster.go`).
3. Ship `tunnel-reconnect` command (reads `.conn.json`).
4. Old tunnels without `.conn.json` continue to work; `tunnel-reconnect` on them returns a clear error.

No rollback complexity — new fields are additive, new file is optional.
