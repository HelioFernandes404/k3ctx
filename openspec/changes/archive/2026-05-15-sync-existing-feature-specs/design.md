## Context

`k3ctx` is a Go CLI with behavior spread across Cobra commands, application usecases, domain models, and local infrastructure adapters. The current OpenSpec baseline only contains `argocd-auto-discovery`, but the shipped code also covers configuration, inventory discovery, host resolution, cluster connection, tunnel management, and `k9s` launching.

This change is a documentation/spec synchronization pass. The implementation is treated as the source of truth, with specs capturing externally observable behavior and stable domain rules.

## Goals / Non-Goals

**Goals:**
- Create missing baseline capabilities under `openspec/specs/`.
- Describe command-visible behavior, public JSON/text outputs where meaningful, error outcomes, and persistent local state paths.
- Keep specs implementation-aware enough to guide future work without documenting every private helper.
- Preserve `argocd-auto-discovery` unchanged.

**Non-Goals:**
- Change CLI behavior, command names, flags, exit codes, or runtime state paths.
- Refactor Go code.
- Add tests or implementation tasks for new product behavior.
- Archive this change automatically.

## Decisions

### Decision: Use Multiple Focused Capabilities

The baseline will be split into focused capabilities instead of one large `cli-baseline` spec.

Alternatives considered:
- Single broad spec: simpler file count, but harder to evolve and review.
- One spec per command: too fragmented because several commands share inventory/config/tunnel behavior.

Rationale: focused capabilities match the architecture seams: config, inventory, connection, tunnel management, and k9s launch.

### Decision: Treat Existing Code As Canonical

Specs will describe current behavior as implemented, not idealized future behavior.

Alternatives considered:
- Redesign specs around desired future UX: useful later, but would confuse sync work with product changes.

Rationale: the purpose is to make OpenSpec accurate before future changes are proposed.

### Decision: Keep ArgoCD Separate

The existing `argocd-auto-discovery` spec remains independent.

Alternatives considered:
- Merge ArgoCD into `cluster-connection`: this would hide a distinct optional best-effort integration.

Rationale: ArgoCD has its own discovery and login semantics and was already archived as a separate change.

## Risks / Trade-offs

- Spec drift from subtle code behavior -> Mitigation: inspect command/usecase/infrastructure files before writing scenarios.
- Over-documenting internals -> Mitigation: focus requirements on observable behavior and stable domain decisions.
- Missing edge cases -> Mitigation: include explicit scenarios for no-match, ambiguous match, missing external binaries, missing config, and tunnel state.
