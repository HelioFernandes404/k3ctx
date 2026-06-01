# k3ctx Skill

Workflow guide for using `k3ctx` to open SSH tunnels, merge kubeconfig contexts, and access K3s clusters with `kubectl` or `k9s`.

## Install

Install the `skills/k3ctx` directory with an Agent Skills-compatible installer.

If your installer supports GitHub repository specs, use:

```bash
npx skills add HelioFernandes404/k3ctx@k3ctx
```

For a specific agent:

```bash
npx skills add HelioFernandes404/k3ctx@k3ctx -a claude-code
npx skills add HelioFernandes404/k3ctx@k3ctx -a codex
```

Globally without prompts:

```bash
npx skills add HelioFernandes404/k3ctx@k3ctx -g -y
```

## Files

```text
skills/k3ctx/
+-- README.md
`-- SKILL.md
```

## Trigger

Use this skill when working with `k3ctx`, K3s cluster access, SSH tunnels, kubeconfig contexts, `kubectl`, or `k9s`.
