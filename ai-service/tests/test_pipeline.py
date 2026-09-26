"""Test integrasi pipeline dan endpoint /internal dengan database test, LLM dan embedder palsu."""

from uuid import UUID, uuid4

import psycopg
import pytest
from pgvector.psycopg import register_vector_async
from psycopg_pool import AsyncConnectionPool

from src.extraction.extractor import JobExtractor
from src.ingestion.pipeline import JobProcessor
from tests.fakes import FakeEmbedder, FakeLLM

RAW = """Kami mencari AI Engineer.

Kualifikasi:
- Python, PostgreSQL
- Pengalaman membangun RAG
- Nilai plus: Docker, LangSmith
"""

EXTRACTED = {
    "role": "ai-engineer",
    "level": "mid",
    "location": "Jakarta",
    "work_type": "remote",
    "required_skills": ["Python", "Postgres", "RAG"],
    "preferred_skills": ["Docker", "LangSmith"],
}


def insert_job(db_url: str, **overrides) -> UUID:
    row = {"title": "AI Engineer", "company": "PT Contoh", "source_name": "Glints", "raw_text": RAW,
           "work_type": None} | overrides
    with psycopg.connect(db_url) as conn:
        return conn.execute(
            """INSERT INTO job_postings (title, company, source_name, raw_text, work_type)
               VALUES (%(title)s, %(company)s, %(source_name)s, %(raw_text)s, %(work_type)s) RETURNING id""",
            row,
        ).fetchone()[0]


async def make_pool(db_url: str) -> AsyncConnectionPool:
    pool = AsyncConnectionPool(db_url, min_size=1, max_size=3, open=False, configure=register_vector_async)
    await pool.open()
    return pool


def make_processor(pool, llm: FakeLLM) -> JobProcessor:
    extractor = JobExtractor(llm, max_retries=1)
    embedder = FakeEmbedder()
    return JobProcessor(pool, lambda: extractor, lambda: embedder, chunk_size=200, chunk_overlap=40)


def fetch(db_url: str, sql: str, *params):
    with psycopg.connect(db_url) as conn:
        return conn.execute(sql, params).fetchall()


@pytest.mark.anyio
async def test_process_job_end_to_end(db_url):
    job_id = insert_job(db_url, work_type="onsite")
    pool = await make_pool(db_url)
    try:
        outcome = await make_processor(pool, FakeLLM([EXTRACTED])).process(job_id)
    finally:
        await pool.close()

    assert outcome.status == "done" and outcome.skills == 4 and outcome.unmapped == 1

    (status, role, level, location, work_type), = fetch(
        db_url,
        """SELECT j.extraction_status, r.slug, j.level, j.location, j.work_type
           FROM job_postings j LEFT JOIN roles r ON r.id = j.role_id WHERE j.id = %s""",
        job_id,
    )
    # work_type yang diisi admin dipertahankan; field kosong diisi hasil ekstraksi
    assert (status, role, level, location, work_type) == ("done", "ai-engineer", "mid", "Jakarta", "onsite")

    skills = dict(fetch(db_url, """SELECT s.slug, js.requirement_type FROM job_skills js
                                   JOIN skills s ON s.id = js.skill_id WHERE js.job_id = %s""", job_id))
    assert skills == {"python": "required", "postgresql": "required", "rag": "required", "docker": "preferred"}
    assert fetch(db_url, "SELECT raw_name FROM unmapped_skills WHERE job_id = %s", job_id) == [("LangSmith",)]

    chunks = fetch(db_url, """SELECT chunk_index, section, vector_dims(embedding), metadata->>'title', tsv IS NOT NULL
                              FROM document_chunks WHERE source_id = %s ORDER BY chunk_index""", job_id)
    assert chunks and all(c[2] == 768 and c[3] == "AI Engineer" and c[4] for c in chunks)
    assert {c[1] for c in chunks} >= {"overview", "qualifications"}

    demand = dict(fetch(db_url, """SELECT s.slug, d.demand_pct FROM skill_demand d
                                   JOIN skills s ON s.id = d.skill_id""", ))
    assert demand["python"] == 100 and len(demand) == 4


@pytest.mark.anyio
async def test_reprocess_is_idempotent_and_keeps_reviewed_unmapped(db_url):
    job_id = insert_job(db_url)
    pool = await make_pool(db_url)
    try:
        await make_processor(pool, FakeLLM([EXTRACTED])).process(job_id)
        with psycopg.connect(db_url) as conn:
            conn.execute("UPDATE unmapped_skills SET status = 'ignored' WHERE job_id = %s", (job_id,))
        second = EXTRACTED | {"required_skills": ["Python"], "preferred_skills": ["LangSmith", "Airflow"]}
        await make_processor(pool, FakeLLM([second])).process(job_id)
    finally:
        await pool.close()

    assert fetch(db_url, "SELECT count(*) FROM job_skills WHERE job_id = %s", job_id) == [(1,)]
    unmapped = fetch(db_url, "SELECT raw_name, status FROM unmapped_skills WHERE job_id = %s ORDER BY raw_name", job_id)
    assert unmapped == [("Airflow", "pending"), ("LangSmith", "ignored")]
    n_chunks = fetch(db_url, "SELECT count(DISTINCT chunk_index), count(*) FROM document_chunks WHERE source_id = %s", job_id)
    assert n_chunks[0][0] == n_chunks[0][1]  # tidak ada chunk ganda


@pytest.mark.anyio
async def test_non_ai_role_goes_to_review(db_url):
    job_id = insert_job(db_url)
    pool = await make_pool(db_url)
    try:
        outcome = await make_processor(pool, FakeLLM([{"role": "other", "required_skills": ["Excel"]}])).process(job_id)
    finally:
        await pool.close()
    assert outcome.status == "review"
    assert fetch(db_url, "SELECT count(*) FROM skill_demand") == [(0,)]


@pytest.mark.anyio
async def test_failure_marks_job_failed(db_url):
    job_id = insert_job(db_url)
    pool = await make_pool(db_url)
    try:
        processor = make_processor(pool, FakeLLM(["rusak", "masih rusak"]))
        assert processor.claim(job_id)
        assert await processor.run_claimed(job_id) is None
        assert processor.claim(job_id), "claim harus dilepas setelah gagal"
    finally:
        await pool.close()
    (status, err), = fetch(db_url, "SELECT extraction_status, extraction_error FROM job_postings WHERE id = %s", job_id)
    assert status == "failed" and "ExtractionError" in err


def test_internal_endpoint(db_url):
    from fastapi.testclient import TestClient

    from main import app
    from src.utils.db import get_pool

    job_id = insert_job(db_url)
    with TestClient(app) as client:
        app.state.processor = make_processor(get_pool(), FakeLLM([EXTRACTED]))
        url = f"/internal/jobs/{job_id}/process"

        assert client.post(url).status_code == 401
        assert client.post(url, headers={"X-Internal-Token": "salah"}).status_code == 401

        auth = {"X-Internal-Token": "test-internal-token"}
        assert client.post(f"/internal/jobs/{uuid4()}/process", headers=auth).status_code == 404
        assert client.post("/internal/jobs/bukan-uuid/process", headers=auth).status_code == 422

        r = client.post(url, headers=auth)
        assert r.status_code == 202 and r.json()["status"] == "accepted"

    # TestClient menjalankan background task sebelum keluar dari request
    assert fetch(db_url, "SELECT extraction_status FROM job_postings WHERE id = %s", job_id) == [("done",)]
