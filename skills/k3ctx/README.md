# k3ctx Skill

Workflow guide for using `k3ctx` to open SSH tunnels, merge kubeconfig contexts, and access K3s clusters with `kubectl` or `k9s`.

## Install

```bash
npx skills add HelioFernandes404/k3ctx@k3ctx
```

Install for a specific agent:

```bash
npx skills add HelioFernandes404/k3ctx@k3ctx -a claude-code
npx skills add HelioFernandes404/k3ctx@k3ctx -a codex
```

Install globally without prompts:

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
