## Requirements

### Requirement: Launch k9s from PATH
The system SHALL provide a `k9s` command that launches the external `k9s` binary found in `PATH`.

#### Scenario: k9s binary is executed
- **WHEN** the user runs `k3ctx k9s` and `k9s` exists in `PATH`
- **THEN** the system starts that binary with inherited standard input, output, and error streams

#### Scenario: Missing k9s binary fails clearly
- **WHEN** the user runs `k3ctx k9s` and `k9s` is not found in `PATH`
- **THEN** the system returns an error indicating that `k9s` was not found
