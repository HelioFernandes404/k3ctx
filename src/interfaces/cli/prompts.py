"""Interactive prompt helpers for the official CLI."""

from __future__ import annotations

import sys
from typing import Optional, Sequence, cast

import questionary
from questionary import Style

from src.domain.models import ClusterTarget
from src.domain.network import detect_network_requirement

custom_style = Style([
    ("qmark", "fg:#E91E63 bold"),
    ("question", "bold"),
    ("answer", "fg:#2196F3 bold"),
    ("pointer", "fg:#E91E63 bold"),
    ("highlighted", "fg:#E91E63 bold"),
    ("selected", "fg:#E91E63"),
    ("separator", "fg:#cc5454"),
    ("instruction", ""),
    ("text", ""),
    ("disabled", "fg:#858585 italic"),
])


class NonInteractiveTerminalError(RuntimeError):
    """Raised when an interactive prompt is attempted without a TTY."""


def _is_interactive_terminal() -> bool:
    stdin_isatty = getattr(sys.stdin, "isatty", lambda: False)()
    stdout_isatty = getattr(sys.stdout, "isatty", lambda: False)()
    return bool(stdin_isatty and stdout_isatty)


def require_interactive_terminal() -> None:
    if _is_interactive_terminal():
        return

    raise NonInteractiveTerminalError(
        "This command requires an interactive terminal. Run it in a local shell."
    )


def confirm_action(message: str, default: bool = False) -> Optional[bool]:
    require_interactive_terminal()
    return cast(
        Optional[bool],
        questionary.confirm(message, default=default, style=custom_style).ask(),
    )


def _autocomplete(
    message: str,
    choices: Sequence[str],
) -> Optional[str]:
    return cast(
        Optional[str],
        questionary.autocomplete(
            message,
            choices=list(choices),
            match_middle=True,
            style=custom_style,
        ).ask(),
    )


def format_target_label(target: ClusterTarget) -> str:
    network_type, network_range, needs_vpn = detect_network_requirement(
        target.host_config,
        target.group_vars,
        target.group,
    )

    indicators: list[str] = []
    if needs_vpn:
        indicators.append("[VPN]")
    if network_type == "sshuttle":
        indicators.append(f"[sshuttle {network_range}]")

    label = f"{target.host_alias} ({target.group})"
    if indicators:
        return f"{label} {' '.join(indicators)}"
    return label


def format_multi_target_label(target: ClusterTarget) -> str:
    network_type, _, needs_vpn = detect_network_requirement(
        target.host_config,
        target.group_vars,
        target.group,
    )
    indicators: list[str] = []
    if needs_vpn:
        indicators.append("[VPN]")
    if network_type == "sshuttle":
        indicators.append("[sshuttle]")
    label = f"{target.company}: {target.host_alias}"
    if indicators:
        return f"{label} {' '.join(indicators)}"
    return label


def select_company(companies: Sequence[str]) -> Optional[str]:
    require_interactive_terminal()
    return _autocomplete("Select company (type to search):", sorted(companies))


def select_single_target(
    company: str,
    targets: Sequence[ClusterTarget],
) -> ClusterTarget | None:
    require_interactive_terminal()
    labels = {format_target_label(target): target for target in targets}
    selected_label = _autocomplete(
        f"Select host in {company} (type to search):",
        list(labels),
    )
    if selected_label is None:
        return None
    return labels[selected_label]


def select_multiple_targets(targets: Sequence[ClusterTarget]) -> list[ClusterTarget]:
    require_interactive_terminal()

    selected: list[ClusterTarget] = []
    label_to_target = {format_multi_target_label(target): target for target in targets}
    labels = list(label_to_target.keys())
    selected_labels: set[str] = set()

    print(f"\n📋 Available: {len(labels)} clusters")
    print("💡 Tip: Type to search, Enter to add, Ctrl+C quando terminar\n")

    while True:
        remaining = [label for label in labels if label not in selected_labels]
        if not remaining:
            print("All clusters selected!")
            break

        if selected:
            print(f"\n✓ Selected ({len(selected)}):")
            for index, target in enumerate(selected, start=1):
                print(f"  {index}. {format_multi_target_label(target)}")
            print()

        try:
            choice = questionary.autocomplete(
                f"Add cluster (type to search, {len(remaining)} remaining):",
                choices=remaining,
                match_middle=True,
                style=custom_style,
            ).ask()
        except KeyboardInterrupt:
            print()
            break

        if choice is None:
            break

        if choice in label_to_target:
            target = label_to_target[choice]
            selected.append(target)
            selected_labels.add(choice)
            print(f"  ✓ Added: {choice}")

    return selected
