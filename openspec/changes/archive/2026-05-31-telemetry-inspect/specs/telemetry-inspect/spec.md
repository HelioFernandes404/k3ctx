## ADDED Requirements

### Requirement: Show last N telemetry events
The system SHALL provide a `k3ctx telemetry tail` subcommand that outputs the most recent telemetry events in newest-first order.

#### Scenario: Tail returns last N events as JSON array
- **WHEN** the user runs `k3ctx telemetry tail --json`
- **THEN** the system outputs a JSON array of the last 20 events (default), each preserving all original JSONL fields (`ts`, `cmd`, `args`, `flags`, `duration_ms`, `ok`, `error`, `version`)

#### Scenario: Tail count is configurable
- **WHEN** the user runs `k3ctx telemetry tail --n 5`
- **THEN** the system returns at most 5 events

#### Scenario: Tail reads across rotated files
- **WHEN** fewer than N events exist in the active file and rotated files are present
- **THEN** the system reads from `telemetry.jsonl.1` and `telemetry.jsonl.2` to reach N total

#### Scenario: Tail with no telemetry data returns empty result
- **WHEN** no telemetry files exist
- **THEN** the system outputs an empty array in JSON mode or `No telemetry data.` in text mode

#### Scenario: Tail text mode prints one event per line
- **WHEN** the user runs `k3ctx telemetry tail` in text mode
- **THEN** the system prints one compact JSON line per event to stdout

### Requirement: Show per-command telemetry statistics
The system SHALL provide a `k3ctx telemetry stats` subcommand that aggregates telemetry events by command name and returns counts and performance metrics.

#### Scenario: Stats returns per-command aggregates as JSON array
- **WHEN** the user runs `k3ctx telemetry stats --json`
- **THEN** the system outputs a JSON array sorted by count descending, each entry containing `cmd`, `count`, `ok_count`, `error_count`, `avg_duration_ms`, and `error_rate` (0–1 float)

#### Scenario: Stats text mode prints a table
- **WHEN** the user runs `k3ctx telemetry stats` in text mode
- **THEN** the system prints one line per command in the format `<cmd>  count=<N>  errors=<E>  avg=<D>ms`

#### Scenario: Stats with no telemetry data returns empty result
- **WHEN** no telemetry files exist
- **THEN** the system outputs an empty array in JSON mode or `No telemetry data.` in text mode

#### Scenario: Corrupted JSONL lines are skipped silently
- **WHEN** a telemetry file contains malformed JSON on one or more lines
- **THEN** those lines are ignored and valid events are still included in the output
