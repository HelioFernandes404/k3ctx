# k3ctx

Go CLI for connecting to K3s clusters: discovers hosts via NetBird (or a legacy Ansible inventory), opens a managed SSH tunnel to the K3s API, merges the generated kubeconfig into `~/.kube/config`, and switches the active kubectl context.

## Install

```bash
make build      # writes bin/k3ctx
make install    # installs to $(HOME)/.local/bin (override with PREFIX=...)
```

The Makefile uses Go `1.25.0`. If `mise` is installed, builds run through `mise`; otherwise `go` from `PATH`.

## Main flow

```bash
k3ctx init
k3ctx clients
k3ctx hosts acme
k3ctx connect acme
k3ctx status
```

Append `--json` to any command for machine-readable output.

## Commands

| Command | Purpose |
| --- | --- |
| `k3ctx init` | Create local config and kubeconfig cache directories. |
| `k3ctx clients [query]` | List clients with host counts. |
| `k3ctx hosts CLIENT [query]` | List or search hosts inside one client. |
| `k3ctx connect [IDENTIFIER]` | Resolve one host and connect to it. |
| `k3ctx tunnel-list` | List managed tunnels that are running. |
| `k3ctx tunnel-kill CONTEXT` | Kill one managed tunnel. |
| `k3ctx tunnel-kill-all` | Kill all managed tunnels. |
| `k3ctx status` | Show active contexts and tunnel state. |
| `k3ctx exec CTX -- CMD...` | Run a command against a connected context. |
| `k3ctx --version` | Print version metadata as JSON. |

## Discovery

```bash
k3ctx clients --limit 20 --cursor <cursor>
k3ctx clients --refresh-inventory

k3ctx hosts acme
k3ctx hosts acme --host api --id sf-1042 --ip 10.0.0.10
```

- `clients` returns client names and host counts.
- `hosts CLIENT` requires a client scope; `--limit`/`--cursor` paginate.
- Inventory refresh is explicit with `--refresh-inventory` (NetBird repoll, or `git pull --ff-only` for the YAML catalog when clean).

## Connect

```bash
k3ctx connect acme
k3ctx connect --client acme --host prod
k3ctx connect --ip 10.0.0.10
k3ctx connect --context acme-prod
k3ctx connect --id sf-1042 --json
```

- `connect` succeeds only when the resolver finds one unique host.
- Ambiguous matches exit with an error and list the candidates.
- On success: kubeconfig is merged into `~/.kube/config` and the active context is switched.
- API readiness is verified by default. Disable with `K3CTX_VERIFY_API_READY=0`; tune with `K3CTX_API_READY_TIMEOUT_SECONDS`.

## Config

`k3ctx init` creates `~/.local/share/k3ctx/yaml/config/config.yaml`.

Lookup order: `CONFIG_FILE` → `$K3CTX_CONFIG_DIR/config.yaml` → `~/.local/share/k3ctx/yaml/config/config.yaml` → `./config.yaml` → `~/.k3ctx-config/config.yaml`. `XDG_DATA_HOME` rebases `~/.local/share`.

Example (NetBird discovery — default):

```yaml
ssh_key_path: ~/.ssh/id_ed25519
remote_k3s_config_path: /etc/rancher/k3s/k3s.yaml
k3s_api_port: 6443
port_range_start: 16443
port_range_size: 10000
# netbird_bin_path: netbird
# netbird_host_filter: ^sf-[a-z]{3}-(?:[a-z]{2}|us)-[0-9]{5}
```

Example (legacy YAML inventory):

```yaml
inventory_path: /path/to/ansible/inventory
ssh_key_path: ~/.ssh/id_ed25519
remote_k3s_config_path: /etc/rancher/k3s/k3s.yaml
```

Environment overlay: any config key can be overridden via its uppercase env-var counterpart (`SSH_KEY_PATH`, `NETBIRD_HOST_FILTER`, `INVENTORY_PATH`, `K3S_API_PORT`, `PORT_RANGE_START`, `PORT_RANGE_SIZE`, etc.).

## Host discovery

### NetBird (default)

When `inventory_path` is unset, k3ctx reads peers from `netbird status --json`.

Peers must follow `{hostAlias}.{client}.{netbird-domain}`:

```
sf-prd-us-00001.systemframe.vpn
│              │ client = systemframe
│              └─ context name = systemframe-sf-prd-us-00001
└─ host alias = sf-prd-us-00001
```

Only peers matching `netbird_host_filter` are included (default regex excludes personal devices and laptops). All matching peers are listed regardless of NetBird connectivity status — `hosts` shows the current status.

Requires the `netbird` daemon installed, authenticated, and running.

### YAML inventory (legacy)

When `inventory_path` is set and exists, k3ctx reads Ansible YAML files matching `*_hosts.yml`. Unknown YAML tags (including `!vault`) are ignored.

```yaml
MY-HOST:
  ansible_host: 1.2.3.4
  systemframe_id: sf-1042
```

To migrate to NetBird: remove `inventory_path`, register hosts in NetBird with the FQDN convention above. To roll back: restore `inventory_path`.

## ArgoCD

During `connect`, k3ctx looks for an ArgoCD NodePort Service in the connected context. When found, it opens a managed tunnel for that NodePort and attempts `argocd login` using the initial admin secret. Cluster connection succeeds even if `argocd` is missing or login fails.

Kill only the ArgoCD tunnel:

```bash
k3ctx tunnel-kill <context>-argocd
```

## Runtime state

| Path | Purpose |
| --- | --- |
| `~/.local/share/k3ctx/yaml/config/config.yaml` | Default config file. |
| `~/.local/share/k3ctx/yaml/kubeconfigs/` | Generated kubeconfig cache. |
| `~/.local/state/k3ctx-tunnels/` | Managed tunnel PID files. |
| `~/.kube/config` | User kubeconfig updated by `connect`. |

## Safety

- `connect` opens SSH tunnels and modifies `~/.kube/config`.
- `tunnel-kill` / `tunnel-kill-all` terminate managed tunnel processes.
- Never commit generated kubeconfigs, local config, SSH keys, inventory secrets, `.env`, or `bin/`.

## Contributing

Architecture, code conventions, and the "adding a feature" checklist live under [`.specs/`](.specs/):

- [`.specs/architecture/architecture.md`](.specs/architecture/architecture.md) — hexagonal layers and dependency rule.
- [`.specs/code/code-conventions.md`](.specs/code/code-conventions.md) — formatting, lint, TDD, test layout.

Read both before opening a PR.
