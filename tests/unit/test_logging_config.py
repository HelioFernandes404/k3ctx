"""Unit tests for logging_config module."""

import json
import logging
import tempfile
from pathlib import Path

import pytest

from src.logging_config import setup_logging, get_logger


class TestSetupLogging:
    """Tests for setup_logging function."""

    def test_setup_logging_with_file_output(self) -> None:
        """Configures logging to write to file."""
        with tempfile.TemporaryDirectory() as tmpdir:
            log_file = Path(tmpdir) / "test.log"

            logger = setup_logging(log_file=str(log_file), level=logging.DEBUG)
            logger.info("Test message")
            logger.warning("Warning message")

            # Verify log file was created
            assert log_file.exists()

            # Verify content
            content = log_file.read_text()
            assert "Test message" in content
            assert "WARNING" in content

    def test_setup_logging_without_file(self) -> None:
        """Configures logging to stderr only when no file specified."""
        logger = setup_logging(log_file=None, level=logging.INFO)

        # Should have at least one handler
        assert len(logger.handlers) > 0

        # Should be StreamHandler
        from logging import StreamHandler
        assert any(isinstance(h, StreamHandler) for h in logger.handlers)

    def test_setup_logging_creates_log_directory(self) -> None:
        """Creates parent directory if log file path doesn't exist."""
        with tempfile.TemporaryDirectory() as tmpdir:
            log_file = Path(tmpdir) / "subdir" / "logs" / "test.log"

            logger = setup_logging(log_file=str(log_file))
            logger.info("Test message")

            # Verify parent directory was created
            assert log_file.parent.exists()
            assert log_file.exists()

    def test_get_logger_returns_configured_logger(self) -> None:
        """get_logger returns the configured logger."""
        logger = get_logger()
        assert logger.name == "k9s-config"
        assert len(logger.handlers) > 0


class TestLogFormatting:
    """Tests for log message formatting."""

    def test_log_messages_include_level_and_message(self) -> None:
        """Log messages have format [LEVEL] message."""
        with tempfile.TemporaryDirectory() as tmpdir:
            log_file = Path(tmpdir) / "test.log"

            logger = setup_logging(log_file=str(log_file))
            logger.info("Info message")
            logger.error("Error message")

            content = log_file.read_text()
            assert "[INFO] Info message" in content
            assert "[ERROR] Error message" in content

    def test_structured_logging_outputs_json_payload(self) -> None:
        """Structured logging emits stable JSON records."""
        with tempfile.TemporaryDirectory() as tmpdir:
            log_file = Path(tmpdir) / "structured.log"

            logger = setup_logging(
                log_file=str(log_file),
                structured=True,
                level=logging.INFO,
            )
            logger.info(
                "Connected cluster",
                extra={
                    "event": "connect.finished",
                    "context_name": "acme-prod",
                    "used_cache": True,
                },
            )

            payload = json.loads(log_file.read_text().strip())
            assert payload["level"] == "INFO"
            assert payload["logger"] == "k9s-config"
            assert payload["message"] == "Connected cluster"
            assert payload["event"] == "connect.finished"
            assert payload["context_name"] == "acme-prod"
            assert payload["used_cache"] is True
            assert "timestamp" in payload

    def test_structured_logging_sanitizes_sensitive_fields(self) -> None:
        """Structured logging redacts known sensitive fields."""
        with tempfile.TemporaryDirectory() as tmpdir:
            log_file = Path(tmpdir) / "structured.log"

            logger = setup_logging(
                log_file=str(log_file),
                structured=True,
                level=logging.INFO,
            )
            logger.info(
                "Loaded config",
                extra={
                    "token": "abc123",
                    "secret_value": "top-secret",
                    "private_key": "pem-data",
                    "kubeconfig": {"clusters": ["raw"]},
                    "nested": {
                        "api_token": "nested-token",
                        "safe_value": "ok",
                    },
                },
            )

            payload = json.loads(log_file.read_text().strip())
            assert payload["token"] == "[REDACTED]"
            assert payload["secret_value"] == "[REDACTED]"
            assert payload["private_key"] == "[REDACTED]"
            assert payload["kubeconfig"] == "[REDACTED]"
            assert payload["nested"]["api_token"] == "[REDACTED]"
            assert payload["nested"]["safe_value"] == "ok"

    def test_structured_logging_sanitizes_sensitive_patterns_inside_message(self) -> None:
        """Structured logging redacts secrets embedded in the message text."""
        with tempfile.TemporaryDirectory() as tmpdir:
            log_file = Path(tmpdir) / "structured.log"

            logger = setup_logging(
                log_file=str(log_file),
                structured=True,
                level=logging.INFO,
            )
            logger.error(
                "failed token=abc123 private_key=/tmp/key password=hunter2 authorization=Bearer123",
            )

            payload = json.loads(log_file.read_text().strip())
            assert payload["message"] == (
                "failed token=[REDACTED] private_key=[REDACTED] "
                "password=[REDACTED] authorization=[REDACTED]"
            )

    def test_structured_logging_sanitizes_sensitive_message_values_with_spaces(self) -> None:
        """Structured logging redacts spaced and quoted sensitive values."""
        with tempfile.TemporaryDirectory() as tmpdir:
            log_file = Path(tmpdir) / "structured.log"

            logger = setup_logging(
                log_file=str(log_file),
                structured=True,
                level=logging.INFO,
            )
            logger.error(
                'failed Authorization: Bearer abc123 password="hunter 2"',
            )

            payload = json.loads(log_file.read_text().strip())
            assert payload["message"] == (
                'failed Authorization: [REDACTED] password=[REDACTED]'
            )
