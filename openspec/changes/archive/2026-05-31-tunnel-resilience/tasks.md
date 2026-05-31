## 1. TCP Liveness Probe

- [x] 1.1 Add `IsPortLive(localPort int, timeout time.Duration) bool` to `internal/tunnel/tunnel.go` using `net.DialTimeout` on `localhost:<port>`
- [x] 1.2 Add `TunnelLiveness(contextName, stateDir string) string` to `internal/tunnel/tunnel.go` returning `"live"`, `"stale"`, or `"dead"` based on PID check + TCP probe

## 2. Status Reporting

- [x] 2.1 Update `LocalStatusReader.ListContextStatus` in `internal/infrastructure/statusreader.go` to call `TunnelLiveness` and include `"liveness"` key in each map entry (keep `"tunnel_running"` for backward compat)
- [x] 2.2 Update `k3ctx status` CLI text output to display `[live]`, `[stale]`, or `[dead]` beside each context name
- [x] 2.3 Add unit tests for `TunnelLiveness` in `internal/tunnel/tunnel_test.go` covering all three states
- [x] 2.4 Update `statusreader` tests to assert `liveness` field is present in output

## 3. Connection Sidecar

- [x] 3.1 Define `ConnParams` struct in `internal/tunnel` package (fields: SSHHost, InternalIP, LocalPort, RemotePort, Username, KeyFile, SSHPort, ProxyCmd) serialised as JSON
- [x] 3.2 Add `SaveConnParams(contextName, stateDir string, p ConnParams) error` to `internal/tunnel`
- [x] 3.3 Add `LoadConnParams(contextName, stateDir string) (ConnParams, error)` to `internal/tunnel` — returns a typed error when file absent
- [x] 3.4 Call `SaveConnParams` in `internal/infrastructure/cluster.go` after a successful `CreateTunnel`, using the same parameters passed to the tunnel
- [x] 3.5 Unit test `SaveConnParams` / `LoadConnParams` roundtrip in `internal/tunnel/tunnel_test.go`

## 4. Tunnel Reconnect Command

- [x] 4.1 Add `TunnelReconnector` interface to `internal/application/ports.go` with `ReconnectTunnel(contextName string) error`
- [x] 4.2 Implement `ReconnectTunnel` on `LocalTunnelManager` in `internal/infrastructure/tunnelmgr.go`: load `.conn.json`, kill existing tunnel, call `tunnel.CreateTunnel`, save new PID
- [x] 4.3 Add `ReconnectTunnel(contextName string, reconnector application.TunnelReconnector) error` use case in `internal/application/usecases/tunnels.go`
- [x] 4.4 Add `tunnel-reconnect` Cobra subcommand in `cli/` with `--json` flag; output `context_name`, `local_port`, `ok`, `error`
- [x] 4.5 Wire `TunnelReconnector` in `internal/bootstrap/bootstrap.go`
- [x] 4.6 Unit test `ReconnectTunnel` use case: missing sidecar returns `no connection state` error; present sidecar delegates to reconnector
