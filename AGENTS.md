# Repository Instructions

## Source Of Truth
- This is a Go CLI repo; prefer `go.mod`, `Makefile`, and Go source over stale README Python/uv commands.
- Module: `github.com/systemframe/k3ctx`; CLI entrypoint: `cmd/k3ctx/main.go`.

## Commands
- Test all packages: `make test` or `go test ./...`.
- Focused test: `go test ./internal/<package> -run TestName`.
- Build: `make build` writes `bin/k3ctx`.
- Install: `make install` installs to `$(HOME)/.local/bin` unless `PREFIX` is set.
- Format Go edits: `gofmt -w <files>`.
- Lint: `make lint` (requires `golangci-lint`; install with `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`).

## Code Quality
- All Go files must be `gofmt`-clean before committing; run `gofmt -l ./...` to check.
- `go vet ./...` and `go test -race ./...` must pass.
- `make lint` runs `golangci-lint` with `.golangci.yml`; fix all reported issues.
- Use `domain.OperationError` as the single structured error type — do not introduce new error structs with the same fields.
- Wrap errors with `fmt.Errorf("context: %w", err)`; use `errors.As` for type-specific handling.

## TDD Workflow
- For any implementation, write or update the focused test first and run it to see it fail.
- Make the smallest code change that passes the focused test.
- Run the focused test again, then broaden to `go test ./...` when the change touches shared behavior.
- Do not skip tests for behavior changes unless the user explicitly asks; explain any untested gap.

## Architecture
- `cli/` contains Cobra commands.
- `internal/domain` contains core models and decisions.
- `internal/application/usecases` orchestrates behavior behind ports.
- `internal/infrastructure` contains local adapters for SSH, kubectl, kubeconfig, ArgoCD, inventory refresh, and tunnel state.
- Runtime wiring is in `internal/bootstrap/bootstrap.go`.

## Runtime Config And State
- `CONFIG_FILE` overrides config path.
- Default config path: `~/.local/share/k3ctx/yaml/config/config.yaml`.
- `K3CTX_CONFIG_DIR` overrides the config directory.
- Kubeconfig cache: `~/.local/share/k3ctx/yaml/kubeconfigs/`.
- Tunnel PID files: `~/.local/state/k3ctx-tunnels/`.

## Gotchas
- `connect` opens SSH tunnels and merges kubeconfig into the user kubeconfig.
- API readiness is enabled by default; disable with `K3CTX_VERIFY_API_READY=0` or tune with `K3CTX_API_READY_TIMEOUT_SECONDS`.
- Inventory refresh is explicit via `--refresh-inventory`; it runs `git pull --ff-only` only when the inventory repo is clean.
- Inventory files are Ansible YAML matching `*_hosts.yml`; unknown YAML tags like `!vault` are ignored.

## Release Workflow
- `k3ctx --version` must return JSON with `version`, `commit`, and `date` from `internal/version`.
- When a fix or feature is complete and the user approves, always: commit → tag next patch version (`v0.x.y+1`) → `git push origin main --tags` → `make install`.
- Determine next version with `git tag --sort=-version:refname | head -1` then increment the patch number.
- Never release without user approval.

## Safety
- Do not commit kubeconfigs, inventory, `.env`, keys, local state, or generated `bin/`.
- Ask before running commands that modify external state, especially real `connect`, tunnel kill commands, package installs, or git commits.
