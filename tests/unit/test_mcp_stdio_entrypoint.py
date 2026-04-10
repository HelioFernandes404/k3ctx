"""Unit tests for the official stdio MCP entrypoint."""

from __future__ import annotations

import os

from pytest_mock import MockerFixture


def test_main_stdio_disables_fastmcp_logs_and_runs_stdio_server(
    mocker: MockerFixture,
) -> None:
    from src.interfaces.mcp import server as mcp_server

    run = mocker.patch.object(mcp_server.mcp, "run")

    mcp_server.main_stdio()

    run.assert_called_once_with(show_banner=False)
    assert os.environ["FASTMCP_LOG_ENABLED"] == "false"
    assert os.environ["FASTMCP_SHOW_SERVER_BANNER"] == "false"
