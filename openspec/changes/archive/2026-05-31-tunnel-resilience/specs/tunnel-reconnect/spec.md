## ADDED Requirements

### Requirement: Re-establish a broken tunnel by context name
The system SHALL provide a `tunnel-reconnect` command that re-establishes an SSH tunnel for a named context using persisted connection parameters, without repeating inventory lookup or kubeconfig merge.

#### Scenario: Successful reconnect
- **WHEN** the user runs `k3ctx tunnel-reconnect <context>` and a `.conn.json` sidecar exists for that context
- **THEN** the system kills any existing tunnel for that context, opens a new SSH tunnel using the stored parameters, updates the PID file, and reports success

#### Scenario: No connection state available
- **WHEN** the user runs `k3ctx tunnel-reconnect <context>` and no `.conn.json` sidecar exists for that context
- **THEN** the system exits with a clear error: `no connection state for context <name>; run k3ctx connect first`

#### Scenario: Reconnect kills stale process before re-dialing
- **WHEN** the user runs `k3ctx tunnel-reconnect <context>` and the tunnel process is still alive (stale state)
- **THEN** the system sends SIGTERM to the existing process before opening the new tunnel

#### Scenario: Reconnect reports new local port
- **WHEN** `k3ctx tunnel-reconnect <context>` succeeds
- **THEN** the system prints the context name and the local port the tunnel was re-established on

#### Scenario: Reconnect supports JSON output
- **WHEN** the user runs `k3ctx tunnel-reconnect <context> --json`
- **THEN** the system outputs a JSON object with `context_name`, `local_port`, `ok` (bool), and `error` (string, omitted when ok)
