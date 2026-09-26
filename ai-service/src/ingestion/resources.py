"""Embedding sumber belajar ke document_chunks agar bisa dicari chatbot."""

from __future__ import annotations

import logging
from collections.abc import Callable
from uuid import UUID

from psycopg_pool import AsyncConnectionPool

from src.chunking.job_chunker import Chunk
from src.embeddings.embedder import Embedder
from src.vectordb.store import replace_chunks

log = logging.getLogger(__name__)

_TYPE_LABEL = {"course": "Kursus", "docs": "Dokumentasi", "video": "Video", "article": "Artikel",
               "book": "Buku", "tutorial": "Tutorial"}
_LEVEL_LABEL = {"beginner": "pemula", "intermediate": "menengah", "advanced": "lanjutan"}


def resource_content(r: dict) -> str:
    """Teks yang di-embed dan diindeks full-text.

    Judul, skill, alias, dan deskripsi skill di depan karena itulah pembeda antar sumber; metadata
    (jenis, level, bahasa, biaya) dijadikan satu baris pendek agar tidak mendominasi embedding.
    """
    aliases = [a for a in r.get("aliases") or [] if a not in (r["skill_name"].lower(), r["skill_slug"])][:8]
    skill = r["skill_name"] + (f" ({', '.join(aliases)})" if aliases else "")
    lines = [r["title"], f"Materi {skill}."]
    if r.get("skill_description"):
        lines.append(r["skill_description"])
    meta = [_TYPE_LABEL.get(r["type"], r["type"])]
    if r.get("level"):
        meta.append(_LEVEL_LABEL.get(r["level"], r["level"]))
    meta.append("bahasa Indonesia" if r["language"] == "id" else "bahasa Inggris")
    meta.append("gratis" if r["is_free"] else "berbayar")
    if r.get("est_hours"):
        meta.append(f"±{r['est_hours']:g} jam")
    lines.append(" · ".join(meta))
    return "\n".join(lines)


class ResourceEmbedder:
    def __init__(self, pool: AsyncConnectionPool, embedder_factory: Callable[[], Embedder]) -> None:
        self.pool = pool
        self._embedder_factory = embedder_factory

    async def exists(self, resource_id: UUID) -> bool:
        async with self.pool.connection() as conn:
            cur = await conn.execute("SELECT 1 FROM learning_resources WHERE id = %s", (resource_id,))
            return await cur.fetchone() is not None

    async def embed(self, resource_id: UUID) -> int:
        async with self.pool.connection() as conn:
            cur = await conn.execute(
                """SELECT r.title, r.url, r.type, r.level, r.language, r.is_free, r.est_hours::float8,
                          s.name, s.slug, s.category, s.id, s.description,
                          COALESCE((SELECT array_agg(a.alias ORDER BY a.alias) FROM skill_aliases a
                                    WHERE a.skill_id = s.id), '{}')
                   FROM learning_resources r JOIN skills s ON s.id = r.skill_id WHERE r.id = %s""",
                (resource_id,),
            )
            row = await cur.fetchone()
        if row is None:
            raise LookupError(f"sumber belajar {resource_id} tidak ditemukan")
        cols = ["title", "url", "type", "level", "language", "is_free", "est_hours",
                "skill_name", "skill_slug", "category", "skill_id", "skill_description", "aliases"]
        r = dict(zip(cols, row))

        chunk = Chunk(index=0, section="resource", content=resource_content(r))
        [vector] = await self._embedder_factory().embed_documents([chunk.content])
        metadata = {k: r[k] for k in ("title", "url", "type", "level", "language", "is_free",
                                      "skill_id", "skill_name", "skill_slug")}
        async with self.pool.connection() as conn:
            async with conn.transaction():
                return await replace_chunks(conn, source_type="learning_resource", source_id=resource_id,
                                            chunks=[chunk], embeddings=[vector], metadata=metadata)

    async def run(self, resource_id: UUID) -> None:
        try:
            await self.embed(resource_id)
            log.info("sumber belajar di-embed", extra={"resource_id": str(resource_id)})
        except Exception:  # noqa: BLE001 - background task: cukup dicatat, bisa di-reindex nanti
            log.exception("embedding sumber belajar %s gagal", resource_id)
