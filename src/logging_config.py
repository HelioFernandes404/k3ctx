"""Logging configuration for k9s-config."""

import json
import logging
import re
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Optional


_REDACTED = "[REDACTED]"
_SENSITIVE_FIELD_MARKERS = (
    "token",
    "secret",
    "private_key",
    "kubeconfig",
    "password",
    "passwd",
    "authorization",
)
_RESERVED_RECORD_FIELDS = set(logging.makeLogRecord({}).__dict__.keys())
_SENSITIVE_ASSIGNMENT_PATTERN = re.compile(
    r"""(?ix)
    \b(token|secret|private_key|kubeconfig|password|passwd|authorization)\b
    (\s*[:=]\s*)
    (
        "[^"]*"
        | '[^']*'
        | bearer\s+[^\s,;]+
        | [^\s,;]+
    )
    """
)


def _is_sensitive_field(key: str) -> bool:
    lowered_key = key.lower()
    return any(marker in lowered_key for marker in _SENSITIVE_FIELD_MARKERS)


def _sanitize_log_text(text: str) -> str:
    return _SENSITIVE_ASSIGNMENT_PATTERN.sub(r"\1\2[REDACTED]", text)


def _sanitize_log_value(value: Any, *, field_name: str | None = None) -> Any:
    if field_name and _is_sensitive_field(field_name):
        return _REDACTED

    if isinstance(value, dict):
        return {
            str(key): _sanitize_log_value(inner, field_name=str(key))
            for key, inner in value.items()
        }
    if isinstance(value, (list, tuple, set, frozenset)):
        return [_sanitize_log_value(item) for item in value]
    if isinstance(value, Path):
        return str(value)
    if isinstance(value, str):
        return _sanitize_log_text(value)
    if isinstance(value, (int, float, bool)) or value is None:
        return value

    return str(value)


class StructuredFormatter(logging.Formatter):
    """Stable JSON formatter with basic field sanitization."""

    def format(self, record: logging.LogRecord) -> str:
        payload: dict[str, Any] = {
            "timestamp": datetime.fromtimestamp(
                record.created,
                tz=timezone.utc,
            ).isoformat(),
            "level": record.levelname,
            "logger": record.name,
            "message": _sanitize_log_text(record.getMessage()),
        }

        for key, value in record.__dict__.items():
            if key in _RESERVED_RECORD_FIELDS or key in payload:
                continue
            payload[key] = _sanitize_log_value(value, field_name=key)

        if record.exc_info:
            exc_type = record.exc_info[0]
            payload["exception"] = {
                "type": exc_type.__name__ if exc_type is not None else "UnknownError",
            }

        return json.dumps(payload, sort_keys=True)


def setup_logging(
    level: int = logging.DEBUG,
    verbose: bool = False,
    log_file: Optional[str] = None,
    structured: bool = False,
) -> logging.Logger:
    """
    Configure logging for k9s-config.

    Args:
        level: Logging level (default: DEBUG)
        verbose: If True, use DEBUG level (deprecated - DEBUG is now default)
        log_file: Optional path to log file (if provided, logs to file + stderr)
        structured: If True, emit stable JSON logs

    Returns:
        Configured logger instance
    """
    if verbose:
        level = logging.DEBUG

    logger = logging.getLogger("k9s-config")
    logger.setLevel(level)

    # Remove existing handlers to avoid duplicates
    logger.handlers = []

    formatter: logging.Formatter
    if structured:
        formatter = StructuredFormatter()
    else:
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
