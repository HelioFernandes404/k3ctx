---
name: k3ctx
description: >
  Workflow guide for k3ctx, the CLI that discovers K3s cluster hosts from NetBird
  peers, opens SSH tunnels, merges kubeconfig contexts, and enables kubectl/k9s
  access. Use this skill when the user wants to connect to a cluster, manage
  tunnels, switch kubectl contexts, use k9s, list available clusters, check
  tunnel status, or debug kubectl access that might be missing a tunnel.
---

## Preflight check

Before running `connect`, verify the environment is ready:

```bash
bash skills/k3ctx/scripts/preflight.sh
```

The script checks: k3ctx binary, config file, NetBird daemon status, SSH key, and active tunnels.
Exit 0 = all clear. Exit 1 = one or more items need attention.

---

## Tool location

```
k3ctx
```

Use the installed `k3ctx` binary from `PATH`.

Typical local install path:

```bash
~/.local/bin/k3ctx
```

---

## Discovery-first flow

Always follow this order — never jump straight to `connect` without knowing the
context name:

```bash
# 1. List clients (derived from NetBird FQDN domain label)
k3ctx clients

# 2. List hosts in a client — shows [Connected]/[Connecting]/[Idle] status
k3ctx hosts <client>
k3ctx hosts <client> --host <filter> --limit 10

# 3. Connect (1–3 identifiers; fails deterministically if ambiguous)
k3ctx connect <client> <host>
k3ctx connect --context <context-name>
k3ctx connect --ip <netbird-ip>

# 4. Validate / launch k9s
k3ctx status
k3ctx k9s
```

`connect` opens an SSH tunnel to the peer FQDN, fetches the kubeconfig, and
merges it into `~/.kube/config`. It verifies the forwarded Kubernetes API is
reachable before reporting success — so a successful `connect` means `kubectl`
will work.

---

## Common commands

| Goal | Command |
|------|---------|
| List clients | `k3ctx clients` |
| Search hosts | `k3ctx hosts systemframe --host prd` |
| Connect by context | `k3ctx connect --context systemframe-sf-prd-us-00001` |
| Connect by host substring | `k3ctx connect systemframe prd-us-00001` |
| Check active tunnels | `k3ctx tunnel-list` |
| Check context/tunnel state | `k3ctx status` or `k3ctx --json status` |
| Launch k9s | `k3ctx k9s` |
| Kill one tunnel | `k3ctx tunnel-kill <context>` |
| Kill all tunnels | `k3ctx tunnel-kill-all` |
| Re-query NetBird peers | add `--refresh-inventory` to `clients` or `hosts` |
| Bypass NetBird preflight | `k3ctx connect --skip-netbird-check <target>` |

Structured output: pass the global `--json` flag, e.g. `k3ctx --json hosts systemframe`.

---

## Host discovery — NetBird

Hosts are discovered from the local NetBird peer list (`netbird status --json`).
No inventory files required. The operator machine must be connected to NetBird.

**FQDN convention:**

```
sf-prd-us-00001.systemframe.vpn
│              │ client = systemframe
│              └─ context name = systemframe-sf-prd-us-00001
└─ host alias = sf-prd-us-00001
```

**Default filter:** only peers matching `^sf-[a-z]{3}-(?:[a-z]{2}|us)-[0-9]{5}`
are shown (excludes personal devices and laptops).

**Status in output:** `[Connected]`, `[Connecting]`, or `[Idle]` — all matching
peers are listed regardless of connectivity; status is informational.

---

## Named environments

| Alias | Context name | NetBird FQDN |
|-------|-------------|--------------|
| `prod-primaria` | `systemframe-sf-prd-us-00001` | `sf-prd-us-00001.systemframe.vpn` |
| `thinkpad-medium` / test | `systemframe-sf-tst-sp-00001` | `sf-tst-sp-00001.systemframe.vpn` |

When user says "prod-primaria" → target `systemframe-sf-prd-us-00001`.
When user says "thinkpad" or "test machine" → target `systemframe-sf-tst-sp-00001`.

---

## Config and state paths

| What | Path |
|------|------|
| Config template | `examples/config/config.yaml` |
| Runtime config | `~/.local/share/k3ctx/yaml/config/config.yaml` |
| Kubeconfig cache | `~/.local/share/k3ctx/yaml/kubeconfigs/<ctx>.yml` |
| Merged kubeconfig | `~/.kube/config` |
| Tunnel PID files | `~/.local/state/k3ctx-tunnels/` |

**Config keys for NetBird:**

| Key | Env var | Default |
|-----|---------|---------|
| `netbird_bin_path` | `NETBIRD_BIN_PATH` | `netbird` (from `$PATH`) |
| `netbird_host_filter` | `NETBIRD_HOST_FILTER` | `^sf-[a-z]{3}-(?:[a-z]{2}\|us)-[0-9]{5}` |
| `inventory_path` | `INVENTORY_PATH` | _(empty — activates NetBird)_ |

`XDG_DATA_HOME` overrides the base data path.

Do not commit config.yaml, kubeconfigs, SSH keys, or state files.

---

## Troubleshooting

**`connect` reports "Kubernetes API did not become ready"**
```bash
K3CTX_API_READY_TIMEOUT_SECONDS=30 k3ctx connect --context <ctx>
# or disable the check and validate manually:
K3CTX_VERIFY_API_READY=0 k3ctx connect --context <ctx>
kubectl --request-timeout=10s get --raw=/version
```

**Host not appearing in `k3ctx hosts`**
1. Verify peer is in NetBird: `netbird status | grep <hostname>`
2. Check the FQDN follows `{hostAlias}.systemframe.vpn` convention
3. Verify hostname matches the filter regex (`sf-prd-*` / `sf-tst-*` pattern)
4. Re-query peers: `k3ctx hosts systemframe --refresh-inventory`

**`NETBIRD_NOT_READY` — daemon not authenticated or offline**
`k3ctx connect` checks NetBird status automatically before resolving any host.
If you see this error, the daemon is offline, not authenticated, or timed out.
```bash
netbird up              # authenticate and start daemon
# then retry connect
```
Use `--skip-netbird-check` to bypass both preflight checks (non-NetBird environments or scripting).

**`PEER_NOT_CONNECTED` — peer not reachable**
The target host is registered in NetBird but its status is not `Connected`.
`connect` fails before SSH — it does not attempt a connection to an unreachable peer.
Check the remote machine's NetBird daemon, then retry.

**`PEER_NOT_FOUND` — target FQDN absent from peer list**
The resolved FQDN does not appear in the NetBird peer list at all.
Run `k3ctx hosts <client> --refresh-inventory` and verify the host exists.

**VPN / sshuttle required**
If the cluster requires VPN or `sshuttle`, the CLI returns a structured error
with remediation steps — no interactive prompt.

---

## What NOT to do

- Don't call `kubectl` or `k9s` directly without first verifying a tunnel is active (`status`).
- Don't run `connect` without at least one identifier — it fails deterministically.
- Don't hardcode IPs — use FQDN-based discovery via `clients`/`hosts`.
- Don't skip `--refresh-inventory` when NetBird peer state may have changed.
- Don't edit `~/.kube/config` manually — `connect` manages the merge.
- Don't set `inventory_path` unless using the legacy YAML catalog.
- Don't troubleshoot `NETBIRD_NOT_READY` with SSH — run `netbird up` first, then retry.
