## 1. Telemetry Reader

- [x] 1.1 Add `CommandStat` struct to `internal/telemetry/reader.go` with fields `Cmd`, `Count`, `OKCount`, `ErrorCount`, `AvgDuration`, `ErrorRate` (all JSON-tagged)
- [x] 1.2 Add `ReadLastN(dir string, n int) ([]map[string]any, error)` to `internal/telemetry/reader.go`: reads `telemetry.jsonl`, `.1`, `.2` newest-first; skips blank/invalid lines; returns up to n events in newest-first order
- [x] 1.3 Add `Aggregate(dir string) ([]CommandStat, error)` to `internal/telemetry/reader.go`: reads all available events, groups by `cmd`, computes count/ok/error/avg_duration/error_rate, returns sorted by count descending
- [x] 1.4 Unit tests for `ReadLastN` in `internal/telemetry/telemetry_test.go`: empty dir returns empty; single file returns last N; overflow reads from rotated file; corrupted lines skipped
- [x] 1.5 Unit tests for `Aggregate`: empty returns empty; multi-command events produce correct per-cmd stats; error_rate computed correctly

## 2. Telemetry CLI Commands

- [x] 2.1 Add `cli/telemetry_cmd.go` with `telemetryCmd` parent (no-op, prints help), `telemetryTailCmd`, and `telemetryStatsCmd` Cobra commands; register all three with `rootCmd`
- [x] 2.2 Implement `runTelemetryTail`: calls `telemetry.ReadLastN(paths.TelemetryDir(), tailN)`; JSON → `jsonEnvelope` with array; text → one compact JSON line per event; empty → `No telemetry data.`
- [x] 2.3 Implement `runTelemetryStats`: calls `telemetry.Aggregate(paths.TelemetryDir())`; JSON → `jsonEnvelope` with array; text → one line per cmd `<cmd>  count=<N>  errors=<E>  avg=<D>ms`; empty → `No telemetry data.`
- [x] 2.4 Add `--n` flag (default 20) to `telemetryTailCmd`
