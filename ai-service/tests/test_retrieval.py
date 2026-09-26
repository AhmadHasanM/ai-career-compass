from uuid import uuid4

import psycopg
import pytest
from pgvector.psycopg import register_vector_async
from psycopg_pool import AsyncConnectionPool

from src.chunking.job_chunker import Chunk
from src.retrieval.hybrid import HybridRetriever, reciprocal_rank_fusion
from src.vectordb.search import Hit, build_tsquery, keyword_search, vector_search
from src.vectordb.store import replace_chunks
from tests.fakes import FakeEmbedder


def test_build_tsquery_drops_stopwords_and_uses_or():
    assert build_tsquery("Skill apa yang paling dicari untuk AI Engineer di Jakarta?") == \
        "skill | dicari | ai | engineer | jakarta"
    assert build_tsquery("apa yang?") is None
    assert build_tsquery("Python python PYTHON") == "python"
    # karakter berbahaya untuk to_tsquery tidak pernah lolos
    assert build_tsquery("rag & (docker | !k8s) 'x'") == "rag | docker | k8s"


def hit(name: str) -> Hit:
    return Hit(chunk_id=uuid4(), source_type="job_posting", source_id=uuid4(), section=None, content=name)


def test_rrf_boosts_documents_in_both_lists():
    a, b, c, d = hit("a"), hit("b"), hit("c"), hit("d")
    fused = reciprocal_rank_fusion([[a, b, c], [c, d]], rrf_k=60)
    names = [h.content for h, _, _ in fused]
    # c: 1/63 + 1/61 > a: 1/61 > d: 1/62 > b: 1/62 (seri: kemunculan pertama)
    assert names == ["c", "a", "b", "d"]
    ranks = {h.content: r for h, _, r in fused}
    assert ranks["c"] == [3, 1] and ranks["d"] == [None, 2]


def test_rrf_empty():
    assert reciprocal_rank_fusion([[], []]) == []


async def seed_chunks(db_url, embedder):
    pool = AsyncConnectionPool(db_url, min_size=1, max_size=2, open=False, configure=register_vector_async)
    await pool.open()
    with psycopg.connect(db_url) as conn:
        done_job = conn.execute("""INSERT INTO job_postings (title, source_name, raw_text, extraction_status)
                                   VALUES ('AI Engineer', 't', 'x', 'done') RETURNING id""").fetchone()[0]
        review_job = conn.execute("""INSERT INTO job_postings (title, source_name, raw_text, extraction_status)
                                     VALUES ('Akuntan', 't', 'x', 'review') RETURNING id""").fetchone()[0]
    texts = {
        done_job: ["Kualifikasi: menguasai Python, FastAPI, dan pipeline RAG", "Benefit: asuransi dan remote"],
        review_job: ["Kualifikasi: menguasai Python dan Excel untuk akuntansi"],
    }
    async with pool.connection() as conn:
        for job_id, ts in texts.items():
            chunks = [Chunk(index=i, section="qualifications", content=t) for i, t in enumerate(ts)]
            await replace_chunks(conn, source_type="job_posting", source_id=job_id, chunks=chunks,
                                 embeddings=await embedder.embed_documents(ts), metadata={"title": "t"})
    return pool, done_job, review_job


@pytest.mark.anyio
async def test_search_and_hybrid_exclude_non_done_jobs(db_url):
    embedder = FakeEmbedder()
    pool, done_job, review_job = await seed_chunks(db_url, embedder)
    try:
        async with pool.connection() as conn:
            kw = await keyword_search(conn, "belajar python dan RAG", 10)
            vec = await vector_search(conn, await embedder.embed_query("x"), 10)
        assert kw and kw[0].content.startswith("Kualifikasi: menguasai Python, FastAPI")
        assert all(h.source_id == done_job for h in kw + vec), "chunk lowongan berstatus review tidak boleh ikut"
        assert len(vec) == 2

        # Query di-embed persis seperti chunk target -> vektor identik -> peringkat vector 1.
        target = "Benefit: asuransi dan remote"

        class TargetEmbedder(FakeEmbedder):
            async def embed_query(self, text):
                return self._vec(target)

        retriever = HybridRetriever(pool, TargetEmbedder, top_k=1)
        [top] = await retriever.retrieve("benefit asuransi")
        assert top.hit.content == target and top.vector_rank == 1 and top.keyword_rank == 1
    finally:
        await pool.close()
