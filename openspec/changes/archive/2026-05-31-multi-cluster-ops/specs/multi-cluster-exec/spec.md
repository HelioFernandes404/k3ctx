## ADDED Requirements

### Requirement: Run a kubectl command across all live k3ctx contexts
The system SHALL provide a `k3ctx exec -- <kubectl args...>` command that runs the given kubectl command against every k3ctx-managed context whose tunnel liveness is `live`, in parallel, and collates the results.

#### Scenario: Exec runs against all live contexts
- **WHEN** the user runs `k3ctx exec -- get nodes` and two live tunnels exist
- **THEN** the system runs `kubectl --context <name> get nodes` for each live context and returns all results

#### Scenario: Exec skips non-live contexts
- **WHEN** a k3ctx-managed context has `liveness` of `stale` or `dead`
- **THEN** the system does not attempt exec against that context

#### Scenario: Exec returns per-context result array in JSON
- **WHEN** the user runs `k3ctx exec --json -- <kubectl args...>`
- **THEN** the system outputs a JSON array where each element contains `context` (string), `ok` (bool), `stdout` (string), `stderr` (string), and `exit_code` (int)

#### Scenario: Exec text output uses context headers
- **WHEN** the user runs `k3ctx exec -- <kubectl args...>` in text mode
- **THEN** the system prints `=== <context> ===` before each context's output

#### Scenario: No live tunnels returns empty result
- **WHEN** the user runs `k3ctx exec -- <kubectl args...>` and no live tunnels exist
- **THEN** the system outputs an empty array in JSON mode or prints `No live tunnels.` in text mode

#### Scenario: Per-context timeout prevents hanging
- **WHEN** kubectl does not respond within the configured timeout (default 30s)
- **THEN** the system kills the kubectl process for that context and records a timeout error in the result

#### Scenario: Timeout is configurable
- **WHEN** the user passes `--timeout <seconds>` to `k3ctx exec`
- **THEN** the system uses that value as the per-context kubectl timeout

### Requirement: Require double-dash separator before kubectl arguments
The system SHALL require a `--` separator between `k3ctx exec` flags and the kubectl arguments to prevent ambiguous flag parsing.

#### Scenario: Missing separator exits with usage error
- **WHEN** the user runs `k3ctx exec get nodes` without `--`
- **THEN** the system exits with a usage error explaining that `--` is required before kubectl arguments
