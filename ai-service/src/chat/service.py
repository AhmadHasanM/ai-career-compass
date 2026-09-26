"""Chat RAG: hybrid retrieval -> prompt bersumber bernomor -> stream jawaban -> sitasi yang benar-benar dipakai."""

from __future__ import annotations

import re
import time
from collections.abc import AsyncIterator, Callable
from datetime import date
from typing import Literal

from pydantic import BaseModel, Field

from src.llm.client import StreamingChatModel
from src.prompts.chat import strip_chunk_header, build_messages, format_market
from src.retrieval.hybrid import HybridRetriever
from src.utils.tracing import Tracer

EXCERPT_CHARS = 240
# Pertanyaan lanjutan pendek ("kalau yang senior?") dicari bersama pertanyaan sebelumnya.
FOLLOW_UP_MAX_WORDS = 8


class HistoryItem(BaseModel):
    role: Literal["user", "assistant"]
    content: str = Field(max_length=8000)


class MarketItem(BaseModel):
    name: str
    demand_pct: float
    required_pct: float


class MarketData(BaseModel):
    role: str
    total_jobs: int
    snapshot_date: date | None = None
    small_sample: bool = False
    items: list[MarketItem] = Field(default_factory=list)


class UserContext(BaseModel):
    education: str | None = None
    current_job: str | None = None
    target_role: str | None = None
    hours_per_week: int | None = None
    skills: list[str] = Field(default_factory=list)
    gaps: list[str] = Field(default_factory=list)
    roadmap: list[str] = Field(default_factory=list)


class ChatRequest(BaseModel):
    question: str = Field(min_length=1, max_length=2000)
    history: list[HistoryItem] = Field(default_factory=list, max_length=12)
    user: UserContext | None = None
    market: MarketData | None = None


_CITE = re.compile(r"\[(\d+(?:\s*,\s*\d+)*)\]")


def parse_citations(answer: str, n_sources: int) -> list[int]:
    """Nomor sumber yang disitir di jawaban ([2], [1][3], [1, 3]), unik, urut kemunculan, dalam rentang."""
    out: list[int] = []
    for group in _CITE.findall(answer):
        for num in group.split(","):
            n = int(num)
            if 1 <= n <= n_sources and n not in out:
                out.append(n)
    return out


def retrieval_query(req: ChatRequest) -> str:
    prev = next((h.content for h in reversed(req.history) if h.role == "user"), None)
    if prev and len(req.question.split()) < FOLLOW_UP_MAX_WORDS:
        return f"{prev} {req.question}"
    return req.question


def citation(n: int, src: dict) -> dict:
    if src["source_type"] == "market_data":
        return {"n": n, "source_type": "market_data", "chunk_id": None, "source_id": None,
                "title": src["title"], "section": None, "excerpt": src["text"][:EXCERPT_CHARS], "url": None,
                "meta": src["meta"]}
    meta = src.get("metadata") or {}
    body = " ".join(strip_chunk_header(src["content"]).split())
    keep = ("company", "source_name", "posted_date", "role") if src["source_type"] == "job_posting" \
        else ("type", "level", "language", "is_free", "skill_name")
    return {
        "n": n,
        "source_type": src["source_type"],
        "chunk_id": str(src["chunk_id"]),
        "source_id": str(src["source_id"]),
        "title": meta.get("title"),
        "section": src.get("section"),
        "excerpt": body[:EXCERPT_CHARS] + ("…" if len(body) > EXCERPT_CHARS else ""),
        "url": meta.get("source_url") or meta.get("url"),
        "meta": {k: meta[k] for k in keep if meta.get(k) is not None},
    }


class ChatService:
    def __init__(self, retriever: HybridRetriever, llm_factory: Callable[[], StreamingChatModel],
                 tracer: Tracer, *, max_tokens: int = 1024, temperature: float = 0.2) -> None:
        self.retriever = retriever
        self._llm_factory = llm_factory
        self.tracer = tracer
        self.max_tokens, self.temperature = max_tokens, temperature

    def ensure_llm(self) -> None:
        """Melempar LLMNotConfiguredError jika LLM belum dikonfigurasi (dicek sebelum stream dimulai)."""
        self._llm_factory()

    async def stream(self, req: ChatRequest) -> AsyncIterator[tuple[str, dict]]:
        """Menghasilkan event SSE: ("token", {text}) berulang, lalu satu ("done", {...})."""
        started = time.perf_counter()
        llm = self._llm_factory()
        retrieved = await self.retriever.retrieve(retrieval_query(req))

        sources: list[dict] = [
            {"source_type": r.hit.source_type, "chunk_id": r.hit.chunk_id, "source_id": r.hit.source_id,
             "section": r.hit.section, "content": r.hit.content, "metadata": r.hit.metadata}
            for r in retrieved
        ]
        if req.market and req.market.total_jobs > 0 and req.market.items:
            market = req.market.model_dump(mode="json")
            sources.append({
                "source_type": "market_data",
                "title": f"Statistik skill demand {req.market.role}",
                "text": format_market(market),
                "meta": {"total_jobs": req.market.total_jobs, "snapshot_date": market["snapshot_date"],
                         "small_sample": req.market.small_sample},
            })

        messages = build_messages(
            question=req.question, sources=sources,
            user=req.user.model_dump() if req.user else None,
            history=[h.model_dump() for h in req.history],
        )

        parts: list[str] = []
        final = None
        first_token_ms = None
        async for delta in llm.chat_stream(messages, temperature=self.temperature, max_tokens=self.max_tokens):
            if delta.done:
                final = delta
            elif delta.text:
                if first_token_ms is None:
                    first_token_ms = int((time.perf_counter() - started) * 1000)
                parts.append(delta.text)
                yield "token", {"text": delta.text}

        answer = "".join(parts).strip()
        cited = parse_citations(answer, len(sources))
        latency_ms = int((time.perf_counter() - started) * 1000)
        done = {
            "answer": answer,
            "citations": [citation(n, sources[n - 1]) for n in cited],
            "retrieved_chunk_ids": [str(r.hit.chunk_id) for r in retrieved],
            "model": final.model if final else None,
            "usage": {
                "prompt_tokens": final.prompt_tokens if final else None,
                "completion_tokens": final.completion_tokens if final else None,
            },
            "latency_ms": latency_ms,
        }
        self.tracer.chat(
            question=req.question,
            retrieved=[{"chunk_id": str(r.hit.chunk_id), "source_type": r.hit.source_type,
                        "rrf": round(r.rrf_score, 5), "vector_rank": r.vector_rank, "keyword_rank": r.keyword_rank}
                       for r in retrieved],
            cited=cited, model=done["model"], usage=done["usage"],
            latency_ms=latency_ms, first_token_ms=first_token_ms,
        )
        yield "done", done
