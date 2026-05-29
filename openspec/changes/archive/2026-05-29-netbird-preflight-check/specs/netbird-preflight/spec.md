## ADDED Requirements

### Requirement: Validate NetBird daemon readiness before connection
The system SHALL check the local NetBird daemon status before any SSH connection attempt when the NetBird binary is available, and fail with a structured error if the daemon is not in `Connected` state.

#### Scenario: Daemon is Connected — check passes
- **WHEN** `netbird status --json` returns a `status` field of `"Connected"`
- **THEN** the preflight check succeeds and the connect flow continues normally

#### Scenario: Daemon is NeedsLogin — check fails with NETBIRD_NOT_READY
- **WHEN** `netbird status --json` returns a `status` field of `"NeedsLogin"`
- **THEN** the system returns a `NETBIRD_NOT_READY` error with hint `"NetBird daemon status: NeedsLogin. Run: netbird up"`

#### Scenario: Daemon is Disconnected — check fails with NETBIRD_NOT_READY
- **WHEN** `netbird status --json` returns a `status` field of `"Disconnected"`
- **THEN** the system returns a `NETBIRD_NOT_READY` error with hint `"NetBird daemon status: Disconnected. Run: netbird up"`

#### Scenario: NetBird binary not found — check is skipped
- **WHEN** the NetBird binary cannot be located on the configured path or `$PATH`
- **THEN** the preflight check is silently skipped and the connect flow continues normally

#### Scenario: NetBird daemon not running — check fails with NETBIRD_NOT_READY
- **WHEN** `netbird status --json` exits with a non-zero code because the daemon is not running
- **THEN** the system returns a `NETBIRD_NOT_READY` error with hint `"NetBird daemon is not running. Run: netbird service start && netbird up"`

### Requirement: Validate target peer connectivity after target resolution
The system SHALL check, after resolving the target FQDN, that the corresponding NetBird peer is present and connected, using the peer list from the same `netbird status --json` output already parsed in the daemon check.

#### Scenario: Target peer is connected — check passes
- **WHEN** the resolved FQDN matches a peer in the NetBird peer list with status `"connected"`
- **THEN** the peer check passes and the connect flow proceeds to SSH

#### Scenario: Target peer is disconnected — check fails with PEER_NOT_CONNECTED
- **WHEN** the resolved FQDN matches a peer in the NetBird peer list but the peer status is not `"connected"`
- **THEN** the system returns a `PEER_NOT_CONNECTED` error with hint `"NetBird peer <fqdn> is <status>. Check peer connectivity in NetBird dashboard"`

#### Scenario: Target peer not found in peer list — check fails with PEER_NOT_FOUND
- **WHEN** the resolved FQDN does not match any peer in the NetBird peer list
- **THEN** the system returns a `PEER_NOT_FOUND` error with hint `"NetBird peer <fqdn> not found. Verify the host is enrolled in your NetBird network"`

#### Scenario: Peer check skipped when daemon check was skipped
- **WHEN** the daemon check was skipped (binary absent or `--skip-netbird-check`)
- **THEN** the peer check is also skipped

### Requirement: Allow bypassing the preflight check via flag
The system SHALL skip both the daemon check and the peer check when the `--skip-netbird-check` flag is provided to `k3ctx connect`.

#### Scenario: Flag skips both checks entirely
- **WHEN** the user runs `k3ctx connect --skip-netbird-check`
- **THEN** the system does not execute `netbird status` and proceeds directly through target resolution and SSH without any NetBird preflight

#### Scenario: Flag is absent — both checks run when binary is available
- **WHEN** the user runs `k3ctx connect` without `--skip-netbird-check` and the NetBird binary is on `$PATH`
- **THEN** the system executes the daemon check before target resolution and the peer check after target resolution

### Requirement: Use configured NetBird binary path for preflight
The system SHALL resolve the NetBird binary path for the preflight check using the same path resolution as peer-discovery: `netbird_bin_path` config key, `NETBIRD_BIN_PATH` env var, then `netbird` from `$PATH`.

#### Scenario: Custom binary path used for preflight
- **WHEN** `netbird_bin_path` or `NETBIRD_BIN_PATH` is set
- **THEN** the preflight check uses that path to invoke the NetBird binary

#### Scenario: Preflight command times out
- **WHEN** `netbird status --json` does not return within 3 seconds
- **THEN** the system treats the timeout as a failed check and returns a `NETBIRD_NOT_READY` error with hint `"NetBird status check timed out. Run: netbird status"` 
