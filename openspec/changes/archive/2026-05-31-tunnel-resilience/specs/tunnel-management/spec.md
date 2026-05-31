## MODIFIED Requirements

### Requirement: Report context and tunnel status
The system SHALL report active managed tunnel status from local PID state combined with a TCP liveness probe on the tunnel's local port.

#### Scenario: No active tunnels message
- **WHEN** the user runs `k3ctx status` and no managed tunnel PID files are present
- **THEN** the system prints `No active tunnels.`

#### Scenario: Live tunnel is marked
- **WHEN** the user runs `k3ctx status` and a context has a running managed tunnel whose local port responds to a TCP dial within 2 seconds
- **THEN** the system prints the context name with a tunnel-running marker and liveness `live`

#### Scenario: Stale tunnel is marked
- **WHEN** the user runs `k3ctx status` and a context has a running managed tunnel (PID alive) whose local port does not respond within 2 seconds
- **THEN** the system prints the context name with a stale-tunnel marker and liveness `stale`

#### Scenario: Dead tunnel is marked
- **WHEN** the user runs `k3ctx status` and a PID file exists but the process is no longer alive
- **THEN** the system prints the context name with a dead-tunnel marker and liveness `dead`

#### Scenario: Status supports JSON output with liveness field
- **WHEN** the user runs `k3ctx status --json`
- **THEN** the system outputs status entries as JSON, each entry including `tunnel_running` (bool, preserved for backward compatibility) and `liveness` (string: `live`, `stale`, or `dead`)

## ADDED Requirements

### Requirement: Persist SSH connection parameters at connect time
The system SHALL write a connection sidecar file `<context>.conn.json` to the tunnel state directory upon a successful cluster connection.

#### Scenario: Sidecar file written after successful connect
- **WHEN** a cluster connection completes successfully and a tunnel PID is recorded
- **THEN** the system writes `<stateDir>/<contextName>.conn.json` containing `ssh_host`, `internal_ip`, `local_port`, `remote_port`, `username`, `key_file`, `ssh_port`, and `proxy_cmd`

#### Scenario: Sidecar file absent does not break status
- **WHEN** the user runs `k3ctx status` and a context has a PID file but no `.conn.json`
- **THEN** the system reports liveness normally (PID + TCP probe) without error
