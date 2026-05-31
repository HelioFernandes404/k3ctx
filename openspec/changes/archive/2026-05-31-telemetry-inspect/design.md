## Context

Telemetry is written to `~/.local/share/k3ctx/telemetry/telemetry.jsonl` with rotation at 10 MB keeping 3 files total (`telemetry.jsonl`, `telemetry.jsonl.1`, `telemetry.jsonl.2`). Each line is a JSON object with fields `ts`, `cmd`, `args`, `flags`, `duration_ms`, `ok`, `error`, `version`. Reading is purely additive — no existing code changes.

## Goals / Non-Goals

**Goals:**
- `ReadLastN(dir string, n int) ([]map[string]any, error)` reads all files newest-first, collects up to N events, returns them in newest-first order.
- `Aggregate(dir string) ([]CommandStat, error)` reads all available events and returns per-command aggregates.
- Both functions live in `internal/telemetry/reader.go` alongside the existing writer.
- `k3ctx telemetry tail` and `k3ctx telemetry stats` surface these in `cli/telemetry_cmd.go`.

**Non-Goals:**
- Streaming / watching the file for new events.
- Filtering by time range or command name (Claude Code can filter the JSON output).
- Writing or modifying telemetry files.

## Decisions

### D1 — Read full files, take last N lines

Each file is at most 10 MB. Reading the entire file into memory is safe. Split by newline, skip blank/invalid lines, collect up to N from the tail of the newest file, overflow into older files.

**Alternative considered:** `tail -n` style reverse byte scan. Rejected — adds complexity with negligible benefit for ≤10 MB files.

### D2 — `CommandStat` struct, not `map[string]any`

```go
type CommandStat struct {
    Cmd         string
    Count       int
    OKCount     int
    ErrorCount  int
    AvgDuration float64 // ms
    ErrorRate   float64 // 0–1
}
```

Returned as a JSON array sorted by `Count` descending. Typed struct avoids key typos and is directly serialisable.

### D3 — Telemetry subcommand as a Cobra parent with two children

`k3ctx telemetry` is registered as a parent command with `tail` and `stats` as subcommands. No action on the parent itself (prints help). This follows Cobra conventions already used in the project.

### D4 — No new port interface

Both functions are pure file-system reads in the `telemetry` package. No dependency injection needed — the dir path comes from `paths.TelemetryDir()`, already a public function used in `root.go`.

## Risks / Trade-offs

- **Empty/missing telemetry dir** → `ReadLastN` and `Aggregate` return empty results with no error (not an error condition).
- **Corrupted JSONL lines** → silently skipped; valid lines are still returned.
- **Large event volume for stats** → all files read in full. Bounded by 3 × 10 MB = 30 MB max; acceptable for a CLI tool.
