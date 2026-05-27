## Context

`k3ctx connect` already opens an ArgoCD SSH tunnel and attempts `argocd login` automatically (via `LocalArgocdConnector.Setup`). The result is captured in `ArgocdLoginResult{Success, LocalPort, Skipped, Message}`, but the CLI text output (`cli/connect.go`) only prints the URL — it never surfaces `Message`. Agents therefore have no feedback when login fails silently.

Additionally, `argocd login --insecure` still negotiates TLS. On clusters where ArgoCD's NodePort speaks plain HTTP, this causes an EOF. The port-ranking in `argocd.go` prefers `https` (rank 5) over `http` (rank 3), so the wrong port may be selected and `--insecure` is used even when `--plaintext` is correct. There is no retry.

The SKILL.md at `.agents/skills/k3ctx/SKILL.md` has no ArgoCD section, so agents have no reference for the post-connect ArgoCD login pattern.

## Goals / Non-Goals

**Goals:**
- Print `ArgocdLoginResult.Message` in CLI text output when login is not fully successful
- Retry `argocd login` with `--plaintext` when `--insecure` fails
- Prefer `--plaintext` automatically when the discovered port has an `http` port name or port 80
- Add a concise ArgoCD section to SKILL.md

**Non-Goals:**
- Changing the JSON output schema (already includes `argocd_local_port`)
- Supporting ArgoCD SSO or cert-based auth
- Changing how the ArgoCD tunnel port is allocated

## Decisions

**D1: Surface message in CLI output**
Print the `Message` field on a second line below the ArgoCD URL when `!result.ArgocdLoginSuccess()` and message is non-empty. Keep it short — one line. This requires exposing `ArgocdLoginSuccess()` and `ArgocdLoginMessage()` accessors on `ConnectResult` (similar to `ArgocdLocalPort()`).

Alternative: expose full login status struct — rejected, over-engineered for one field.

**D2: Plaintext fallback in infrastructure**
In `LocalArgocdConnector.Setup`, after a failed `--insecure` login, retry once with `--plaintext`. Also: when `bestNodePort` selects a port whose name is `http` or whose port is 80, set `cfg.Plaintext = true` before attempting login, skipping the `--insecure` attempt entirely.

Alternative: always try `--plaintext` first — rejected, most ArgoCD servers use TLS and `--plaintext` to a TLS port will also fail; `--insecure` is the safer first attempt.

**D3: SKILL.md update**
Add a dedicated "ArgoCD" section after the "Common commands" table. Document: auto-login on connect, what to do when skipped (argocd not in PATH), and the manual login command using `kubectl` to get the admin password.

## Risks / Trade-offs

- [Retry adds latency] Second `argocd login` attempt adds ~30s timeout on networks where the first attempt hangs → Mitigation: the retry only runs if the first call returns an error, not a timeout; timeout is already 30s total per call, not additive.
- [Message text changes are observable] Surfacing the message in text output is new — scripts parsing stdout could break → Mitigation: the line is only printed when login is not fully successful; success path is unchanged.

## Migration Plan

No migration needed. All changes are additive:
- New accessor methods on `ConnectResult`
- New retry path in `Setup` (existing behavior unchanged on success)
- New SKILL.md section (existing sections untouched)
