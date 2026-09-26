"""Panggilan LLM yang wajib menghasilkan JSON valid, dengan retry + umpan balik error."""

from __future__ import annotations

import json
import logging
import re
from collections.abc import Callable
from dataclasses import dataclass
from typing import Generic, TypeVar

from src.llm.client import ChatModel

log = logging.getLogger(__name__)

T = TypeVar("T")

_FENCE = re.compile(r"^```(?:json)?\s*|\s*```$", re.IGNORECASE)


class JSONCallError(RuntimeError):
    pass


@dataclass
class JSONResult(Generic[T]):
    data: T
    model: str
    attempts: int
    prompt_tokens: int
    completion_tokens: int


def load_json_object(content: str) -> dict:
    """Ambil objek JSON terluar dari balasan LLM (toleran terhadap code fence dan teks pengantar)."""
    text = _FENCE.sub("", content.strip())
    start, end = text.find("{"), text.rfind("}")
    if start == -1 or end <= start:
        raise ValueError("tidak ditemukan objek JSON")
    try:
        raw = json.loads(text[start : end + 1])
    except json.JSONDecodeError as e:
        raise ValueError(f"JSON tidak valid: {e.msg} (posisi {e.pos})") from e
    if not isinstance(raw, dict):
        raise ValueError("root JSON harus objek")
    return raw


def repair_prompt(error: str) -> str:
    return (
        f"Output sebelumnya tidak valid: {error}\n"
        "Kirim ulang HANYA objek JSON yang valid sesuai skema, tanpa penjelasan."
    )


async def chat_json(
    llm: ChatModel,
    messages: list[dict[str, str]],
    parse: Callable[[str], T],
    *,
    max_retries: int = 2,
    temperature: float | None = None,
    max_tokens: int | None = None,
) -> JSONResult[T]:
    """parse() melempar ValueError berisi pesan yang dikirim balik ke LLM pada percobaan berikutnya."""
    messages = list(messages)
    prompt_tokens = completion_tokens = 0
    last_error = ""
    for attempt in range(1, max_retries + 2):
        res = await llm.chat(messages, temperature=temperature, max_tokens=max_tokens, json_mode=True)
        prompt_tokens += res.prompt_tokens or 0
        completion_tokens += res.completion_tokens or 0
        try:
            return JSONResult(parse(res.content), res.model, attempt, prompt_tokens, completion_tokens)
        except ValueError as e:
            last_error = str(e)
            log.warning("output JSON percobaan %d tidak valid: %s", attempt, last_error)
            messages += [
                {"role": "assistant", "content": res.content},
                {"role": "user", "content": repair_prompt(last_error)},
            ]
    raise JSONCallError(f"output LLM tetap tidak valid setelah {max_retries + 1} percobaan: {last_error}")
