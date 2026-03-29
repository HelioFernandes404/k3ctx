# Repository Guidelines

## Project Structure & Module Organization

Core Python modules live in `src/` and cover config loading, inventory parsing, SSH, tunnel lifecycle, kubeconfig merging, and CLI flow. Entry scripts at the repository root handle the main workflows: `fetch_k3s_config.py` for single-cluster setup, `multi_connect.py` for multi-cluster setup, and shell helpers such as `k9s-with-tunnel.sh` and `init.sh`. Tests are split into `tests/unit/` for isolated behavior and `tests/smoke/` for end-to-end validation. Local defaults live in `config.yaml`.

## Build, Test, and Development Commands

- `make init`: first-time setup for local development.
- `make sync`: install or sync dependencies with `uv`.
- `make run`: start the interactive single-cluster flow.
- `make multi-connect`: connect to multiple clusters in one session.
- `make k9s`: launch `k9s` after tunnel validation.
- `make status`: show active cluster and tunnel state.
- `make test`: run the full test suite with verbose output.
- `uv run pytest tests/unit -q`: fast unit test pass.
- `uv run mypy src tests`: run static type checks.

## Coding Style & Naming Conventions

Use Python 3.10+ compatible code and 4-space indentation. Prefer small typed functions and keep cluster-specific logic inside `src/` modules instead of root scripts. Follow existing naming patterns: `snake_case` for files, functions, variables, and test modules like `test_tunnel.py`. Keep shell scripts focused on orchestration; keep business logic in Python. `mypy.ini` enables strict checks, so add or update type hints when changing behavior.

## Testing Guidelines

Use `pytest`. Place unit tests under `tests/unit/` and smoke coverage under `tests/smoke/`. Name files `test_*.py` and test functions `test_*`. Update tests together with behavior changes, especially around config parsing, SSH validation, tunnel cleanup, and kubeconfig generation.

## Commit & Pull Request Guidelines

Local Git history is not available in this directory, so no verified project-specific commit pattern could be derived from `git log`. Use short imperative commit messages, preferably Conventional Commit style, for example `fix: handle missing TTY in run flow`. PRs should include: purpose, behavior impact, test evidence (`make test`, `uv run mypy src tests`), and screenshots or terminal excerpts when changing interactive flows.

## Security & Configuration Tips

Do not commit generated kubeconfigs, SSH keys, or local state files. Treat `config.yaml` as machine-specific; verify `inventory_path`, `ssh_key_path`, and port range settings before testing against real clusters.
