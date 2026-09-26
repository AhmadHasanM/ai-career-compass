"""Ekstraksi lowongan dengan LLM: output JSON divalidasi Pydantic, dengan retry bila tidak valid."""

from __future__ import annotations

import json
import logging
import re
from dataclasses import dataclass

from pydantic import ValidationError

from src.extraction.schema import JobExtraction
from src.llm.client import ChatModel
from src.prompts.extraction import SYSTEM_PROMPT, build_repair_prompt, build_user_prompt

log = logging.getLogger(__name__)

_FENCE = re.compile(r"^```(?:json)?\s*|\s*```$", re.IGNORECASE)


class ExtractionError(RuntimeError):
    pass


@dataclass
class ExtractionResult:
    data: JobExtraction
    model: str
    attempts: int
    prompt_tokens: int
    completion_tokens: int


def parse_extraction(content: str, allowed_roles: list[str]) -> JobExtraction:
    """Parse teks balasan LLM. Melempar ValueError dengan pesan yang bisa dikirim balik ke LLM."""
    text = _FENCE.sub("", content.strip())
    # Beberapa model menambah kalimat di sekitar JSON; ambil objek terluar.
    start, end = text.find("{"), text.rfind("}")
    if start == -1 or end <= start:
        raise ValueError("tidak ditemukan objek JSON")
    try:
        raw = json.loads(text[start : end + 1])
    except json.JSONDecodeError as e:
        raise ValueError(f"JSON tidak valid: {e.msg} (posisi {e.pos})") from e
    if not isinstance(raw, dict):
        raise ValueError("root JSON harus objek")

    try:
        data = JobExtraction.model_validate(raw)
    except ValidationError as e:
        issues = "; ".join(f"{'.'.join(map(str, err['loc']))}: {err['msg']}" for err in e.errors())
        raise ValueError(f"tidak sesuai skema: {issues}") from e

    if data.role not in allowed_roles and data.role != "other":
        raise ValueError(f"role '{data.role}' tidak valid; pilih dari {', '.join(allowed_roles)}, other")

    # Skill yang muncul di dua daftar dianggap wajib.
    required_lower = {s.lower() for s in data.required_skills}
    data.preferred_skills = [s for s in data.preferred_skills if s.lower() not in required_lower]
    return data


class JobExtractor:
    def __init__(self, llm: ChatModel, *, max_retries: int = 2, max_input_chars: int = 12000,
                 temperature: float = 0.0, max_tokens: int = 1500) -> None:
        self.llm = llm
        self.max_retries = max_retries
        self.max_input_chars = max_input_chars
        self.temperature = temperature
        self.max_tokens = max_tokens

    async def extract(self, *, title: str, company: str | None, raw_text: str, roles: list[str]) -> ExtractionResult:
        text = raw_text if len(raw_text) <= self.max_input_chars else raw_text[: self.max_input_chars] + "\n[dipotong]"
        messages = [
            {"role": "system", "content": SYSTEM_PROMPT},
            {"role": "user", "content": build_user_prompt(title=title, company=company, raw_text=text, roles=roles)},
        ]
        prompt_tokens = completion_tokens = 0
        last_error = ""
        for attempt in range(1, self.max_retries + 2):
            res = await self.llm.chat(messages, temperature=self.temperature, max_tokens=self.max_tokens, json_mode=True)
            prompt_tokens += res.prompt_tokens or 0
            completion_tokens += res.completion_tokens or 0
            try:
                data = parse_extraction(res.content, roles)
                return ExtractionResult(data, res.model, attempt, prompt_tokens, completion_tokens)
            except ValueError as e:
                last_error = str(e)
                log.warning("ekstraksi percobaan %d gagal: %s", attempt, last_error)
                messages += [
                    {"role": "assistant", "content": res.content},
                    {"role": "user", "content": build_repair_prompt(last_error)},
                ]
        raise ExtractionError(f"output LLM tetap tidak valid setelah {self.max_retries + 1} percobaan: {last_error}")
