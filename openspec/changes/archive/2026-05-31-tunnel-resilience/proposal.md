## Why

SSH tunnel processes die silently — the PID file remains but the port is no longer forwarded — causing `k3ctx status` to report a tunnel as running when it is broken. Since the CLI is driven by Claude Code (an AI agent), there is no human to notice the stale state; broken tunnels must be detected and healed automatically.

## What Changes

- `k3ctx status` performs a real TCP liveness probe on each tunnel's local port instead of relying solely on PID existence.
- A new `k3ctx tunnel-reconnect` command re-establishes a named broken tunnel without a full `connect` flow.
- `k3ctx tunnel-list` and `k3ctx status` distinguish between *running* (PID alive + port responds) and *dead* (PID file present but port unresponsive) states.
- Auto-reconnect can be triggered explicitly; no background daemon is introduced (keeps the CLI stateless and agent-friendly).

## Capabilities

### New Capabilities

- `tunnel-reconnect`: Re-establish a named broken tunnel using the stored connection parameters, without repeating the full connect flow (no inventory lookup, no kubeconfig re-merge).

### Modified Capabilities

- `tunnel-management`: Extend status/list reporting to include TCP liveness state (`running` vs `dead`); `k3ctx status` must probe each local port and report the real state; JSON output gains a `liveness` field.

## Impact

- `internal/infrastructure/tunnelstate` or equivalent: liveness probe logic (TCP dial with short timeout).
- `internal/application/usecases` (status use case): consumes liveness result.
- `cli/` (tunnel-related commands): surface `dead` state in text and JSON output.
- New use case + Cobra subcommand for `tunnel-reconnect`.
- Stored connection parameters must be retrievable from local state (SSH host, port, remote port) — verify whether kubeconfig cache contains enough to reconstruct the tunnel or if a new state file is needed.
