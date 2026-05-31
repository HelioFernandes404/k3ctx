## ADDED Requirements

### Requirement: Connect to all hosts in a client with a single command
The system SHALL accept `--all-hosts <client>` on the `connect` command to connect to every host belonging to the named client in sequence, without requiring the caller to enumerate hosts manually.

#### Scenario: All-hosts connect returns array of results
- **WHEN** the user runs `k3ctx connect --all-hosts <client> --json`
- **THEN** the system outputs a JSON array where each element is a per-context connect result (same schema as a single connect result), including both successful and failed connections

#### Scenario: All-hosts connect continues on per-host failure
- **WHEN** the user runs `k3ctx connect --all-hosts <client>` and one or more hosts fail to connect
- **THEN** the system continues connecting remaining hosts and reports all results, including failures

#### Scenario: All-hosts connect with unknown client returns no-match error
- **WHEN** the user runs `k3ctx connect --all-hosts <client>` and no hosts exist for that client
- **THEN** the system exits with a `NO_MATCH` error

#### Scenario: All-hosts flag is mutually exclusive with identifier arguments
- **WHEN** the user runs `k3ctx connect --all-hosts <client>` with additional positional arguments or `--host`/`--id`/`--ip`/`--context` flags
- **THEN** the system exits with a usage error explaining that `--all-hosts` cannot be combined with other host filters

#### Scenario: All-hosts text output lists connected contexts
- **WHEN** the user runs `k3ctx connect --all-hosts <client>` in text mode
- **THEN** the system prints one line per host: `Connected: <context>` for successes and `Failed: <context>: <reason>` for failures
