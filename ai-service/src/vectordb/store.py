"""Operasi document_chunks (pgvector)."""

from __future__ import annotations

import json
from uuid import UUID

from pgvector import Vector

from src.chunking.job_chunker import JobChunk


async def replace_chunks(
    conn,
    *,
    source_type: str,
    source_id: UUID,
    chunks: list[JobChunk],
    embeddings: list[list[float]],
    metadata: dict,
) -> int:
    """Ganti semua chunk milik satu sumber (idempotent saat diproses ulang). Dipanggil di dalam transaksi."""
    if len(chunks) != len(embeddings):
        raise ValueError("jumlah chunk dan embedding berbeda")
    await conn.execute(
        "DELETE FROM document_chunks WHERE source_type = %s AND source_id = %s", (source_type, source_id)
    )
    if not chunks:
        return 0
    async with conn.cursor() as cur:
        await cur.executemany(
            """INSERT INTO document_chunks (source_type, source_id, chunk_index, section, content, embedding, metadata)
               VALUES (%s, %s, %s, %s, %s, %s, %s)""",
            [
                (source_type, source_id, c.index, c.section, c.content, Vector(e),
                 json.dumps({**metadata, "section": c.section}))
                for c, e in zip(chunks, embeddings)
            ],
        )
    return len(chunks)
