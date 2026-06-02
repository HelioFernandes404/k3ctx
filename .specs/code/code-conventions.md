# Code Conventions

## Formatting

- All Go files must be `gofmt`-clean before committing. Run `gofmt -l ./...` to check (must print nothing).
- Imports are organized by `goimports` (configured via `.golangci.yml`).

## Static analysis

`make lint` runs `golangci-lint` with `.golangci.yml`. Enabled linters:

`govet`, `staticcheck`, `ineffassign`, `unused`, `errcheck`, `revive`, `gosec`, `misspell`, `unparam`, `nakedret`, `prealloc`, `bodyclose`, `noctx`, `gocritic`.

Mandatory checks before any commit:

```
gofmt -l ./...        # must print nothing
go vet ./...
go test -race ./...
make lint
```

Fix all linter reports — never silence with `//nolint:` without a comment explaining why.

## Errors

- `domain.OperationError` is the **single** structured error type. Do not introduce parallel error structs with the same fields.
- Wrap with `fmt.Errorf("context: %w", err)` when adding context.
- Use `errors.As` for type-specific handling — never `err.(*T)`.
- Public error codes (`OperationError.Code`) are part of the CLI contract. Don't rename without checking CLI tests and downstream consumers.

## TDD workflow

For any behavior change:

1. Write or update the focused test first. Run it to **see it fail** — that proves the test actually exercises the new behavior.
2. Make the smallest code change that turns the test green.
3. Re-run the focused test, then broaden to `go test ./...` when the change touches shared behavior.
4. If you must skip a test (regression that's intentional, flaky external dep), call it out explicitly in the response. Never silently delete a failing test.

Focused test: `go test ./internal/<package> -run TestName`

## Test organization

- **Internal tests** (`package <pkg>`) for testing unexported functions like `buildSSHArgs`, `filterRecords`, `buildSSHTunnelArgs`.
- **External tests** (`package <pkg>_test`) for testing the public API of usecases — forces you to consume the package the way callers do.
- Stubs/fakes live next to the tests that use them. CLI ports are mocked in `cli/mocks_test.go` and reassigned to `svcs.X` per test (with `t.Cleanup` to restore).
- Prefer table-driven tests for pure functions with many input combinations.
- Use `t.TempDir()` for filesystem fixtures and `t.Setenv()` for env overrides — both auto-clean.
- Test files for adapters live next to the adapter: `cluster.go` ⇆ `cluster_test.go`.

## File organization

- One adapter per file in `internal/infrastructure/` + a paired `_test.go`.
- When a file passes ~250 lines, split by responsibility (see `cluster.go` → `cluster_steps.go` + `cluster_apiready.go` for the pattern).
- Pure command-arg builders go into their own unexported function so subprocess wrappers can be tested without shelling out.

## Comments

- Default to writing no comment. Only add one when the *why* is non-obvious (hidden constraint, subtle invariant, workaround for a specific bug).
- Don't explain *what* the code does — well-named identifiers cover that.
- Don't reference current tasks, PRs, or issue numbers in comments — those rot.
- Public types and exported functions get a one-line doc comment in the standard Go style (`// FuncName does X.`).

## Branch and commit conventions

- Branch names: `<type>/<scope>-<summary>` (e.g. `feat/argocd-discovery`, `fix/tunnel-stale-pid`).
- Types: `feat`, `fix`, `docs`, `chore`, `refactor`, `test`.
- Summary is 2–5 lowercase words separated by hyphens.
- Commit messages follow the same `<type>(<scope>): <summary>` pattern; body explains *why*, not *what*.

## Don'ts

- Don't add backward-compat shims, `// removed` markers, renamed unused vars to `_X`, or re-exports just to ease a rename. If something is unused, delete it.
- Don't validate inside the system (between internal layers) — only at boundaries (CLI args, external APIs, subprocess output).
- Don't add error handling for impossible-by-construction states.
- Don't create new abstractions for hypothetical future requirements.
