"""Hybrid retrieval: vector + full-text digabung dengan Reciprocal Rank Fusion (RRF)."""

from __future__ import annotations

from collections.abc import Callable
from dataclasses import dataclass
from uuid import UUID

from psycopg_pool import AsyncConnectionPool

from src.embeddings.embedder import Embedder
from src.vectordb.search import Hit, keyword_search, vector_search


@dataclass
class RetrievedChunk:
    hit: Hit
    rrf_score: float
    vector_rank: int | None
    keyword_rank: int | None


def reciprocal_rank_fusion(rankings: list[list[Hit]], rrf_k: int = 60) -> list[tuple[Hit, float, list[int | None]]]:
    """skor(d) = Σ 1 / (rrf_k + rank_i(d)), rank mulai dari 1. Dokumen yang muncul di kedua daftar naik.

    Mengembalikan (hit, skor, rank per daftar) urut skor tertinggi; seri dipecah dengan urutan kemunculan.
    """
    scores: dict[UUID, float] = {}
    hits: dict[UUID, Hit] = {}
    ranks: dict[UUID, list[int | None]] = {}
    for li, ranking in enumerate(rankings):
        for rank, hit in enumerate(ranking, start=1):
            key = hit.chunk_id
            hits.setdefault(key, hit)
            ranks.setdefault(key, [None] * len(rankings))[li] = rank
            scores[key] = scores.get(key, 0.0) + 1.0 / (rrf_k + rank)
    order = sorted(scores, key=lambda cid: -scores[cid])  # sorted stabil: seri mengikuti kemunculan
    return [(hits[cid], scores[cid], ranks[cid]) for cid in order]


class HybridRetriever:
    def __init__(self, pool: AsyncConnectionPool, embedder_factory: Callable[[], Embedder], *,
                 top_k: int = 6, rrf_k: int = 60, vector_candidates: int = 30, keyword_candidates: int = 30) -> None:
        self.pool = pool
        self._embedder_factory = embedder_factory
        self.top_k, self.rrf_k = top_k, rrf_k
        self.vector_candidates, self.keyword_candidates = vector_candidates, keyword_candidates

    async def retrieve(self, query: str, top_k: int | None = None) -> list[RetrievedChunk]:
        query_vec = await self._embedder_factory().embed_query(query)
        async with self.pool.connection() as conn:
            vec_hits = await vector_search(conn, query_vec, self.vector_candidates)
            kw_hits = await keyword_search(conn, query, self.keyword_candidates)
        fused = reciprocal_rank_fusion([vec_hits, kw_hits], self.rrf_k)
        return [
            RetrievedChunk(hit=h, rrf_score=score, vector_rank=r[0], keyword_rank=r[1])
            for h, score, r in fused[: top_k or self.top_k]
        ]
