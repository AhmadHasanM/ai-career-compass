"""Embedding teks: lewat API OpenAI-compatible atau model lokal multilingual-e5-base.

Model e5 butuh prefix "query: " untuk pertanyaan dan "passage: " untuk dokumen.
Dimensi harus sama dengan kolom document_chunks.embedding (vector(768)).
"""

from __future__ import annotations

import asyncio
import logging
from functools import lru_cache
from typing import Protocol

from src.utils.config import Settings, get_settings

log = logging.getLogger(__name__)


class Embedder(Protocol):
    dim: int
    model_name: str

    async def embed_documents(self, texts: list[str]) -> list[list[float]]: ...
    async def embed_query(self, text: str) -> list[float]: ...


class DimensionMismatchError(RuntimeError):
    pass


def _check_dim(vectors: list[list[float]], dim: int, model: str) -> list[list[float]]:
    for v in vectors:
        if len(v) != dim:
            raise DimensionMismatchError(
                f"model {model} menghasilkan dimensi {len(v)}, kolom database {dim}; "
                "sesuaikan EMBEDDING_DIM dan migrasi"
            )
    return vectors


def _is_e5(model: str) -> bool:
    return "e5" in model.lower()


class LocalEmbedder:
    """sentence-transformers di CPU; model diunduh ke cache HF saat pertama dipakai."""

    def __init__(self, model_name: str, dim: int, batch_size: int = 32) -> None:
        self.model_name, self.dim, self.batch_size = model_name, dim, batch_size
        self._model = None
        self._lock = asyncio.Lock()

    async def _load(self):
        async with self._lock:
            if self._model is None:
                log.info("memuat model embedding lokal %s", self.model_name)
                from sentence_transformers import SentenceTransformer

                self._model = await asyncio.to_thread(SentenceTransformer, self.model_name, device="cpu")
        return self._model

    async def _encode(self, texts: list[str]) -> list[list[float]]:
        model = await self._load()
        arr = await asyncio.to_thread(
            model.encode, texts, batch_size=self.batch_size, normalize_embeddings=True, show_progress_bar=False
        )
        return _check_dim([v.tolist() for v in arr], self.dim, self.model_name)

    async def embed_documents(self, texts: list[str]) -> list[list[float]]:
        prefix = "passage: " if _is_e5(self.model_name) else ""
        return await self._encode([prefix + t for t in texts])

    async def embed_query(self, text: str) -> list[float]:
        prefix = "query: " if _is_e5(self.model_name) else ""
        return (await self._encode([prefix + text]))[0]


class ApiEmbedder:
    def __init__(self, settings: Settings, batch_size: int = 32) -> None:
        from openai import AsyncOpenAI

        cfg = settings.pipeline.get("llm", {})
        self.model_name, self.dim, self.batch_size = settings.embedding_model, settings.embedding_dim, batch_size
        self._client = AsyncOpenAI(
            base_url=settings.llm_base_url,
            api_key=settings.llm_api_key,
            timeout=cfg.get("timeout_seconds", 60),
            max_retries=cfg.get("max_retries", 3),
        )

    async def _embed(self, texts: list[str]) -> list[list[float]]:
        out: list[list[float]] = []
        for i in range(0, len(texts), self.batch_size):
            resp = await self._client.embeddings.create(model=self.model_name, input=texts[i : i + self.batch_size])
            out.extend(d.embedding for d in sorted(resp.data, key=lambda d: d.index))
        return _check_dim(out, self.dim, self.model_name)

    async def embed_documents(self, texts: list[str]) -> list[list[float]]:
        prefix = "passage: " if _is_e5(self.model_name) else ""
        return await self._embed([prefix + t for t in texts])

    async def embed_query(self, text: str) -> list[float]:
        prefix = "query: " if _is_e5(self.model_name) else ""
        return (await self._embed([prefix + text]))[0]


@lru_cache
def get_embedder() -> Embedder:
    s = get_settings()
    batch = s.pipeline.get("embedding", {}).get("batch_size", 32)
    if s.embedding_provider == "api":
        return ApiEmbedder(s, batch)
    if s.embedding_provider != "local":
        raise ValueError(f"EMBEDDING_PROVIDER tidak dikenal: {s.embedding_provider} (pakai 'api' atau 'local')")
    return LocalEmbedder(s.embedding_model, s.embedding_dim, batch)
