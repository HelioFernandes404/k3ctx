## ADDED Requirements

### Requirement: Run NetBird preflight before SSH for authenticated daemon check
The system SHALL execute the NetBird preflight check at the start of the connect flow, before SSH resolution, and abort with a `NETBIRD_NOT_READY` error if the check fails.

#### Scenario: Preflight failure aborts connection with structured error
- **WHEN** the NetBird preflight check returns a `NETBIRD_NOT_READY` error
- **THEN** the system returns a failed connect result with `code: "NETBIRD_NOT_READY"` and the daemon status hint, without attempting SSH

#### Scenario: Preflight success allows connection to proceed
- **WHEN** the NetBird preflight check passes (daemon is Connected or binary is absent)
- **THEN** the system continues to SSH resolution and tunnel setup as normal

#### Scenario: NETBIRD_NOT_READY error included in JSON output
- **WHEN** `k3ctx connect --json` fails due to `NETBIRD_NOT_READY`
- **THEN** the JSON output contains `"ok": false`, `"error.code": "NETBIRD_NOT_READY"`, and the daemon status in `"error.hint"`
