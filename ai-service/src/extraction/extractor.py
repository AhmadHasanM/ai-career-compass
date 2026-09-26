"""Ekstraksi lowongan dengan LLM: output JSON divalidasi Pydantic, dengan retry bila tidak valid."""

from __future__ import annotations

from dataclasses import dataclass

from pydantic import ValidationError

from src.extraction.schema import JobExtraction
from src.llm.client import ChatModel
from src.llm.json_call import JSONCallError, chat_json, load_json_object
from src.prompts.extraction import SYSTEM_PROMPT, build_user_prompt

# Nama lama dipertahankan agar pemanggil (pipeline, test) tidak perlu tahu detail helper JSON.
ExtractionError = JSONCallError


@dataclass
class ExtractionResult:
    data: JobExtraction
    model: str
    attempts: int
    prompt_tokens: int
    completion_tokens: int


def parse_extraction(content: str, allowed_roles: list[str]) -> JobExtraction:
    """Parse teks balasan LLM. Melempar ValueError dengan pesan yang bisa dikirim balik ke LLM."""
    raw = load_json_object(content)
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
        res = await chat_json(
            self.llm, messages, lambda c: parse_extraction(c, roles),
            max_retries=self.max_retries, temperature=self.temperature, max_tokens=self.max_tokens,
        )
        return ExtractionResult(res.data, res.model, res.attempts, res.prompt_tokens, res.completion_tokens)
