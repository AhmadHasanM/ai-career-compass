"""Log JSON satu baris, termasuk field dari `extra=` (token, latensi, dll.)."""

import json
import logging
import sys

_STANDARD = set(logging.makeLogRecord({}).__dict__) | {"message", "asctime", "taskName"}


class JsonFormatter(logging.Formatter):
    def format(self, record: logging.LogRecord) -> str:
        out = {
            "time": self.formatTime(record, "%Y-%m-%dT%H:%M:%S%z"),
            "level": record.levelname,
            "logger": record.name,
            "msg": record.getMessage(),
        }
        out.update({k: v for k, v in record.__dict__.items() if k not in _STANDARD})
        if record.exc_info:
            out["exc"] = self.formatException(record.exc_info)
        return json.dumps(out, default=str, ensure_ascii=False)


def setup_logging(level: str = "INFO") -> None:
    handler = logging.StreamHandler(sys.stdout)
    handler.setFormatter(JsonFormatter())
    root = logging.getLogger()
    root.handlers[:] = [handler]
    root.setLevel(level.upper())
    for noisy in ("httpx", "httpcore", "openai"):
        logging.getLogger(noisy).setLevel(logging.WARNING)
