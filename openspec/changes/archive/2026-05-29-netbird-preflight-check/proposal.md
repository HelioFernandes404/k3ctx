## Why

When the NetBird daemon is in `NeedsLogin` state, `k3ctx connect` fails with a generic `CONNECTION_FAILED` error that gives no indication the root cause is an unauthenticated VPN daemon. Users are forced into manual diagnosis via `netbird status` instead of receiving an actionable fix.

## What Changes

- Before attempting SSH to any NetBird FQDN target, check that the local NetBird daemon is in `Connected` state.
- If the daemon is not ready, return a structured `NETBIRD_NOT_READY` error with the current daemon status and corrective hint (`netbird up`).
- Add a `--skip-netbird-check` flag to `k3ctx connect` to bypass the preflight in environments without NetBird.

## Capabilities

### New Capabilities

- `netbird-preflight`: Preflight check that validates the local NetBird daemon is authenticated and connected before any SSH attempt to a NetBird-hosted FQDN.

### Modified Capabilities

- `cluster-connection`: New preflight requirement — the system must verify NetBird daemon readiness before proceeding with SSH when the target host is a NetBird FQDN.

## Impact

- `cli/` — new `--skip-netbird-check` flag on the `connect` command.
- `internal/application/usecases` — preflight step injected before SSH resolution in the connect use case.
- `internal/infrastructure` — new NetBird status adapter (reuses `netbird status --json` output already parsed by peer-discovery).
- `internal/domain` — new `NETBIRD_NOT_READY` error code.
- No breaking changes to existing connection behavior when NetBird is healthy or target is non-NetBird.
