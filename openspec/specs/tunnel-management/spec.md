## Requirements

### Requirement: Report context and tunnel status
The system SHALL report active managed tunnel status from local PID state.

#### Scenario: No active tunnels message
- **WHEN** the user runs `k3ctx status` and no managed tunnel PID files are present
- **THEN** the system prints `No active tunnels.`

#### Scenario: Running tunnel is marked
- **WHEN** the user runs `k3ctx status` and a context has a running managed tunnel
- **THEN** the system prints the context name with a tunnel-running marker

#### Scenario: Status supports JSON output
- **WHEN** the user runs `k3ctx status --json`
- **THEN** the system outputs status entries as JSON

### Requirement: List active managed tunnels
The system SHALL provide a command that lists context names for active managed SSH tunnels.

#### Scenario: Tunnel list shows running contexts
- **WHEN** the user runs `k3ctx tunnel-list`
- **THEN** the system prints context names whose managed tunnels are running

### Requirement: Kill one managed tunnel
The system SHALL provide a command that terminates a managed tunnel by context name.

#### Scenario: Tunnel kill delegates to manager
- **WHEN** the user runs `k3ctx tunnel-kill CONTEXT`
- **THEN** the system attempts to terminate the managed tunnel for `CONTEXT`

### Requirement: Kill all managed tunnels
The system SHALL provide a command that terminates all managed tunnels represented by local PID files.

#### Scenario: Tunnel kill all iterates PID files
- **WHEN** the user runs `k3ctx tunnel-kill-all`
- **THEN** the system scans the managed tunnel state directory for PID files and attempts to kill each corresponding context tunnel

### Requirement: Use default tunnel state directory
The system SHALL use the local state tunnel directory for managed tunnel PID files unless a test adapter overrides it.

#### Scenario: Default state directory is used
- **WHEN** runtime code needs the managed tunnel state directory
- **THEN** the system uses `$HOME/.local/state/k3ctx-tunnels`
