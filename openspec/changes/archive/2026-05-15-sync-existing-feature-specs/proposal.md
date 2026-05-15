## Why

The OpenSpec baseline only documents ArgoCD auto-discovery, while the codebase already contains several shipped CLI capabilities. This change brings `openspec/specs/` back in sync with existing behavior so future changes have an accurate source of truth.

## What Changes

- Add baseline specifications for local configuration and storage paths.
- Add baseline specifications for Ansible inventory discovery, client listing, host search, pagination, and host resolution.
- Add baseline specifications for cluster connection behavior, including SSH resolution, tunnel creation, kubeconfig caching/merge, API readiness, network requirements, and context switching.
- Add baseline specifications for managed tunnel status, listing, and termination.
- Add baseline specifications for launching `k9s` from the CLI.
- Preserve the existing `argocd-auto-discovery` capability without changing its requirements.
- No product behavior changes are intended; this is a spec synchronization change.

## Capabilities

### New Capabilities
- `local-configuration`: CLI initialization, config file discovery, environment overrides, XDG storage paths, and local cache directories.
- `inventory-discovery`: Ansible YAML inventory loading, client summaries, host search, filters, pagination, and host resolution.
- `cluster-connection`: Resolving a target into an SSH-backed K3s connection, opening tunnels, preparing kubeconfig, checking API readiness, and switching contexts.
- `tunnel-management`: Listing active managed tunnels, reporting context status, and killing one or all managed tunnels.
- `k9s-launch`: Launching the external `k9s` binary through the CLI.

### Modified Capabilities
- None.

## Impact

- Affected OpenSpec artifacts: `openspec/specs/` and this change directory.
- Referenced implementation areas: `cli/`, `internal/config`, `internal/paths`, `internal/inventory`, `internal/application/usecases`, `internal/domain`, and `internal/infrastructure`.
- No new runtime dependencies, CLI flags, commands, or behavior changes.
