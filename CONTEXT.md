# k3ctx — Domain Glossary

## Telemetry

**Telemetry**: Structured runtime event log written locally by the k3ctx CLI. Purpose: diagnostic, usage analysis, and improvement direction.

**Telemetry Event**: One JSONL line appended to the telemetry file on each CLI command execution. Contains: `ts` (RFC3339), `cmd` (command name), `args` (full argument list including values), `flags` (flag names used), `duration_ms` (wall-clock duration), `ok` (boolean success), `error` (full error message or null), `version` (binary version string).

**Telemetry File**: Local append-only JSONL file at `~/.local/share/k3ctx/telemetry/telemetry.jsonl`. Always-on — no opt-in or opt-out.

**Telemetry Rotation**: Size-based log rotation. Maximum 10 MB per file. Maximum 3 files retained (telemetry.jsonl, telemetry.jsonl.1, telemetry.jsonl.2). Rotation occurs at write time when the active file exceeds the size limit.
