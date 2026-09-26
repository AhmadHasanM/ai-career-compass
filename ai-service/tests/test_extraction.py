import pytest

from src.extraction.extractor import ExtractionError, JobExtractor, parse_extraction
from tests.fakes import FakeLLM

ROLES = ["ai-engineer", "ml-engineer"]
VALID = {
    "role": "ai-engineer",
    "level": "Mid",
    "location": "Jakarta",
    "work_type": "hybrid",
    "required_skills": ["Python", "python ", "RAG", ""],
    "preferred_skills": ["Docker", "Python"],
}


def test_parse_valid_and_cleans_lists():
    d = parse_extraction('```json\n{"role": "ai-engineer", "level": "Mid", "required_skills": ["Python", "python "]}\n```', ROLES)
    assert d.level == "mid"
    assert d.required_skills == ["Python"]


def test_parse_skill_in_both_lists_is_required():
    import json

    d = parse_extraction(json.dumps(VALID), ROLES)
    assert d.required_skills == ["Python", "RAG"]
    assert d.preferred_skills == ["Docker"]


def test_parse_tolerates_text_around_json_and_null_strings():
    d = parse_extraction('Berikut hasilnya: {"role": "other", "level": "null", "work_type": ""} semoga membantu', ROLES)
    assert d.role == "other" and d.level is None and d.work_type is None


@pytest.mark.parametrize(
    "content,msg",
    [
        ("tidak ada json", "tidak ditemukan objek JSON"),
        ('{"role": "ai-engineer",}', "JSON tidak valid"),
        ('{"role": "ai-engineer", "level": "expert"}', "level"),
        ('{"role": "devops"}', "role 'devops' tidak valid"),
        ('{"role": "ai-engineer", "required_skills": "Python"}', "required_skills"),
    ],
)
def test_parse_errors(content, msg):
    with pytest.raises(ValueError, match=msg):
        parse_extraction(content, ROLES)


@pytest.mark.anyio
async def test_extractor_retries_with_error_feedback():
    llm = FakeLLM(["bukan json", '{"role": "ai-engineer", "required_skills": ["Python"]}'])
    res = await JobExtractor(llm, max_retries=2).extract(title="AI Engineer", company=None, raw_text="x", roles=ROLES)
    assert res.attempts == 2 and res.data.required_skills == ["Python"]
    assert res.prompt_tokens == 200
    # percobaan kedua membawa balasan lama + pesan perbaikan
    second = llm.calls[1]
    assert second[-2] == {"role": "assistant", "content": "bukan json"}
    assert "tidak valid" in second[-1]["content"]


@pytest.mark.anyio
async def test_extractor_gives_up_after_max_retries():
    llm = FakeLLM(["x", "y", "z"])
    with pytest.raises(ExtractionError, match="3 percobaan"):
        await JobExtractor(llm, max_retries=2).extract(title="t", company=None, raw_text="x", roles=ROLES)
    assert len(llm.calls) == 3


@pytest.mark.anyio
async def test_extractor_truncates_long_input():
    llm = FakeLLM([{"role": "other"}])
    await JobExtractor(llm, max_input_chars=100).extract(title="t", company=None, raw_text="a" * 500, roles=ROLES)
    user_msg = llm.calls[0][1]["content"]
    assert "a" * 101 not in user_msg and "[dipotong]" in user_msg
