"""Pipeline satu lowongan: ekstraksi LLM -> normalisasi skill -> chunking -> embedding -> simpan -> refresh demand.

Status akhir di job_postings:
- done    : role AI/ML dikenali dan minimal satu skill cocok dengan taxonomy
- review  : role "other" atau tidak ada skill yang cocok (perlu dicek admin)
- failed  : error (LLM tidak dikonfigurasi, output tetap tidak valid, DB, dll.); pesan di extraction_error
"""

from __future__ import annotations

import asyncio
import logging
import time
from collections.abc import Callable
from dataclasses import dataclass
from uuid import UUID

from psycopg_pool import AsyncConnectionPool

from src.chunking.job_chunker import chunk_job
from src.embeddings.embedder import Embedder
from src.extraction.extractor import JobExtractor
from src.extraction.normalize import SkillNormalizer, load_alias_map
from src.vectordb.store import replace_chunks

log = logging.getLogger(__name__)

MAX_ERROR_CHARS = 1000


class JobNotFoundError(LookupError):
    pass


@dataclass
class ProcessOutcome:
    job_id: UUID
    status: str
    skills: int
    unmapped: int
    chunks: int


class JobProcessor:
    def __init__(
        self,
        pool: AsyncConnectionPool,
        extractor_factory: Callable[[], JobExtractor],
        embedder_factory: Callable[[], Embedder],
        *,
        chunk_size: int = 800,
        chunk_overlap: int = 100,
        concurrency: int = 2,
    ) -> None:
        self.pool = pool
        # Factory dipanggil saat dibutuhkan: LLM yang belum dikonfigurasi membuat job "failed",
        # bukan membuat service gagal start.
        self._extractor_factory = extractor_factory
        self._embedder_factory = embedder_factory
        self.chunk_size, self.chunk_overlap = chunk_size, chunk_overlap
        self._sem = asyncio.Semaphore(concurrency)
        self._in_flight: set[UUID] = set()

    async def job_exists(self, job_id: UUID) -> bool:
        async with self.pool.connection() as conn:
            cur = await conn.execute("SELECT 1 FROM job_postings WHERE id = %s", (job_id,))
            return await cur.fetchone() is not None

    def claim(self, job_id: UUID) -> bool:
        """Tandai job sedang diproses; False jika sudah ada proses berjalan untuk job ini."""
        if job_id in self._in_flight:
            return False
        self._in_flight.add(job_id)
        return True

    async def run_claimed(self, job_id: UUID) -> ProcessOutcome | None:
        """Jalankan job yang sudah di-claim. Tidak melempar error: kegagalan dicatat sebagai status failed."""
        try:
            async with self._sem:
                return await self.process(job_id)
        except Exception as e:  # noqa: BLE001 - semua kegagalan harus tercatat di job
            log.exception("proses job %s gagal", job_id)
            await self._mark_failed(job_id, e)
            return None
        finally:
            self._in_flight.discard(job_id)

    async def process(self, job_id: UUID) -> ProcessOutcome:
        started = time.perf_counter()
        async with self.pool.connection() as conn:
            cur = await conn.execute(
                """SELECT title, company, raw_text, source_name, source_url, posted_date
                   FROM job_postings WHERE id = %s""",
                (job_id,),
            )
            row = await cur.fetchone()
            if row is None:
                raise JobNotFoundError(str(job_id))
            title, company, raw_text, source_name, source_url, posted_date = row
            cur = await conn.execute("SELECT id, slug FROM roles ORDER BY id")
            role_ids = {slug: rid for rid, slug in await cur.fetchall()}
            normalizer = SkillNormalizer(await load_alias_map(conn))

        extraction = await self._extractor_factory().extract(
            title=title, company=company, raw_text=raw_text, roles=list(role_ids)
        )
        data = extraction.data
        normalized = normalizer.normalize(data.required_skills, data.preferred_skills)

        chunks = chunk_job(title=title, company=company, raw_text=raw_text,
                           size=self.chunk_size, overlap=self.chunk_overlap)
        embedder = self._embedder_factory()
        embeddings = await embedder.embed_documents([c.content for c in chunks])

        role_id = role_ids.get(data.role)
        status = "done" if role_id is not None and normalized.skills else "review"
        metadata = {
            "job_id": str(job_id),
            "title": title,
            "company": company,
            "source_name": source_name,
            "source_url": source_url,
            "posted_date": posted_date.isoformat() if posted_date else None,
            "role": data.role,
        }

        async with self.pool.connection() as conn:
            async with conn.transaction():
                await conn.execute("DELETE FROM job_skills WHERE job_id = %s", (job_id,))
                if normalized.skills:
                    async with conn.cursor() as cur:
                        await cur.executemany(
                            "INSERT INTO job_skills (job_id, skill_id, requirement_type) VALUES (%s, %s, %s)",
                            [(job_id, sid, req) for sid, req in normalized.skills.items()],
                        )
                # Unmapped yang sudah direview admin dipertahankan; yang masih pending diganti hasil terbaru.
                await conn.execute(
                    "DELETE FROM unmapped_skills WHERE job_id = %s AND status = 'pending'", (job_id,)
                )
                for raw_name in normalized.unmapped:
                    await conn.execute(
                        """INSERT INTO unmapped_skills (job_id, raw_name)
                           SELECT %s, %s WHERE NOT EXISTS (
                               SELECT 1 FROM unmapped_skills WHERE job_id = %s AND lower(raw_name) = lower(%s))""",
                        (job_id, raw_name, job_id, raw_name),
                    )
                n_chunks = await replace_chunks(
                    conn, source_type="job_posting", source_id=job_id,
                    chunks=chunks, embeddings=embeddings, metadata=metadata,
                )
                # Nilai yang diisi admin saat input dipertahankan; ekstraksi hanya mengisi yang kosong.
                await conn.execute(
                    """UPDATE job_postings SET
                           role_id = %s,
                           level = COALESCE(level, %s),
                           location = COALESCE(location, %s),
                           work_type = COALESCE(work_type, %s),
                           extraction_status = %s,
                           extraction_error = NULL,
                           updated_at = now()
                       WHERE id = %s""",
                    (role_id, data.level, data.location, data.work_type, status, job_id),
                )
        await self.refresh_skill_demand()

        outcome = ProcessOutcome(job_id, status, len(normalized.skills), len(normalized.unmapped), n_chunks)
        log.info(
            "job diproses",
            extra={
                "job_id": str(job_id), "status": status, "role": data.role,
                "skills": outcome.skills, "unmapped": outcome.unmapped, "chunks": n_chunks,
                "llm_model": extraction.model, "llm_attempts": extraction.attempts,
                "prompt_tokens": extraction.prompt_tokens, "completion_tokens": extraction.completion_tokens,
                "embedding_model": embedder.model_name,
                "latency_ms": int((time.perf_counter() - started) * 1000),
            },
        )
        return outcome

    async def refresh_skill_demand(self) -> None:
        async with self.pool.connection() as conn:
            await conn.execute("REFRESH MATERIALIZED VIEW CONCURRENTLY skill_demand")

    async def _mark_failed(self, job_id: UUID, err: Exception) -> None:
        msg = f"{type(err).__name__}: {err}"[:MAX_ERROR_CHARS]
        try:
            async with self.pool.connection() as conn:
                await conn.execute(
                    """UPDATE job_postings SET extraction_status = 'failed', extraction_error = %s, updated_at = now()
                       WHERE id = %s""",
                    (msg, job_id),
                )
            await self.refresh_skill_demand()
        except Exception:  # noqa: BLE001
            log.exception("gagal menandai job %s sebagai failed", job_id)
