"""Merakit JobProcessor dari konfigurasi."""

from functools import lru_cache

from psycopg_pool import AsyncConnectionPool

from src.embeddings.embedder import get_embedder
from src.extraction.extractor import JobExtractor
from src.ingestion.pipeline import JobProcessor
from src.llm.client import OpenAICompatibleLLM
from src.utils.config import get_settings


@lru_cache
def get_extractor() -> JobExtractor:
    s = get_settings()
    cfg = s.pipeline.get("extraction", {})
    return JobExtractor(
        OpenAICompatibleLLM(s),
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
