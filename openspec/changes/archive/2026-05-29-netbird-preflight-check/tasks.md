## 1. Domain

- [x] 1.1 Add error code constants `NETBIRD_NOT_READY`, `PEER_NOT_CONNECTED`, `PEER_NOT_FOUND` to `internal/domain`
- [x] 1.2 Write unit tests asserting each constant exists and serialises correctly in `OperationError`

## 2. Infrastructure — NetBird preflight adapter

- [x] 2.1 Add `NetBirdPreflightChecker` port interface with two methods: `CheckDaemonReady(ctx context.Context, skipCheck bool) error` and `CheckPeerReady(ctx context.Context, fqdn string, skipCheck bool) error`
- [x] 2.2 Implement `netbirdPreflightChecker` in `internal/infrastructure`: resolve binary path (reusing `netbird_bin_path` / `NETBIRD_BIN_PATH` / `$PATH`), run `netbird status --json` once in `CheckDaemonReady` with 3s timeout, parse and cache the full JSON (daemon status + peer list) for reuse in `CheckPeerReady`
- [x] 2.3 `CheckDaemonReady`: binary not found → return nil (silent skip, sets internal skip flag)
- [x] 2.4 `CheckDaemonReady`: non-zero exit / daemon not running → return `NETBIRD_NOT_READY` with hint `"Run: netbird service start && netbird up"`
- [x] 2.5 `CheckDaemonReady`: status != `"Connected"` → return `NETBIRD_NOT_READY` with actual status in hint
- [x] 2.6 `CheckDaemonReady`: command timeout → return `NETBIRD_NOT_READY` with hint `"NetBird status check timed out. Run: netbird status"`
- [x] 2.7 `CheckPeerReady`: if daemon check was skipped (binary absent or skipCheck) → return nil
- [x] 2.8 `CheckPeerReady`: FQDN found in peer list with status `"connected"` → return nil
- [x] 2.9 `CheckPeerReady`: FQDN found but status != `"connected"` → return `PEER_NOT_CONNECTED` with peer name and status in hint
- [x] 2.10 `CheckPeerReady`: FQDN not found in peer list → return `PEER_NOT_FOUND` with FQDN in hint
- [x] 2.11 Write unit tests for all scenarios covering both methods (use command injection / fake binary)

## 3. Use Case — Connect two-step preflight

- [x] 3.1 Add `NetBirdPreflightChecker` port to the `Connect` use case struct
- [x] 3.2 Call `CheckDaemonReady` as the first step in `Connect`, before target resolution; propagate error as failed connect result
- [x] 3.3 Call `CheckPeerReady` after target resolution, passing the resolved FQDN (`ansible_host`), before SSH resolution; propagate error as failed connect result
- [x] 3.4 Pass `--skip-netbird-check` from input DTO to both check calls
- [x] 3.5 Write use-case unit tests: daemon failure aborts before resolution, peer failure aborts after resolution, skip bypasses both, success continues through both

## 4. CLI — Flag

- [x] 4.1 Add `--skip-netbird-check` boolean flag to the `connect` Cobra command in `cli/`
- [x] 4.2 Pass the flag value into the connect input DTO / use case call

## 5. Bootstrap — Wiring

- [x] 5.1 Instantiate `netbirdPreflightChecker` in `internal/bootstrap/bootstrap.go` and inject it into the `Connect` use case

## 6. Validation

- [x] 6.1 Run `go test ./...` — all tests pass
- [x] 6.2 Run `make lint` — no new lint issues
- [x] 6.3 Smoke test: daemon in `NeedsLogin` → `NETBIRD_NOT_READY` JSON with correct hint
- [x] 6.4 Smoke test: daemon `Connected`, target peer `Disconnected` → `PEER_NOT_CONNECTED` JSON
- [x] 6.5 Smoke test: `--skip-netbird-check` → nenhum erro de preflight independente do estado do daemon
