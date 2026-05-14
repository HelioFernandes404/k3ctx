# Repository Guidelines

## LLM Startup Instructions

At the start of each session:

- Read this file before proposing commands, edits, or architecture changes.
- Treat `src/interfaces/cli/app.py` as the official entrypoint.
- Do not reintroduce repository-root helper scripts or interactive prompt flows.
- Prefer the discovery-first CLI flow: `init`, `clients`, `hosts`, `connect`, `status`, `k9s`.
- Assume local YAML data belongs under `~/.local/share/k3s-context-tunnel-manager/yaml/`, not in the repository root.
- Keep the default CLI non-interactive and automation-safe; preserve clean `--json` stdout contracts.
- Refresh inventory only when explicitly requested via `--refresh-inventory` or `K9S_REFRESH_INVENTORY=1`.
- Before changing behavior, read the affected module and its current tests in `tests/unit/` and `tests/smoke/`.
- When validating a real cluster flow, verify the tunnel, current context, and a real `kubectl` call instead of trusting setup messages alone.

## Project Overview

This project is a local K3s context tunnel manager for the `systemframe` workspace. It fetches kubeconfig files over SSH, opens local tunnels to K3s API servers, merges contexts into `~/.kube/config`, and supports local workflows with `kubectl`, `k9s`, and related tools.

## Project Structure & Module Organization

The codebase is organized in layers. Core domain models and pure policies live in `src/domain/`. Application orchestration lives in `src/application/use_cases/`. Infrastructure adapters live in `src/infrastructure/adapters/` and handle inventory access, connection, context switching, tunnel management, and status reads. The user-facing entrypoint is `src/interfaces/cli/`. Repository-root helper scripts have been removed in favor of the CLI entrypoint under `src`. Tests stay split between `tests/unit/` and `tests/smoke/`. Local YAML data now lives under `~/.local/share/k3s-context-tunnel-manager/yaml/`, with config in `config/config.yaml` and generated kubeconfig cache files in `kubeconfigs/<context>.yml`. `XDG_DATA_HOME` overrides the base location.

## Stack

The main stack is Python 3, `uv`, `paramiko`, `PyYAML`, and Bash.

## Build, Test, and Development Commands

- `make init`: first-time setup for local development plus legacy YAML migration.
- `make sync`: install or sync dependencies with `uv`.
- `make run`: start the discovery-first `connect` flow.
- `make k9s`: launch `k9s` after tunnel validation.
- `make status`: show active cluster and tunnel state.
- `make tunnel-list`: list active SSH tunnels.
- `make tunnel-kill CONTEXT=<name>`: stop one tunnel by context.
- `make tunnel-kill-all`: stop all managed tunnels.
- `make test`: run the full test suite with verbose output.
- `uv run context-tunnel-manager init`: prepare local config, YAML storage, and log directories.
- `uv run context-tunnel-manager clients`: list clients with host counts.
- `uv run context-tunnel-manager hosts <client>`: list or search hosts inside one client.
- `uv run context-tunnel-manager clients --refresh-inventory`: explicitly refresh inventory before listing.
- `uv run context-tunnel-manager hosts <client> --refresh-inventory`: explicitly refresh inventory before search.
- `uv run context-tunnel-manager connect [identifiers...]`: resolve identifiers, verify the forwarded Kubernetes API, and connect if unique.
- `uv run context-tunnel-manager k9s`: validate current tunnel and launch `k9s`.
- `uv run context-tunnel-manager tunnel-list`: list active SSH tunnels.
- `uv run context-tunnel-manager tunnel-kill <context>`: stop one managed tunnel.
- `uv run context-tunnel-manager tunnel-kill-all`: stop all managed tunnels.
- `uv run context-tunnel-manager status`: show active contexts and tunnels.
- `uv run python -m pytest tests/unit -q`: fast unit test pass.
- `uv run python -m pytest tests/smoke -q`: smoke validation for user-facing entrypoints and docs.
- `uv run python -m mypy src tests`: run static type checks.

## Coding Style & Naming Conventions

Use Python 3.10+ compatible code and 4-space indentation. Keep transport concerns thin in `src/interfaces/cli/`; reusable behavior belongs in `src/application/use_cases/`, with side-effecting implementations in `src/infrastructure/adapters/`. Do not move SSH, tunnel, or kubeconfig business rules into CLI handlers. Follow existing naming patterns: `snake_case` for files, functions, variables, and test modules like `test_tunnel.py`. Keep shell scripts focused on orchestration. `mypy.ini` enables strict checks, so add or update type hints when changing behavior.

**TDD**: Write the failing test first, then implement the minimum code to make it pass, then refactor. No production code without a corresponding test.

## Testing Guidelines

Use `pytest`. Place unit tests under `tests/unit/` and smoke coverage under `tests/smoke/`. Name files `test_*.py` and test functions `test_*`. Update tests together with behavior changes, especially around discovery queries, connect resolution, config parsing, SSH validation, tunnel cleanup, and kubeconfig generation. Interactive CLI flows require a real TTY and should fail with a clear error when run non-interactively, while `--json` flows must remain non-interactive-safe.

When writing or reviewing tests, use the `/pytest-quality` skill. It provides isolation patterns per layer, stub/mock conventions, naming rules, and a quality checklist.

## Commit & Pull Request Guidelines

Local Git history is not available in this directory, so no verified project-specific commit pattern could be derived from `git log`. Use short imperative commit messages, preferably Conventional Commit style, for example `feat: add tunnel-kill-all command`. PRs should include: purpose, behavior impact, test evidence (`uv run python -m pytest tests/unit -q`, `uv run python -m pytest tests/smoke -q`, `uv run mypy src tests`), and terminal excerpts when changing interactive flows.

## Security & Configuration Tips

Do not commit generated kubeconfigs, SSH keys, or local state files. Treat `~/.local/share/k3s-context-tunnel-manager/yaml/config/config.yaml` as machine-specific; verify `inventory_path`, `ssh_key_path`, and port range settings before testing against real clusters. The tracked config template now lives in `examples/config/config.yaml`. Do not assume a local `./inventory`; this workspace commonly points `inventory_path` to an external Ansible inventory via `config.yaml` or `INVENTORY_PATH`. CLI inventory refresh is explicit via `--refresh-inventory` or `K9S_REFRESH_INVENTORY=1`, default CLI logging is human-readable INFO unless `K9S_LOG_LEVEL=DEBUG` or `K9S_LOG_FORMAT=json` is enabled, and the post-tunnel API readiness check can be tuned with `K9S_API_READY_TIMEOUT_SECONDS` or disabled with `K9S_VERIFY_API_READY=0`. Contexts are merged into `~/.kube/config`, generated kubeconfig cache files live in `~/.local/share/k3s-context-tunnel-manager/yaml/kubeconfigs/`, tunnel PID files live in `~/.local/state/k9s-tunnels`, and local logs live in `~/.local/state/k9s/`. Avoid removing `.venv` by default on local setups unless you are intentionally resetting the environment. See `README.md` for the discovery-first CLI flow and current validation examples.
