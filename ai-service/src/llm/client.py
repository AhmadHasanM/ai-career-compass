"""Client LLM untuk endpoint OpenAI-compatible (OpenCode).

Retry untuk 429 / 5xx / timeout ditangani SDK (max_retries di config.yaml);
model dan kredensial dari .env (LLM_BASE_URL, LLM_API_KEY, LLM_MODEL).
"""

from __future__ import annotations

import logging
import time
from dataclasses import dataclass
from typing import Protocol

from openai import AsyncOpenAI

from src.utils.config import Settings, get_settings

log = logging.getLogger(__name__)


class LLMNotConfiguredError(RuntimeError):
    pass


@dataclass
class ChatResult:
    content: str
    model: str
    prompt_tokens: int | None
    completion_tokens: int | None
    latency_ms: int


class ChatModel(Protocol):
    async def chat(
        self,
        messages: list[dict[str, str]],
        *,
        temperature: float | None = None,
        max_tokens: int | None = None,
        json_mode: bool = False,
    ) -> ChatResult: ...


class OpenAICompatibleLLM:
    def __init__(self, settings: Settings | None = None) -> None:
        s = settings or get_settings()
        if not (s.llm_base_url and s.llm_api_key and s.llm_model):
            raise LLMNotConfiguredError("LLM_BASE_URL, LLM_API_KEY, dan LLM_MODEL wajib diisi di .env")
        cfg = s.pipeline.get("llm", {})
        self.model = s.llm_model
        self.default_temperature = cfg.get("temperature", 0.2)
        self.default_max_tokens = cfg.get("max_tokens", 1024)
        self.reasoning_effort = cfg.get("reasoning_effort")
        self._client = AsyncOpenAI(
            base_url=s.llm_base_url,
            api_key=s.llm_api_key,
            timeout=cfg.get("timeout_seconds", 60),
            max_retries=cfg.get("max_retries", 3),
        )
        # Tidak semua model di endpoint kompatibel mendukung response_format;
        # jika ditolak sekali, berikutnya instruksi JSON di prompt yang diandalkan.
        self._json_mode_supported = True

    async def chat(
        self,
        messages: list[dict[str, str]],
        *,
        temperature: float | None = None,
        max_tokens: int | None = None,
        json_mode: bool = False,
    ) -> ChatResult:
        kwargs: dict = {
            "model": self.model,
            "messages": messages,
            "temperature": self.default_temperature if temperature is None else temperature,
            "max_tokens": max_tokens or self.default_max_tokens,
        }
        if self.reasoning_effort:
            kwargs["reasoning_effort"] = self.reasoning_effort
        use_json = json_mode and self._json_mode_supported
        if use_json:
            kwargs["response_format"] = {"type": "json_object"}

        start = time.perf_counter()
        try:
            resp = await self._client.chat.completions.create(**kwargs)
        except Exception as e:  # noqa: BLE001 - SDK melempar berbagai subclass BadRequestError
            if use_json and _looks_like_unsupported_response_format(e):
                log.warning("model %s menolak response_format; lanjut tanpa JSON mode", self.model)
                self._json_mode_supported = False
                kwargs.pop("response_format")
                resp = await self._client.chat.completions.create(**kwargs)
            else:
                raise
        latency_ms = int((time.perf_counter() - start) * 1000)

        usage = resp.usage
        return ChatResult(
            content=resp.choices[0].message.content or "",
            model=resp.model or self.model,
            prompt_tokens=usage.prompt_tokens if usage else None,
            completion_tokens=usage.completion_tokens if usage else None,
            latency_ms=latency_ms,
        )


def _looks_like_unsupported_response_format(e: Exception) -> bool:
    status = getattr(e, "status_code", None)
    return status in (400, 422) and "response_format" in str(e).lower()
