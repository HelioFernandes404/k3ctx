"""Logging configuration for k9s-config."""

import logging
import sys
from pathlib import Path
from typing import Optional


def setup_logging(level: int = logging.DEBUG, verbose: bool = False, log_file: Optional[str] = None) -> logging.Logger:
    """
    Configure logging for k9s-config.

    Args:
        level: Logging level (default: DEBUG)
        verbose: If True, use DEBUG level (deprecated - DEBUG is now default)
        log_file: Optional path to log file (if provided, logs to file + stderr)

    Returns:
        Configured logger instance
    """
    if verbose:
        level = logging.DEBUG

    logger = logging.getLogger("k9s-config")
    logger.setLevel(level)

    # Remove existing handlers to avoid duplicates
    logger.handlers = []

    # Formatter: [LEVEL] message
    formatter = logging.Formatter(
        fmt="[%(levelname)s] %(message)s",
        datefmt="%Y-%m-%d %H:%M:%S"
    )

    # Console handler (stderr)
    console_handler = logging.StreamHandler(sys.stderr)
    console_handler.setLevel(level)
    console_handler.setFormatter(formatter)
    logger.addHandler(console_handler)

    # File handler (optional)
    if log_file:
        log_path = Path(log_file)
        log_path.parent.mkdir(parents=True, exist_ok=True)

        file_handler = logging.FileHandler(log_file)
        file_handler.setLevel(level)
        file_handler.setFormatter(formatter)
        logger.addHandler(file_handler)

    return logger


def get_logger() -> logging.Logger:
    """Get the configured logger instance."""
    logger = logging.getLogger("k9s-config")
    if not logger.handlers:
        setup_logging()
    return logger
