---
name: k3ctx
description: >
  Workflow guide for k3ctx, the CLI that opens SSH tunnels, merges kubeconfig
  contexts, and enables kubectl/k9s access to k3s clusters. Use this skill when
  the user wants to connect to a cluster, manage tunnels, switch kubectl contexts,
  use k9s, list available clusters, check tunnel status, or debug kubectl access
  that might be missing a tunnel.
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

Prefer the `k3ctx` executable resolved from `PATH`. Do not hardcode a user-specific
absolute path unless the user asks you to diagnose that exact installation.

---

## Discovery-first flow

Always follow this order — never jump straight to `connect` without knowing the
context name:

```bash
# 1. List clients (high-level groups)
k3ctx clients

# 2. List hosts in a client
k3ctx hosts <client>
k3ctx hosts <client> --host <filter> --limit 10

# 3. Connect (1–3 identifiers; fails deterministically if ambiguous)
k3ctx connect <client> <env>
k3ctx connect --context <context-name>
k3ctx connect --ip 10.0.0.10
k3ctx connect --id sf-1042

# 4. Validate / launch k9s
k3ctx status
k3ctx k9s
```

`connect` opens an SSH tunnel, fetches the kubeconfig, and merges it into
`~/.kube/config`. It also verifies the forwarded Kubernetes API is reachable
before reporting success — so a successful `connect` means `kubectl` will work.

---

## Common commands

| Goal | Command |
|------|---------|
| List clients | `k3ctx clients` |
| Search hosts | `k3ctx hosts acme --host api` |
| Connect by name | `k3ctx connect --context acme-prod` |
| Check active tunnels | `k3ctx tunnel-list` |
| Check context/tunnel state | `k3ctx status` or `k3ctx --json status` |
| Launch k9s | `k3ctx k9s` |
| Kill one tunnel | `k3ctx tunnel-kill <context>` |
| Kill all tunnels | `k3ctx tunnel-kill-all` |
| Refresh inventory | add `--refresh-inventory` to `clients` or `hosts` |

Structured output: pass the global `--json` flag, for example `k3ctx --json status`.

---

## Named environments

| Alias | Helm-values path | SSH alias |
|-------|-----------------|-----------|
| `prod-primaria` | `helm-values/systemframe/sf-prd-us-00001` | — |
| `thinkpad-medium` (test) | `helm-values/systemframe/sf-tst-sp-00001` | `ssh thinkpad-dev-medium` |

When user says "prod-primaria" → target `sf-prd-us-00001`.
When user says "thinkpad" or "test machine" → target `sf-tst-sp-00001`.

## Config and state paths

| What | Path |
|------|------|
| Config template | `examples/config/config.yaml` |
| Runtime config | `~/.local/share/k3ctx/yaml/config/config.yaml` |
| Kubeconfig cache | `~/.local/share/k3ctx/yaml/kubeconfigs/<ctx>.yml` |
| Merged kubeconfig | `~/.kube/config` |
| Tunnel PID files | `~/.local/state/k3ctx-tunnels/` |
| Inventory (Ansible) | value of `inventory_path` in config.yaml |

`XDG_DATA_HOME` overrides the base data path.

Do not commit config.yaml, kubeconfigs, SSH keys, or state files.

---

## Troubleshooting

**`connect` reports "Kubernetes API did not become ready"**
The tunnel opened but the API health check timed out. Try:
```bash
K3CTX_API_READY_TIMEOUT_SECONDS=30 k3ctx connect --context <ctx>
# or disable the check temporarily and validate manually:
K3CTX_VERIFY_API_READY=0 k3ctx connect --context <ctx>
kubectl --request-timeout=10s get --raw=/version
```

**Inventory outdated**
```bash
k3ctx clients --refresh-inventory
```

**VPN / sshuttle required**
If the cluster requires VPN or `sshuttle`, the CLI returns a structured error with
remediation steps — no interactive prompt.

---

## What NOT to do

- Don't call `kubectl` or `k9s` directly without first verifying a tunnel is active (`status`).
- Don't run `connect` without at least one identifier — it fails deterministically.
- Don't hardcode IPs or ports — derive them from inventory via `clients`/`hosts`.
- Don't skip `--refresh-inventory` when inventory changes were expected.
- Don't edit `~/.kube/config` manually — `connect` manages the merge.
