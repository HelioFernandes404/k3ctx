# K3s Context Tunnel Manager

Go CLI for managing SSH tunnels and kubeconfig contexts for K3s clusters.

It discovers hosts from an Ansible inventory, opens a local SSH tunnel to the K3s API, merges the generated kubeconfig into `~/.kube/config`, and switches the active kubectl context.

## Local Path

```bash
cd /home/helio/Obsidian/03-projetos/k3ctx
```

## Project Structure

```text
.
├── cmd/k3ctx/                  # CLI entrypoint
├── cli/                        # Cobra commands
├── internal/application/        # Use cases and ports
├── internal/domain/             # Core models and decisions
├── internal/infrastructure/     # Local adapters
├── internal/config/             # Config loading and env overlays
├── internal/paths/              # XDG/local path resolution
├── internal/inventory/          # Ansible inventory parsing
├── internal/kubeconfig/         # Kubeconfig helpers
├── internal/network/            # Local port selection
├── internal/ssh/                # SSH command helpers
├── internal/tunnel/             # Tunnel process state
└── examples/config/config.yaml  # Example config
```

## Build And Install

```bash
make build
make install
```

`make build` writes `bin/k3ctx`.

`make install` installs to `$(HOME)/.local/bin/k3ctx` by default. Override with `PREFIX=/some/path make install`.

The Makefile uses Go `1.25.0`. If `mise` is available, it runs Go through `mise`; otherwise it uses `go` from `PATH`.

## Main Flow

```bash
k3ctx init
k3ctx clients
k3ctx hosts acme
k3ctx connect acme
k3ctx tunnel-list
k3ctx status
```

From the repo without installing:

```bash
go run ./cmd/k3ctx --help
go run ./cmd/k3ctx clients
```

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

Global flag:

```bash
k3ctx --json <command>
```

## Discovery Examples

```bash
k3ctx clients --limit 20
k3ctx clients --cursor <cursor>
k3ctx clients --refresh-inventory

k3ctx hosts acme
k3ctx hosts acme api
k3ctx hosts acme --host api --limit 10
k3ctx hosts acme --id sf-1042
k3ctx hosts acme --ip 10.0.0.10
k3ctx hosts acme --refresh-inventory
```

Rules:

- `clients` returns client names and host counts.
- `hosts CLIENT` requires a client scope.
- Pagination uses `--limit` and `--cursor`.
- Inventory refresh is explicit with `--refresh-inventory`.

## Connect Examples

```bash
k3ctx connect acme
k3ctx connect --client acme --host prod
k3ctx connect --ip 10.0.0.10
k3ctx connect --context acme-prod
k3ctx connect --id sf-1042 --json
```

Rules:

- `connect` succeeds only when the resolver finds one unique host.
- Ambiguous matches exit with an error and list matching contexts.
- No-match results exit with an error and a hint when available.
- On success, the generated kubeconfig is merged into `~/.kube/config`.
- The active kubectl context is switched to the connected context.

## Config

`k3ctx init` creates:

```text
~/.local/share/k3ctx/yaml/
├── config/config.yaml
└── kubeconfigs/
```

Config lookup order:

1. `CONFIG_FILE`, when set
2. `K3CTX_CONFIG_DIR/config.yaml`, when `K3CTX_CONFIG_DIR` is set
3. `~/.local/share/k3ctx/yaml/config/config.yaml`
4. `config.yaml` in the current project directory
5. `~/.k3ctx-config/config.yaml`

`XDG_DATA_HOME` changes the base for `~/.local/share` paths.

Example:

```yaml
inventory_path: /home/helio/Work/systemframe/ansible/inventory
ssh_key_path: ~/.ssh/id_ed25519
remote_k3s_config_path: /etc/rancher/k3s/k3s.yaml
k3s_api_port: 6443
port_range_start: 16443
port_range_size: 10000
```

Supported environment overlays:

| Config key | Environment variable |
| --- | --- |
| `inventory_path` | `INVENTORY_PATH` |
| `ssh_key_path` | `SSH_KEY_PATH` |
| `remote_k3s_config_path` | `REMOTE_K3S_CONFIG_PATH` |
| `k3s_api_port` | `K3S_API_PORT` |
| `port_range_start` | `PORT_RANGE_START` |
| `port_range_size` | `PORT_RANGE_SIZE` |

`ssh_config_path` is currently fixed to `~/.ssh/config`.

## Inventory

The inventory parser reads Ansible YAML files matching `*_hosts.yml`.

Unknown YAML tags, including tags like `!vault`, are ignored.

Expected host fields include:

```yaml
MY-HOST:
  ansible_host: 1.2.3.4
  systemframe_id: sf-1042
```

## ArgoCD

During `connect`, k3ctx automatically looks for an ArgoCD NodePort Service in the connected Kubernetes context.

When found, `connect` opens a managed tunnel for the discovered ArgoCD NodePort and attempts `argocd login` using the initial admin secret from the discovered namespace. No inventory fields are required.

If the `argocd` CLI is missing or login fails, the cluster connection can still succeed. Use the reported local ArgoCD port for manual login.

To kill only the ArgoCD tunnel:

```bash
k3ctx tunnel-kill <context>-argocd
```

## Runtime State

| Path | Purpose |
| --- | --- |
| `~/.local/share/k3ctx/yaml/config/config.yaml` | Default config file. |
| `~/.local/share/k3ctx/yaml/kubeconfigs/` | Generated kubeconfig cache. |
| `~/.local/state/k3ctx-tunnels/` | Managed tunnel PID files. |
| `~/.kube/config` | User kubeconfig updated by `connect`. |

## Development

```bash
make test
go test ./...
go test ./internal/application/usecases -run TestName
gofmt -w <files>
```

Useful Make targets:

| Target | Action |
| --- | --- |
| `make test` | Run `go test ./...`. |
| `make build` | Build `bin/k3ctx`. |
| `make install` | Install the binary. |
| `make clean` | Remove `bin/`. |

## Safety

- `connect` opens SSH tunnels and modifies `~/.kube/config`.
- `tunnel-kill` and `tunnel-kill-all` terminate managed tunnel processes.
- Do not commit generated kubeconfigs, local config, SSH keys, inventory secrets, `.env`, or `bin/`.
