"""Merakit JobProcessor dari konfigurasi."""

from functools import lru_cache

from psycopg_pool import AsyncConnectionPool

from src.embeddings.embedder import get_embedder
from src.extraction.extractor import JobExtractor
from src.ingestion.pipeline import JobProcessor
from src.ingestion.resources import ResourceEmbedder
from src.llm.client import OpenAICompatibleLLM
from src.utils.config import get_settings


@lru_cache
def get_llm() -> OpenAICompatibleLLM:
    """Melempar LLMNotConfiguredError jika .env belum diisi (tidak di-cache, jadi dicoba lagi nanti)."""
    return OpenAICompatibleLLM(get_settings())


@lru_cache
def get_extractor() -> JobExtractor:
    s = get_settings()
    cfg = s.pipeline.get("extraction", {})
    return JobExtractor(
        get_llm(),
        max_retries=cfg.get("max_retries", 2),
        max_input_chars=cfg.get("max_input_chars", 12000),
        temperature=cfg.get("temperature", 0.0),
        max_tokens=cfg.get("max_tokens", 1500),
    )


def build_processor(pool: AsyncConnectionPool) -> JobProcessor:
    p = get_settings().pipeline
    return JobProcessor(
        pool,
        get_extractor,
        get_embedder,
        chunk_size=p.get("chunking", {}).get("chunk_size", 800),
        chunk_overlap=p.get("chunking", {}).get("chunk_overlap", 100),
        concurrency=p.get("ingestion", {}).get("concurrency", 2),
    )


def build_resource_embedder(pool: AsyncConnectionPool) -> ResourceEmbedder:
    return ResourceEmbedder(pool, get_embedder)


def build_chat_service(pool: AsyncConnectionPool) -> "ChatService":
    from src.chat.service import ChatService
    from src.retrieval.hybrid import HybridRetriever
    from src.utils.tracing import LogTracer

    p = get_settings().pipeline
    r, llm_cfg = p.get("retrieval", {}), p.get("llm", {})
    retriever = HybridRetriever(
        pool, get_embedder,
        top_k=r.get("top_k", 6), rrf_k=r.get("rrf_k", 60),
        vector_candidates=r.get("vector_candidates", 30), keyword_candidates=r.get("keyword_candidates", 30),
    )
    return ChatService(retriever, get_llm, LogTracer(),
                       max_tokens=llm_cfg.get("max_tokens", 1024), temperature=llm_cfg.get("temperature", 0.2))
