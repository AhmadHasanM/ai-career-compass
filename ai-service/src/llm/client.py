"""Client LLM untuk endpoint OpenAI-compatible (Groq, Gemini, OpenRouter, dll.).

Retry untuk 429 / 5xx / timeout ditangani SDK (max_retries di config.yaml);
model dan kredensial dari .env (LLM_BASE_URL, LLM_API_KEY, LLM_MODEL).
"""

from __future__ import annotations

import logging
import time
from collections.abc import AsyncIterator
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


@dataclass
class StreamDelta:
    """Satu potong teks dari stream. Event terakhir (done=True) membawa model dan usage;
    usage bisa None jika provider tidak mengirimnya."""

    text: str = ""
    done: bool = False
    model: str | None = None
    prompt_tokens: int | None = None
    completion_tokens: int | None = None


class ChatModel(Protocol):
    async def chat(
        self,
        messages: list[dict[str, str]],
        *,
        temperature: float | None = None,
        max_tokens: int | None = None,
        json_mode: bool = False,
    ) -> ChatResult: ...


class StreamingChatModel(ChatModel, Protocol):
    def chat_stream(
        self, messages: list[dict[str, str]], *, temperature: float | None = None, max_tokens: int | None = None
    ) -> AsyncIterator[StreamDelta]: ...


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
        # Tidak semua endpoint kompatibel mendukung response_format / stream_options / reasoning_effort;
        # jika ditolak sekali, parameter itu tidak dikirim lagi.
        self._json_mode_supported = True
        self._stream_usage_supported = True
        self._reasoning_supported = True

    def _base_kwargs(self, messages, temperature, max_tokens) -> dict:
        kwargs: dict = {
            "model": self.model,
            "messages": messages,
            "temperature": self.default_temperature if temperature is None else temperature,
            "max_tokens": max_tokens or self.default_max_tokens,
        }
        if self.reasoning_effort and self._reasoning_supported:
            kwargs["reasoning_effort"] = self.reasoning_effort
        return kwargs

    async def _create(self, kwargs: dict):
        """Panggil API; jika parameter opsional ditolak (400/422), kirim ulang tanpa parameter itu."""
        optional = [("reasoning_effort", "_reasoning_supported"), ("response_format", "_json_mode_supported"),
                    ("stream_options", "_stream_usage_supported")]
        while True:
            try:
                return await self._client.chat.completions.create(**kwargs)
            except Exception as e:  # noqa: BLE001 - SDK melempar berbagai subclass BadRequestError
                rejected = next((p for p, _ in optional if p in kwargs and _rejected_param(e, p)), None)
                if rejected is None:
                    raise
                log.warning("model %s menolak %s; dikirim ulang tanpa parameter itu", self.model, rejected)
                setattr(self, dict(optional)[rejected], False)
                kwargs.pop(rejected)

    async def chat(
        self,
        messages: list[dict[str, str]],
        *,
        temperature: float | None = None,
        max_tokens: int | None = None,
        json_mode: bool = False,
    ) -> ChatResult:
        kwargs = self._base_kwargs(messages, temperature, max_tokens)
        use_json = json_mode and self._json_mode_supported
        if use_json:
            kwargs["response_format"] = {"type": "json_object"}

        start = time.perf_counter()
        resp = await self._create(kwargs)
        latency_ms = int((time.perf_counter() - start) * 1000)

        usage = resp.usage
        return ChatResult(
            content=resp.choices[0].message.content or "",
            model=resp.model or self.model,
            prompt_tokens=usage.prompt_tokens if usage else None,
            completion_tokens=usage.completion_tokens if usage else None,
            latency_ms=latency_ms,
        )

    async def chat_stream(
        self, messages: list[dict[str, str]], *, temperature: float | None = None, max_tokens: int | None = None
    ) -> AsyncIterator[StreamDelta]:
        kwargs = self._base_kwargs(messages, temperature, max_tokens) | {"stream": True}
        if self._stream_usage_supported:
            kwargs["stream_options"] = {"include_usage": True}
        stream = await self._create(kwargs)

        final = StreamDelta(done=True, model=self.model)
        async for chunk in stream:
            if chunk.model:
                final.model = chunk.model
            if chunk.usage:
                final.prompt_tokens = chunk.usage.prompt_tokens
                final.completion_tokens = chunk.usage.completion_tokens
            for choice in chunk.choices or []:
                if choice.delta and choice.delta.content:
                    yield StreamDelta(text=choice.delta.content)
        yield final


def _rejected_param(e: Exception, param: str) -> bool:
    return getattr(e, "status_code", None) in (400, 422) and param in str(e).lower()
