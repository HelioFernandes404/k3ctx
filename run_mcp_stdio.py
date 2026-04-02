"""Stable stdio entrypoint for local MCP clients."""

from __future__ import annotations

import os


def main() -> None:
    os.environ.setdefault("FASTMCP_LOG_ENABLED", "false")
    os.environ.setdefault("FASTMCP_SHOW_SERVER_BANNER", "false")

    from src.mcp_server import mcp

    mcp.run(show_banner=False)


if __name__ == "__main__":
    main()
