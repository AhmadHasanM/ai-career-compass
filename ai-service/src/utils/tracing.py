"""Observability chat: setiap query dicatat (pertanyaan, chunk ter-retrieve, latensi, token).

LogTracer menulis log JSON terstruktur. Adapter Langfuse bisa ditambahkan dengan interface yang sama
setelah LANGFUSE_PUBLIC_KEY / LANGFUSE_SECRET_KEY tersedia.
"""

import logging
from typing import Protocol

log = logging.getLogger("chat")


class Tracer(Protocol):
    def chat(self, **fields) -> None: ...


class LogTracer:
    def chat(self, **fields) -> None:
        log.info("chat", extra=fields)
