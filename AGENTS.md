# Repository Guidelines

## Project Structure & Module Organization

The codebase is organized in layers. Core typed models live in `src/models.py`. Shared business logic lives in `src/services/` and handles inventory discovery, cluster connection, context switching, status inspection, and network validation. Infrastructure helpers in `src/` support config loading, SSH, tunnel lifecycle, kubeconfig merging, and logging. Manual entrypoints remain at the repository root in `fetch_k3s_config.py`, `multi_connect.py`, and shell helpers such as `k9s-with-tunnel.sh`. The MCP interface is isolated in [`src/mcp_server.py`](/home/helio/Obsidian/work/02-trabalho/systemframe/custom-tools/k3s-context-tunnel-manager/src/mcp_server.py). Tests stay split between `tests/unit/` and `tests/smoke/`. Local defaults live in `config.yaml`.

## Build, Test, and Development Commands

- `make init`: first-time setup for local development.
- `make sync`: install or sync dependencies with `uv`.
- `make run`: start the interactive single-cluster flow.
- `make multi-connect`: connect to multiple clusters in one session.
- `make k9s`: launch `k9s` after tunnel validation.
- `make status`: show active cluster and tunnel state.
- `make mcp-stdio`: start the MCP server over stdio.
- `make mcp-http`: start the MCP server over HTTP on `127.0.0.1:8000` by default.
- `make test`: run the full test suite with verbose output.
- `uv run python -m pytest tests/unit -q`: fast unit test pass.
- `uv run python -m pytest tests/smoke -q`: smoke validation for user-facing entrypoints and docs.
- `uv run mypy src tests`: run static type checks.

## Coding Style & Naming Conventions

Use Python 3.10+ compatible code and 4-space indentation. Keep interactive prompts inside manual CLI files and keep reusable logic in `src/services/`. Treat `src/mcp_server.py` as a thin transport layer only; do not move SSH, tunnel, or kubeconfig business rules into FastMCP handlers. Follow existing naming patterns: `snake_case` for files, functions, variables, and test modules like `test_tunnel.py`. Keep shell scripts focused on orchestration. `mypy.ini` enables strict checks, so add or update type hints when changing behavior.

## Testing Guidelines

Use `pytest`. Place unit tests under `tests/unit/` and smoke coverage under `tests/smoke/`. Name files `test_*.py` and test functions `test_*`. Update tests together with behavior changes, especially around config parsing, SSH validation, tunnel cleanup, kubeconfig generation, and MCP/manual entrypoints.

## Commit & Pull Request Guidelines

Local Git history is not available in this directory, so no verified project-specific commit pattern could be derived from `git log`. Use short imperative commit messages, preferably Conventional Commit style, for example `docs: add MCP entrypoints`. PRs should include: purpose, behavior impact, test evidence (`uv run python -m pytest tests/unit -q`, `uv run python -m pytest tests/smoke -q`, `uv run mypy src tests`), and terminal excerpts when changing interactive or MCP flows.

## Security & Configuration Tips

Do not commit generated kubeconfigs, SSH keys, or local state files. Treat `config.yaml` as machine-specific; verify `inventory_path`, `ssh_key_path`, and port range settings before testing against real clusters. MCP tools may open SSH tunnels, change current kube context, and stop per-context tunnels, so prefer local loopback exposure for HTTP mode unless remote access is intentional.
