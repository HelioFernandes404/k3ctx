## Context

`k3ctx connect` resolves a cluster target and then opens an SSH tunnel to it. When targets are discovered via NetBird peer-discovery, the SSH hostname is a NetBird FQDN (e.g., `sf-prd-us-00002.systemframe.vpn`). This FQDN only resolves when the local NetBird daemon is authenticated and in `Connected` state.

Currently, if the daemon is in `NeedsLogin` (common after reboot or idle timeout), SSH fails with a generic `CONNECTION_FAILED` error. The domain message carries no information about the VPN layer, forcing the user into manual `netbird status` diagnosis.

The peer-discovery infrastructure already executes `netbird status --json` to enumerate peers. The daemon state is available in that same JSON payload but is discarded after peer mapping.

## Goals / Non-Goals

**Goals:**
- Check NetBird daemon status before any SSH attempt when the NetBird binary is present.
- Check target peer connectivity after target resolution, using the same `netbird status --json` output.
- Return structured errors (`NETBIRD_NOT_READY`, `PEER_NOT_CONNECTED`, `PEER_NOT_FOUND`) with actionable hints.
- Allow bypassing both checks via `--skip-netbird-check` flag for non-NetBird environments.
- Reuse the existing NetBird binary path resolution (config / `NETBIRD_BIN_PATH` / `$PATH`).

**Non-Goals:**
- Auto-authenticating or reconnecting the NetBird daemon.
- Modifying peer-discovery behavior.
- Supporting NetBird API (only CLI).

## Decisions

### 1. Check unconditionally when binary is present, not per target type

**Decision**: Run preflight whenever the `netbird` binary is reachable on the configured path or `$PATH`, regardless of whether the target hostname looks like a NetBird FQDN.

**Rationale**: Custom NetBird domains (e.g., `.systemframe.vpn`) cannot be reliably detected by suffix. Checking whether the discovery source is NetBird would require threading source metadata through the use case. Unconditional check on binary presence is simpler and correct: if the binary is installed, the environment uses NetBird; if the binary is absent, the check silently skips without penalty.

**Alternative considered**: Per-target detection via FQDN suffix — rejected because custom domains make this fragile.

### 2. Parse `netbird status --json` for daemon state

**Decision**: Execute `netbird status --json` and extract the top-level `status` field (e.g., `"Connected"`, `"NeedsLogin"`, `"Disconnected"`). Treat only `"Connected"` as ready.

**Rationale**: The JSON output is already consumed by peer-discovery. Reusing the same command and parser avoids a second subprocess call format and keeps the binary path resolution path identical.

**Alternative considered**: Parse human-readable `netbird status` output — rejected because it is locale- and version-sensitive.

### 3. Fail fast with `NETBIRD_NOT_READY`, not warn-and-continue

**Decision**: When the daemon status is not `Connected`, abort the connect flow immediately with a structured `OperationError{Code: "NETBIRD_NOT_READY"}`.

**Rationale**: Continuing would result in a silent SSH failure anyway. Failing fast with a clear code and hint (`Run: netbird up`) eliminates all diagnostic ambiguity.

### 4. Split into two steps: daemon check then peer check

**Decision**: The preflight runs as two distinct use-case steps:
1. **Daemon check** — at the very start of `Connect`, before target resolution. Validates daemon is `Connected`.
2. **Peer check** — after target resolution, before SSH. Receives the resolved FQDN, finds the matching peer in the already-parsed peer list, validates peer-level status.

**Rationale**: Failing fast on daemon state (step 1) avoids paying the cost of target resolution when the daemon is offline. The peer check (step 2) eliminates the false-negative class: daemon reports `Connected` globally but the specific target peer is `Disconnected` or absent. Both steps use the same single `netbird status --json` invocation — the JSON is parsed once and reused.

**Alternative considered**: Single post-resolution check — simpler but wastes target resolution cost when daemon is offline.

### 5. Inject preflight as a use-case port, not an infrastructure adapter concern

**Decision**: The preflight checker is injected into the `Connect` use case via a port interface with two methods: `CheckDaemonReady(ctx, skipCheck)` and `CheckPeerReady(ctx, fqdn, skipCheck)`. The infrastructure adapter executes `netbird status --json` once in `CheckDaemonReady` and caches the parsed peer list for `CheckPeerReady`.

**Rationale**: Keeps orchestration in the use case. Mirrors how `APIReadiness` is currently injected.

### 6. Skip gracefully when binary is absent; fail on daemon not running

**Decision**: Binary not found → skip silently (return nil). Daemon not running (non-zero exit) → `NETBIRD_NOT_READY` with hint `"Run: netbird service start && netbird up"`.

**Rationale**: Binary absence means a non-NetBird environment — no penalty. A running-but-not-authenticated daemon is a fixable user error that warrants an explicit error.

## Risks / Trade-offs

- **Latency**: `netbird status --json` adds ~50–200ms per connect call. Acceptable given connect is already SSH-bound. → Mitigation: 3s timeout, JSON parsed once and reused between daemon and peer checks.
- **Peer list staleness**: The JSON snapshot may not reflect a peer that just connected. → Acceptable; the window is milliseconds and the alternative is a second subprocess call.
- **Binary path drift**: If the user changes `netbird_bin_path` after install, check may target wrong binary. → Mitigation: same path resolution used by peer-discovery; single source of truth.
