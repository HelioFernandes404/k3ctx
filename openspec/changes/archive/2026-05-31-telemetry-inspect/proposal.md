## Why

Telemetry events are already written to JSONL files but there is no way to inspect them from the CLI itself. Since Claude Code is the primary consumer, it needs a machine-readable way to review recent command history, durations, and error patterns without parsing raw JSONL files directly. Adding `tail` and `stats` subcommands exposes the existing telemetry data as structured output.

## What Changes

- A new `k3ctx telemetry` command group is added with two subcommands:
  - `k3ctx telemetry tail [--n N]` — outputs the last N events (default 20) across all rotated files, newest-first.
  - `k3ctx telemetry stats` — aggregates per-command counts, avg duration, and error rate across all available events.
- Both subcommands support `--json` for machine-readable output (inherited from root).
- Both are read-only; they never write or modify telemetry files.

## Capabilities

### New Capabilities

- `telemetry-inspect`: Read and aggregate local telemetry JSONL data via `tail` and `stats` subcommands.

### Modified Capabilities

(none — write behavior in `cli-telemetry` is unchanged)

## Impact

- New `cli/telemetry_cmd.go` Cobra command group.
- New `internal/telemetry/reader.go` with `ReadLastN` and `Aggregate` functions.
- `internal/paths` already exposes `TelemetryDir()` — reused.
- No new ports, infrastructure adapters, or config changes.
