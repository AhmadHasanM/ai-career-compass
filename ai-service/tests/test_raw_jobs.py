from datetime import date, timedelta
from pathlib import Path

import pytest

from src.ingestion.raw_jobs import RawJobError, iter_raw_job_files, parse_raw_job

BODY = "Kualifikasi: Python, RAG, dan pengalaman membangun API. " * 10
TEMPLATE = Path(__file__).resolve().parents[2] / "data" / "raw_jobs" / "_TEMPLATE.md"


def write(tmp_path: Path, front: str, body: str = BODY, name: str = "job.md") -> Path:
    p = tmp_path / name
    p.write_text(f"---\n{front}---\n{body}", encoding="utf-8")
    return p


def test_valid_job(tmp_path):
    p = write(
        tmp_path,
        "title: AI Engineer\ncompany: PT Contoh\nsource_name: LinkedIn\n"
        "source_url: https://example.com/job/1\ncollected_at: 2026-09-01\nwork_type: hybrid\n",
    )
    job = parse_raw_job(p)
    assert job.meta.title == "AI Engineer"
    assert job.meta.collected_at == date(2026, 9, 1)
    assert job.raw_text.startswith("Kualifikasi")


@pytest.mark.parametrize(
    "front,msg",
    [
        ("source_name: LinkedIn\ncollected_at: 2026-09-01\n", "title"),
        ("title: AI Engineer\nsource_name: Glints\ncollected_at: 2026-09-01\nwork_type: wfh\n", "work_type"),
        ("title: AI Engineer\nsource_name: Glints\ncollected_at: 2026-09-01\nsalary: 10\n", "salary"),
        (f"title: AI Engineer\nsource_name: Glints\ncollected_at: {date.today() + timedelta(days=1)}\n", "masa depan"),
        ("title: AI Engineer\nsource_name: Glints\ncollected_at: 2026-09-01\nposted_date: 2026-09-05\n", "posted_date"),
    ],
)
def test_invalid_meta(tmp_path, front, msg):
    with pytest.raises(RawJobError, match=msg):
        parse_raw_job(write(tmp_path, front))


def test_body_too_short(tmp_path):
    with pytest.raises(RawJobError, match="terlalu pendek"):
        parse_raw_job(write(tmp_path, "title: AI Engineer\nsource_name: Glints\ncollected_at: 2026-09-01\n", body="pendek"))


def test_template_and_readme_skipped(tmp_path):
    for name in ["_TEMPLATE.md", "README.md", "a.md", "b.md"]:
        (tmp_path / name).write_text("x")
    assert [p.name for p in iter_raw_job_files(tmp_path)] == ["a.md", "b.md"]


def test_template_front_matter_is_valid(tmp_path):
    """Template harus lolos validasi metadata begitu placeholder teksnya diganti."""
    front = TEMPLATE.read_text(encoding="utf-8").split("---\n", 2)[1]
    parse_raw_job(write(tmp_path, front))
