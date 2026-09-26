import psycopg
import pytest

from src.roadmap.explainer import ExplainNode, ExplainProfile, ExplainRequest, explain_roadmap, parse_explanation
from tests.fakes import FakeEmbedder, FakeLLM


def req(*ids: int) -> ExplainRequest:
    return ExplainRequest(
        role="AI Engineer",
        profile=ExplainProfile(hours_per_week=10, skills=["Docker"]),
        nodes=[ExplainNode(skill_id=i, name=f"Skill {i}", category="llm", stage="Tahap 1", demand_pct=50) for i in ids],
    )


def test_parse_explanation_filters_unknown_and_duplicate_ids():
    content = '{"nodes": [{"skill_id": 1, "rationale": "a", "est_weeks": 2}, {"skill_id": 9, "rationale": "x", "est_weeks": 1},' \
              ' {"skill_id": 1, "rationale": "dup", "est_weeks": 3}, {"skill_id": 2, "rationale": "b", "est_weeks": 4}]}'
    nodes = parse_explanation(content, {1, 2})
    assert [(n.skill_id, n.rationale) for n in nodes] == [(1, "a"), (2, "b")]


@pytest.mark.parametrize(
    "content,msg",
    [
        ('{"nodes": [{"skill_id": 9, "rationale": "x", "est_weeks": 1}]}', "tidak ada skill_id yang cocok"),
        ('{"nodes": [{"skill_id": 1, "rationale": "", "est_weeks": 1}]}', "rationale"),
        ('{"nodes": [{"skill_id": 1, "rationale": "a", "est_weeks": 0}]}', "est_weeks"),
    ],
)
def test_parse_explanation_errors(content, msg):
    with pytest.raises(ValueError, match=msg):
        parse_explanation(content, {1})


@pytest.mark.anyio
async def test_explain_roadmap_retries_and_sends_nodes_to_llm():
    llm = FakeLLM(["salah", {"nodes": [{"skill_id": 1, "rationale": "Fondasi.", "est_weeks": 2}]}])
    res = await explain_roadmap(llm, req(1, 2))
    assert res.attempts == 2 and res.data[0].rationale == "Fondasi."
    assert '"skill_id": 2' in llm.calls[0][1]["content"]


def test_explain_request_rejects_empty_nodes():
    with pytest.raises(ValueError):
        ExplainRequest(role="AI Engineer", profile=ExplainProfile(), nodes=[])


def insert_resource(db_url: str) -> str:
    with psycopg.connect(db_url) as conn:
        return conn.execute(
            """INSERT INTO learning_resources (skill_id, title, url, type, level, language, is_free, est_hours)
               VALUES ((SELECT id FROM skills WHERE slug = 'python'), 'Python Tutorial',
                       'https://docs.python.org/3/tutorial/', 'docs', 'beginner', 'en', true, 10)
               RETURNING id"""
        ).fetchone()[0]


def test_internal_roadmap_and_resource_endpoints(db_url):
    from fastapi.testclient import TestClient

    from main import app
    from src.api.roadmap import get_llm
    from src.ingestion.resources import ResourceEmbedder
    from src.utils.db import get_pool

    rid = insert_resource(db_url)
    auth = {"X-Internal-Token": "test-internal-token"}
    llm = FakeLLM([{"nodes": [{"skill_id": 1, "rationale": "Fondasi.", "est_weeks": 2}]}])
    app.dependency_overrides[get_llm] = lambda: llm
    try:
        with TestClient(app) as client:
            app.state.resource_embedder = ResourceEmbedder(get_pool(), FakeEmbedder)

            r = client.post("/internal/roadmap/explain", json=req(1).model_dump(), headers=auth)
            assert r.status_code == 200 and r.json() == {
                "model": "fake-llm", "nodes": [{"skill_id": 1, "rationale": "Fondasi.", "est_weeks": 2.0}]}
            assert client.post("/internal/roadmap/explain", json=req(1).model_dump()).status_code == 401

            assert client.post(f"/internal/resources/{rid}/embed", headers=auth).status_code == 202
            missing = "00000000-0000-0000-0000-000000000000"
            assert client.post(f"/internal/resources/{missing}/embed", headers=auth).status_code == 404
    finally:
        app.dependency_overrides.clear()

    with psycopg.connect(db_url) as conn:
        row = conn.execute(
            """SELECT section, vector_dims(embedding), metadata->>'skill_slug', content
               FROM document_chunks WHERE source_type = 'learning_resource' AND source_id = %s""", (rid,)
        ).fetchall()
    assert len(row) == 1
    section, dim, slug, content = row[0]
    assert (section, dim, slug) == ("resource", 768, "python")
    assert content.startswith("Python Tutorial\nMateri Python") and "±10 jam" in content


def test_roadmap_explain_returns_503_without_llm(db_url, monkeypatch):
    from fastapi.testclient import TestClient

    from main import app
    from src.ingestion import factory
    from src.llm.client import LLMNotConfiguredError

    def not_configured():
        raise LLMNotConfiguredError("LLM belum dikonfigurasi")

    monkeypatch.setattr(factory, "get_llm", not_configured)
    with TestClient(app) as client:
        r = client.post("/internal/roadmap/explain", json=req(1).model_dump(),
                        headers={"X-Internal-Token": "test-internal-token"})
    assert r.status_code == 503
