"""Pencarian di document_chunks: vector similarity (pgvector) dan full-text (tsvector)."""

from __future__ import annotations

import re
from dataclasses import dataclass, field
from uuid import UUID

from pgvector import Vector

# Chunk lowongan hanya dipakai jika ekstraksinya selesai (bukan review/failed/pending).
_ACTIVE_SOURCE = """(c.source_type <> 'job_posting' OR EXISTS (
    SELECT 1 FROM job_postings j WHERE j.id = c.source_id AND j.extraction_status = 'done'))"""

_COLUMNS = "c.id, c.source_type, c.source_id, c.section, c.content, c.metadata"


@dataclass
class Hit:
    chunk_id: UUID
    source_type: str
    source_id: UUID
    section: str | None
    content: str
    metadata: dict = field(default_factory=dict)
    score: float = 0.0


async def vector_search(conn, query_vec: list[float], k: int) -> list[Hit]:
    """Top-k chunk berdasarkan cosine similarity (index HNSW)."""
    cur = await conn.execute(
        f"""SELECT {_COLUMNS}, 1 - (c.embedding <=> %s) AS score
            FROM document_chunks c
            WHERE c.embedding IS NOT NULL AND {_ACTIVE_SOURCE}
            ORDER BY c.embedding <=> %s
            LIMIT %s""",
        (Vector(query_vec), Vector(query_vec), k),
    )
    return [Hit(*row) for row in await cur.fetchall()]


# Kata umum Indonesia/Inggris yang tidak membantu pencarian kata kunci.
STOPWORDS = frozenset("""
yang dan di ke dari untuk dengan pada adalah ini itu apa apakah bagaimana gimana kenapa mengapa berapa
saya aku kamu anda kita kami mereka dia ada akan bisa dapat harus perlu sudah belum juga atau tapi
tetapi karena jika kalau agar supaya sebagai oleh dalam tentang seperti lebih paling sangat banyak
sedikit mau ingin jadi menjadi tidak bukan ya nya lah kah pun saja hanya masih lagi mana siapa kapan
the a an and or of to in on for with is are was were be been being what which who how why when where
do does did can could should would will i you we they it this that these those my your our as at by
from about into than then there their some any all more most much many not no yes
""".split())

_TOKEN = re.compile(r"[a-z0-9]+")


def build_tsquery(text: str, max_terms: int = 12) -> str | None:
    """Ubah pertanyaan bebas menjadi tsquery OR (kata a | kata b | ...), aman untuk to_tsquery.

    websearch_to_tsquery memakai AND sehingga pertanyaan panjang hampir tidak pernah cocok;
    OR + ts_rank_cd tetap memberi peringkat lebih tinggi pada chunk yang cocok dengan lebih banyak kata.
    """
    terms: list[str] = []
    for tok in _TOKEN.findall(text.lower()):
        if len(tok) >= 2 and tok not in STOPWORDS and tok not in terms:
            terms.append(tok)
    return " | ".join(terms[:max_terms]) or None


async def keyword_search(conn, query: str, k: int) -> list[Hit]:
    tsquery = build_tsquery(query)
    if tsquery is None:
        return []
    cur = await conn.execute(
        f"""SELECT {_COLUMNS}, ts_rank_cd(c.tsv, q) AS score
            FROM document_chunks c, to_tsquery('simple', %s) q
            WHERE c.tsv @@ q AND {_ACTIVE_SOURCE}
            ORDER BY score DESC, c.id
            LIMIT %s""",
        (tsquery, k),
    )
    return [Hit(*row) for row in await cur.fetchall()]
