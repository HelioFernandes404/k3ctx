# Repository Guidelines

## Project Overview

This project is a local K3s context tunnel manager for the `systemframe` workspace. It fetches kubeconfig files over SSH, opens local tunnels to K3s API servers, merges contexts into `~/.kube/config`, and supports local workflows with `kubectl`, `k9s`, and related tools.

## Project Structure & Module Organization

The codebase is organized in layers. Core domain models and pure policies live in `src/domain/`. Application orchestration lives in `src/application/use_cases/`. Infrastructure adapters live in `src/infrastructure/adapters/` and handle inventory access, connection, context switching, tunnel management, and status reads. User-facing entrypoints live in `src/interfaces/cli/`, `src/interfaces/http/`, and `src/interfaces/mcp/`, with compatibility wrappers such as [`src/mcp_server.py`](/home/helio/Obsidian/work/02-trabalho/systemframe/custom-tools/k3s-context-tunnel-manager/src/mcp_server.py) preserved at the repository root. Shell helpers such as `k9s-with-tunnel.sh` remain for local workflows. Tests stay split between `tests/unit/` and `tests/smoke/`. Local defaults live in `config.yaml`. Generated kubeconfig cache files are stored under `~/.cache/k9s-config/<context>.yml`.

## Stack

The main stack is Python 3, `uv`, `paramiko`, `PyYAML`, and Bash.

## Build, Test, and Development Commands

- `make init`: first-time setup for local development.
- `make sync`: install or sync dependencies with `uv`.
- `make run`: start the discovery-first `connect` flow.
- `make k9s`: launch `k9s` after tunnel validation.
- `make status`: show active cluster and tunnel state.
- `make tunnel-list`: list active SSH tunnels.
- `make tunnel-kill CONTEXT=<name>`: stop one tunnel by context.
- `make tunnel-kill-all`: stop all managed tunnels.
- `make mcp-stdio`: start the MCP server over stdio.
- `make mcp-http`: start the MCP server over HTTP on `127.0.0.1:8000` by default.
- `make test`: run the full test suite with verbose output.
- `uv run context-tunnel-manager clients`: list clients with host counts.
- `uv run context-tunnel-manager hosts <client>`: list or search hosts inside one client.
- `uv run context-tunnel-manager connect [identifiers...]`: resolve identifiers and connect if unique.
- `uv run context-tunnel-manager status`: show active contexts and tunnels.
- `uv run python -m pytest tests/unit -q`: fast unit test pass.
- `uv run python -m pytest tests/smoke -q`: smoke validation for user-facing entrypoints and docs.
- `uv run python -m mypy src tests`: run static type checks.

## Coding Style & Naming Conventions

Use Python 3.10+ compatible code and 4-space indentation. Keep transport concerns thin in `src/interfaces/*`; reusable behavior belongs in `src/application/use_cases/`, with side-effecting implementations in `src/infrastructure/adapters/`. Treat compatibility wrappers such as `src/mcp_server.py` as forwarding layers only; do not move SSH, tunnel, or kubeconfig business rules into CLI, HTTP, or MCP handlers. Follow existing naming patterns: `snake_case` for files, functions, variables, and test modules like `test_tunnel.py`. Keep shell scripts focused on orchestration. `mypy.ini` enables strict checks, so add or update type hints when changing behavior.

## Testing Guidelines

Use `pytest`. Place unit tests under `tests/unit/` and smoke coverage under `tests/smoke/`. Name files `test_*.py` and test functions `test_*`. Update tests together with behavior changes, especially around discovery queries, connect resolution, config parsing, SSH validation, tunnel cleanup, kubeconfig generation, and MCP/manual entrypoints. Interactive CLI flows require a real TTY and should fail with a clear error when run non-interactively, while `--json` flows must remain non-interactive-safe.

## Commit & Pull Request Guidelines

Local Git history is not available in this directory, so no verified project-specific commit pattern could be derived from `git log`. Use short imperative commit messages, preferably Conventional Commit style, for example `docs: add MCP entrypoints`. PRs should include: purpose, behavior impact, test evidence (`uv run python -m pytest tests/unit -q`, `uv run python -m pytest tests/smoke -q`, `uv run mypy src tests`), and terminal excerpts when changing interactive or MCP flows.

## Security & Configuration Tips

Do not commit generated kubeconfigs, SSH keys, or local state files. Treat `config.yaml` as machine-specific; verify `inventory_path`, `ssh_key_path`, and port range settings before testing against real clusters. Do not assume a local `./inventory`; this workspace commonly points `inventory_path` to an external Ansible inventory via `config.yaml` or `INVENTORY_PATH`. MCP tools may open SSH tunnels, change current kube context, and stop per-context tunnels, so prefer local loopback exposure for HTTP mode unless remote access is intentional. Contexts are merged into `~/.kube/config`, tunnel PID files live in `~/.local/state/k9s-tunnels`, and local logs live in `~/.local/state/k9s/`. Avoid removing `.venv` by default on local setups unless you are intentionally resetting the environment. See `README.md` for the discovery-first CLI flow and current validation examples.
