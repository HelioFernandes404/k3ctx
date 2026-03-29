"""Unit tests for logging_config module."""

import pytest
import tempfile
import logging
from pathlib import Path
from src.logging_config import setup_logging, get_logger


class TestSetupLogging:
    """Tests for setup_logging function."""

    def test_setup_logging_with_file_output(self):
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

    def test_setup_logging_without_file(self):
        """Configures logging to stderr only when no file specified."""
        logger = setup_logging(log_file=None, level=logging.INFO)

        # Should have at least one handler
        assert len(logger.handlers) > 0

        # Should be StreamHandler
        from logging import StreamHandler
        assert any(isinstance(h, StreamHandler) for h in logger.handlers)

    def test_setup_logging_creates_log_directory(self):
        """Creates parent directory if log file path doesn't exist."""
        with tempfile.TemporaryDirectory() as tmpdir:
            log_file = Path(tmpdir) / "subdir" / "logs" / "test.log"

            logger = setup_logging(log_file=str(log_file))
            logger.info("Test message")

            # Verify parent directory was created
            assert log_file.parent.exists()
            assert log_file.exists()

    def test_get_logger_returns_configured_logger(self):
        """get_logger returns the configured logger."""
        logger = get_logger()
        assert logger.name == "k9s-config"
        assert len(logger.handlers) > 0


class TestLogFormatting:
    """Tests for log message formatting."""

    def test_log_messages_include_level_and_message(self):
        """Log messages have format [LEVEL] message."""
        with tempfile.TemporaryDirectory() as tmpdir:
            log_file = Path(tmpdir) / "test.log"

            logger = setup_logging(log_file=str(log_file))
            logger.info("Info message")
            logger.error("Error message")

            content = log_file.read_text()
            assert "[INFO] Info message" in content
            assert "[ERROR] Error message" in content
