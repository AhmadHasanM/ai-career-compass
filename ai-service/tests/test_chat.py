import json
from uuid import uuid4

import pytest

from src.chat.service import ChatRequest, ChatService, HistoryItem, parse_citations, retrieval_query
from src.prompts.chat import build_messages, format_market
from src.retrieval.hybrid import RetrievedChunk
from src.vectordb.search import Hit
from tests.fakes import FakeStreamingLLM


def test_parse_citations_formats_and_range():
    ans = "Python paling dicari [2]. RAG juga [1][3], lihat [3, 5] dan [9]."
    assert parse_citations(ans, 5) == [2, 1, 3, 5]
    assert parse_citations("tanpa sitasi", 3) == []


def test_retrieval_query_merges_short_follow_up():
    base = ChatRequest(question="kalau yang senior?", history=[
        HistoryItem(role="user", content="Skill apa yang dicari untuk AI Engineer junior?"),
        HistoryItem(role="assistant", content="..."),
    ])
    assert retrieval_query(base) == "Skill apa yang dicari untuk AI Engineer junior? kalau yang senior?"
    long_q = base.model_copy(update={"question": "Apa perbedaan kebutuhan skill antara junior dan senior AI Engineer di Jakarta?"})
    assert retrieval_query(long_q) == long_q.question


def test_build_messages_numbers_sources_and_adds_profile():
    sources = [
        {"source_type": "job_posting", "section": "qualifications", "metadata": {"title": "AI Engineer", "company": "PT A"},
         "content": "AI Engineer — PT A\nBagian: Kualifikasi\n\nMenguasai Python dan RAG"},
        {"source_type": "market_data", "text": "Statistik ..."},
    ]
    msgs = build_messages(question="Q?", sources=sources, history=[{"role": "user", "content": "halo"}],
                          user={"current_job": "Backend Dev", "skills": ["Python"], "gaps": ["RAG"]})
    system = msgs[0]["content"]
    assert "[1] (Lowongan) AI Engineer — PT A · bagian: Kualifikasi\nMenguasai Python dan RAG" in system
    assert "[2] (Data pasar) Statistik ..." in system
    assert "Skill yang belum dimiliki (prioritas): RAG" in system
    assert [m["role"] for m in msgs] == ["system", "user", "user"] and msgs[-1]["content"] == "Q?"


def test_format_market_marks_small_sample():
    text = format_market({"role": "AI Engineer", "total_jobs": 8, "snapshot_date": "2026-09-20", "small_sample": True,
                          "items": [{"name": "Python", "demand_pct": 87.5, "required_pct": 75.0}]})
    assert "dari 8 lowongan, snapshot 2026-09-20. SAMPEL KECIL." in text and "Python 87.5% (wajib 75%)" in text


class FakeRetriever:
    def __init__(self, hits):
        self.hits, self.queries = hits, []

    async def retrieve(self, query, top_k=None):
        self.queries.append(query)
        return [RetrievedChunk(hit=h, rrf_score=0.03, vector_rank=i + 1, keyword_rank=None) for i, h in enumerate(self.hits)]


class ListTracer:
    def __init__(self):
        self.records = []

    def chat(self, **fields):
        self.records.append(fields)


def job_hit(title, text, url="https://example.com/job"):
    return Hit(chunk_id=uuid4(), source_type="job_posting", source_id=uuid4(), section="qualifications",
               content=f"{title}\nBagian: Kualifikasi\n\n{text}",
               metadata={"title": title, "company": "PT A", "source_url": url, "posted_date": "2026-09-01"})


@pytest.mark.anyio
async def test_chat_stream_emits_tokens_then_done_with_only_cited_sources():
    hits = [job_hit("AI Engineer", "Python dan RAG"), job_hit("ML Engineer", "PyTorch")]
    llm = FakeStreamingLLM(["Python wajib ", "[1]. Data pasar: 90% ", "[3]."])
    tracer = ListTracer()
    svc = ChatService(FakeRetriever(hits), lambda: llm, tracer)
    req = ChatRequest(question="Skill apa yang dicari?", market={
        "role": "AI Engineer", "total_jobs": 10, "snapshot_date": "2026-09-20",
        "items": [{"name": "Python", "demand_pct": 90, "required_pct": 80}]})

    events = [e async for e in svc.stream(req)]
    assert [e for e, _ in events] == ["token", "token", "token", "done"]
    done = events[-1][1]
    assert done["answer"] == "Python wajib [1]. Data pasar: 90% [3]."
    assert [c["n"] for c in done["citations"]] == [1, 3]
    job_cite, market_cite = done["citations"]
    assert job_cite["title"] == "AI Engineer" and job_cite["url"] == "https://example.com/job"
    assert job_cite["excerpt"] == "Python dan RAG" and job_cite["meta"]["company"] == "PT A"
    assert market_cite["source_type"] == "market_data" and market_cite["meta"]["total_jobs"] == 10
    assert done["usage"] == {"prompt_tokens": 321, "completion_tokens": 45} and done["model"] == "fake-llm"
    assert len(done["retrieved_chunk_ids"]) == 2
    assert tracer.records and tracer.records[0]["cited"] == [1, 3]
    assert "[3] (Data pasar)" in llm.stream_calls[0][0]["content"]


@pytest.mark.anyio
async def test_chat_without_market_data_has_no_market_source():
    llm = FakeStreamingLLM(["Maaf, saya hanya membantu seputar karier tech."])
    svc = ChatService(FakeRetriever([]), lambda: llm, ListTracer())
    events = [e async for e in svc.stream(ChatRequest(question="Resep rendang?"))]
    assert events[-1][1]["citations"] == []
    assert "(tidak ada sumber relevan)" in llm.stream_calls[0][0]["content"]


def parse_sse(text: str) -> list[tuple[str, dict]]:
    out = []
    for block in text.strip().split("\n\n"):
        lines = dict(line.split(": ", 1) for line in block.splitlines())
        out.append((lines["event"], json.loads(lines["data"])))
    return out


def test_internal_chat_endpoint_streams_sse(db_url):
    from fastapi.testclient import TestClient

    from main import app

    auth = {"X-Internal-Token": "test-internal-token"}
    with TestClient(app) as client:
        app.state.chat_service = ChatService(FakeRetriever([job_hit("AI Engineer", "Python")]),
                                             lambda: FakeStreamingLLM(["Halo ", "[1]"]), ListTracer())
        with client.stream("POST", "/internal/chat", json={"question": "Halo?"}, headers=auth) as r:
            assert r.status_code == 200 and r.headers["content-type"].startswith("text/event-stream")
            events = parse_sse("".join(r.iter_text()))
        assert [e for e, _ in events] == ["token", "token", "done"]
        assert events[-1][1]["citations"][0]["n"] == 1

        assert client.post("/internal/chat", json={"question": "x"}).status_code == 401
        assert client.post("/internal/chat", json={"question": ""}, headers=auth).status_code == 422

        def broken():
            from src.llm.client import LLMNotConfiguredError
            raise LLMNotConfiguredError("belum dikonfigurasi")

        app.state.chat_service = ChatService(FakeRetriever([]), broken, ListTracer())
        assert client.post("/internal/chat", json={"question": "x"}, headers=auth).status_code == 503


def test_internal_chat_reports_mid_stream_errors(db_url):
    from fastapi.testclient import TestClient

    from main import app

    class FailingRetriever:
        async def retrieve(self, query, top_k=None):
            raise RuntimeError("database mati")

    with TestClient(app) as client:
        app.state.chat_service = ChatService(FailingRetriever(), lambda: FakeStreamingLLM([]), ListTracer())
        with client.stream("POST", "/internal/chat", json={"question": "x"},
                           headers={"X-Internal-Token": "test-internal-token"}) as r:
            events = parse_sse("".join(r.iter_text()))
    assert events == [("error", {"code": "internal_error", "message": "Terjadi kesalahan saat menyusun jawaban."})]
