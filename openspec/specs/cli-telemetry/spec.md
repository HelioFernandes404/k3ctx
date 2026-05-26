## Purpose

Define how k3ctx records structured telemetry events locally for diagnostic, usage analysis, and improvement purposes.

## Requirements

### Requirement: Record every CLI command as a telemetry event
The system SHALL append a structured JSONL event to the local telemetry file after every CLI command execution, capturing full diagnostic and usage data.

#### Scenario: Successful command produces a telemetry event
- **WHEN** the user runs any k3ctx command and it succeeds
- **THEN** the system appends one JSON line to `~/.local/share/k3ctx/telemetry/telemetry.jsonl` containing `ts`, `cmd`, `args`, `flags`, `duration_ms`, `ok: true`, `error: null`, and `version` fields

#### Scenario: Failed command records error in telemetry
- **WHEN** the user runs any k3ctx command and it fails with an error
- **THEN** the system appends one JSON line with `ok: false` and the full error message in the `error` field

#### Scenario: Event timestamp is RFC3339 UTC
- **WHEN** a telemetry event is written
- **THEN** the `ts` field is formatted as RFC3339 in UTC

#### Scenario: Duration is wall-clock milliseconds
- **WHEN** a telemetry event is written
- **THEN** the `duration_ms` field contains the wall-clock duration of the command in milliseconds

### Requirement: Inject binary version into telemetry events
The system SHALL record the binary version in every telemetry event using a version string injected at build time via ldflags.

#### Scenario: Version is set at build time
- **WHEN** the binary is built with `make build`
- **THEN** the `version` field in all telemetry events contains the output of `git describe --tags --always --dirty`

#### Scenario: Version falls back to "dev" when not injected
- **WHEN** the binary is built without ldflags (e.g., `go run`)
- **THEN** the `version` field defaults to `"dev"`

### Requirement: Rotate telemetry file by size
The system SHALL rotate the local telemetry file when it reaches 10 MB, retaining at most 3 files total.

#### Scenario: Active file rotates when size limit is exceeded
- **WHEN** appending an event would cause `telemetry.jsonl` to exceed 10 MB
- **THEN** the system renames the active file to `telemetry.jsonl.1` (shifting older files) and opens a new `telemetry.jsonl`

#### Scenario: Oldest file is deleted to enforce retention limit
- **WHEN** rotation occurs and 3 files already exist
- **THEN** the system deletes `telemetry.jsonl.2` before shifting, keeping at most 3 files total

### Requirement: Never block CLI execution on telemetry failure
The system SHALL silently suppress any telemetry write error and continue normal CLI operation.

#### Scenario: Telemetry directory is unwritable
- **WHEN** the telemetry directory cannot be created or written to
- **THEN** the CLI command completes normally and no error is surfaced to the user
