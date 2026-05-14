# Repository Guidelines

## LLM Startup Instructions

At the start of each session:

- Read this file before proposing commands, edits, or architecture changes.
- **Active branch**: `k3ctx-go` — this is the Go rewrite. The Python source under `src/` is the migration reference; do not modify it.
- Treat `cmd/k3ctx/main.go` as the binary entrypoint and `cli/` as the cobra command layer.
- Prefer the discovery-first CLI flow: `init`, `clients`, `hosts`, `connect`, `status`, `k9s`.
- Assume local YAML data belongs under `~/.local/share/k3s-context-tunnel-manager/yaml/`, not in the repository root.
- Keep the default CLI non-interactive and automation-safe; preserve clean `--json` stdout contracts.
- Refresh inventory only when explicitly requested via `--refresh-inventory`.
- Before changing behavior, read the affected Go module and its `_test.go` file in the same package.
- When validating a real cluster flow, verify the tunnel, current context, and a real `kubectl` call instead of trusting setup messages alone.

## Project Overview

This project is a local K3s context tunnel manager for the `systemframe` workspace. It fetches kubeconfig files over SSH, opens local tunnels to K3s API servers, merges contexts into `~/.kube/config`, and supports local workflows with `kubectl`, `k9s`, and related tools.

When a cluster has ArgoCD configured in its inventory (`argocd_enabled: true`), `connect` also opens a parallel SSH tunnel to the ArgoCD NodePort and runs `argocd login` automatically.

## Migration Status

The project is being rewritten from Python to Go on branch `k3ctx-go` using TDD. The Python source under `src/` remains as the authoritative reference during migration — use it to understand expected behavior, then port tests first, then implement.

| Layer | Status | Go location |
|---|---|---|
| Domain models + network policies | Done | `internal/domain/` |
| Config + XDG paths | Done | `internal/config/`, `internal/paths/` |
| Tunnel PID lifecycle | Done | `internal/tunnel/` |
| Kubeconfig merge/update | Done | `internal/kubeconfig/` |
| Inventory YAML loading | Done | `internal/inventory/` |
| Network validator | Done | `internal/network/` |
| Application ports (interfaces) | Done | `internal/application/ports.go` |
| Use cases (all 6 modules) | Done | `internal/application/usecases/` |
| Infrastructure adapters (partial) | Done | `internal/infrastructure/` |
| Bootstrap / service container | Done | `internal/bootstrap/` |
| CLI (cobra, all 9 commands) | Done | `cli/`, `cmd/k3ctx/` |
| SSH cluster connector | **Pending** | `internal/infrastructure/cluster.go` |
| ArgoCD connector | **Pending** | `internal/infrastructure/argocd.go` |

## Go Project Structure

```
cmd/k3ctx/main.go         ← binary entrypoint; only calls cli.Execute()
cli/                      ← cobra commands (root, clients, hosts, connect, status, k9s, tunnel)
internal/
  domain/                 ← ClusterTarget, ConnectResult, EffectiveConfig, NetworkRequirement,
  │                          OperationError, ArgocdConfig, network policies, discovery models
  config/                 ← LoadConfig, LoadEffectiveConfig (YAML + env override)
  paths/                  ← XDG-compliant path resolution
  inventory/              ← vault-tag-safe YAML loading, ExtractHostsFromInventory
  tunnel/                 ← GetUniquePort, IsTunnelRunning, CreateTunnel, KillTunnel, SaveTunnelPID
  kubeconfig/             ← UpdateKubeconfigServer, MergeKubeconfig
  network/                ← GetNetworkMetadata, ValidateContextNetwork, CheckSshuttleActive
  application/
    ports.go              ← Go interfaces: ClusterConnector, InventoryCatalog, ContextSwitcher, etc.
    usecases/             ← connect, contexts, discovery, inventory, status, tunnels
  infrastructure/         ← YamlInventoryCatalog, KubectlContextSwitcher, LocalTunnelManager,
  │                          LocalStatusReader, GitInventoryRefresher
  bootstrap/              ← ServiceContainer, Build()
go.mod                    ← module github.com/systemframe/k3ctx
```

Python reference (do not modify):
```
src/                      ← original Python source; read for behavior reference only
tests/unit/               ← original Python tests; use as porting blueprint
```

## Stack

Go 1.25, `github.com/spf13/cobra`, `gopkg.in/yaml.v3`, `golang.org/x/crypto/ssh` (pending), `github.com/stretchr/testify`.

## Build, Test, and Development Commands

```bash
# Build
go build -o k3ctx ./cmd/k3ctx/

# Run all tests
go test ./...

# Run a specific package
go test ./internal/domain/ -v

# Vet
go vet ./...

# CLI (after build)
./k3ctx --help
./k3ctx init
./k3ctx clients
./k3ctx hosts <client>
./k3ctx connect [identifiers...]
./k3ctx status
./k3ctx tunnel-list
./k3ctx tunnel-kill <context>
./k3ctx tunnel-kill-all
./k3ctx k9s
```

Environment overrides (same as Python version):

| Env var | Purpose |
|---|---|
| `CONFIG_FILE` | Override config file path |
| `INVENTORY_PATH` | Override inventory directory |
| `K3S_API_PORT` | Override K3s API port |
| `SSH_KEY_PATH` | Override SSH key path |
| `PORT_RANGE_START` | Override port range start |
| `PORT_RANGE_SIZE` | Override port range size |
| `XDG_DATA_HOME` | Override XDG data home |

## Coding Style & Naming Conventions

- `MixedCaps` for exported identifiers; `mixedCaps` for unexported.
- Acronyms all-caps: `SSH`, `URL`, `ID`, `API`.
- Interfaces defined in `internal/application/ports.go`; sized to what callers need.
- `cmd/k3ctx/main.go` only calls `cli.Execute()` — no logic there.
- `cli/` handlers stay thin: load config, call use case, format output.
- Domain and use-case logic stays in `internal/`; nothing SSH- or subprocess-related goes into CLI handlers.
- Return `error` as the last value; never panic in normal flow.
- Use value types for domain structs (no pointer receivers on small structs).
- Packages: one clear responsibility; avoid `util`, `manager`, `helper` names.

**TDD**: Write the failing test first, then implement the minimum code to make it pass, then refactor. No production code without a corresponding test.

## Testing Guidelines

- Test files: `<file>_test.go` in the same package (e.g., `internal/domain/models_test.go`).
- Test functions: `TestXxx_DescribedBehavior(t *testing.T)`.
- Use `github.com/stretchr/testify/assert` and `require`.
- Define stub implementations inline in `_test.go` files (implement the port interface directly; no mock framework needed for simple cases).
- Infrastructure adapters that call subprocess or SSH: use injectable function parameters (e.g., `checkSshuttle func(string) bool`) for testability.
- `t.TempDir()` for filesystem isolation; `t.Setenv()` for env var isolation.
- Layer ordering for tests: domain → config/paths → primitives → use cases → infrastructure → CLI.

## Commit & Pull Request Guidelines

Use Conventional Commit style: `feat:`, `fix:`, `chore:`, `test:`, `refactor:`. PRs should include: purpose, behavior impact, test evidence (`go test ./...`, `go vet ./...`), and terminal excerpts when changing interactive flows.

## ArgoCD Inventory Keys

Add these to a host entry or group_vars to enable the ArgoCD integration:

```yaml
argocd_enabled: true          # required — activates tunnel + argocd login
argocd_namespace: argocd      # optional, default "argocd"
argocd_node_port: 30080       # required — NodePort of argocd-server
argocd_plaintext: true        # optional, default false — use --plaintext instead of --insecure
```

ArgoCD tunnel state is saved under `~/.local/state/k9s-tunnels/<context>-argocd.pid`. Kill it with:

```bash
./k3ctx tunnel-kill <context>-argocd
```

`argocd login` uses the `argocd-initial-admin-secret` Kubernetes secret in the configured namespace. If the secret is absent or `argocd` CLI is not in PATH, the K3s connection still succeeds; only the login step is skipped with a message.

## Security & Configuration Tips

Do not commit generated kubeconfigs, SSH keys, or local state files. Treat `~/.local/share/k3s-context-tunnel-manager/yaml/config/config.yaml` as machine-specific; verify `inventory_path`, `ssh_key_path`, and port range settings before testing against real clusters. The tracked config template lives in `examples/config/config.yaml`. Do not assume a local `./inventory`; this workspace commonly points `inventory_path` to an external Ansible inventory via `config.yaml` or `INVENTORY_PATH`. Contexts are merged into `~/.kube/config`, generated kubeconfig cache files live in `~/.local/share/k3s-context-tunnel-manager/yaml/kubeconfigs/`, and tunnel PID files live in `~/.local/state/k9s-tunnels`.
