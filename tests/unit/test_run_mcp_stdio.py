"""Unit tests for the stdio MCP wrapper entrypoint."""

from __future__ import annotations

import os
from typing import Any

import run_mcp_stdio
from pytest_mock import MockerFixture


def test_main_disables_fastmcp_logs_and_runs_stdio_server(
    mocker: MockerFixture,
) -> None:
    server = mocker.Mock()
    fake_module = mocker.Mock()
    fake_module.mcp = server

    real_import = __import__

    def fake_import(
        name: str,
        globals: dict[str, Any] | None = None,
        locals: dict[str, Any] | None = None,
        fromlist: tuple[str, ...] = (),
        level: int = 0,
    ) -> Any:
        if name == "src.mcp_server":
            assert os.environ["FASTMCP_LOG_ENABLED"] == "false"
            assert os.environ["FASTMCP_SHOW_SERVER_BANNER"] == "false"
            return fake_module
        return real_import(name, globals, locals, fromlist, level)

    import_mock = mocker.patch("builtins.__import__", side_effect=fake_import)

    run_mcp_stdio.main()

    assert import_mock.called
    server.run.assert_called_once_with(show_banner=False)
    assert os.environ["FASTMCP_LOG_ENABLED"] == "false"
    assert os.environ["FASTMCP_SHOW_SERVER_BANNER"] == "false"
