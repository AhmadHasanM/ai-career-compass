import importlib.util
import io
from datetime import date
from pathlib import Path

import pytest

from src.ingestion.raw_jobs import parse_raw_job

_spec = importlib.util.spec_from_file_location("add_job", Path(__file__).resolve().parents[1] / "scripts" / "add_job.py")
add_job = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(add_job)

TEXT = "AI Engineer\nPT Contoh\n\nKualifikasi:\n" + "- Menguasai Python, RAG, dan Docker\n" * 15


@pytest.mark.parametrize(
    "url,expected",
    [
        ("https://id.jobstreet.com/id/job/12345678", "JobStreet"),
        ("https://www.linkedin.com/jobs/view/123", "LinkedIn"),
        ("https://glints.com/id/opportunities/jobs/ai-engineer/abc", "Glints"),
        ("https://careers.contoh.co.id/ai", None),
    ],
)
def test_detect_source(url, expected):
    assert add_job.detect_source(url) == expected


def test_slugify_and_unique_filename(tmp_path):
    assert add_job.slugify("Senior AI Engineer (LLM) – Jakarta!") == "senior-ai-engineer-llm-jakarta"
    first = add_job.build_filename(tmp_path, date(2026, 9, 27), "PT Contoh Tëch", "AI Engineer")
    assert first.name == "20260927-pt-contoh-tech-ai-engineer.md"
    first.write_text("x")
    assert add_job.build_filename(tmp_path, date(2026, 9, 27), "PT Contoh Tëch", "AI Engineer").name.endswith("-2.md")


def test_normalize_url_ignores_tracking_params():
    a = add_job.normalize_url("https://id.jobstreet.com/id/job/123?ref=search&utm_source=x#top")
    assert a == add_job.normalize_url("https://ID.JOBSTREET.COM/id/job/123/")


def test_render_roundtrips_through_validator(tmp_path):
    meta = {"title": "AI Engineer: LLM & RAG", "company": None, "source_name": "JobStreet",
            "source_url": "https://id.jobstreet.com/id/job/1", "posted_date": date(2026, 9, 20),
            "collected_at": date(2026, 9, 27), "location": "Jakarta", "work_type": "hybrid", "level": None}
    path = tmp_path / "job.md"
    path.write_text(add_job.render(meta, TEXT))
    job = parse_raw_job(path)
    assert job.meta.title == "AI Engineer: LLM & RAG" and job.meta.company is None and job.meta.level is None
    assert job.raw_text.startswith("AI Engineer\nPT Contoh")


def test_main_end_to_end_with_pasted_text(tmp_path, monkeypatch, capsys):
    (tmp_path / "existing.md").write_text(add_job.render(
        {"title": "Lama", "source_name": "Glints", "source_url": "https://glints.com/id/job/9",
         "collected_at": date(2026, 9, 1)}, TEXT))
    answers = iter([
        "https://id.jobstreet.com/id/ai-engineer-jobs",   # halaman pencarian -> ditolak
        "https://id.jobstreet.com/id/job/555?ref=x",      # URL detail
        "",                                               # judul: pakai tebakan baris pertama
        "PT Contoh",
        "",                                               # sumber: pakai deteksi JobStreet
        "Jakarta",
        "wfh", "hybrid",                                  # pilihan tidak valid lalu valid
        "mid",
        "20-09-2026", "2026-09-20",                       # format salah lalu benar
    ])
    monkeypatch.setattr("builtins.input", lambda _="": next(answers))
    monkeypatch.setattr("sys.stdin", io.StringIO(TEXT))
    monkeypatch.setattr("sys.argv", ["add_job.py", "--dir", str(tmp_path), "--paste"])

    assert add_job.main() == 0
    out = capsys.readouterr().out
    assert "HASIL PENCARIAN" in out and "Progres: 2/50" in out
    [saved] = [p for p in tmp_path.glob("*.md") if p.name != "existing.md"]
    job = parse_raw_job(saved)
    assert (job.meta.title, job.meta.company, job.meta.source_name, job.meta.work_type, job.meta.level) == \
        ("AI Engineer", "PT Contoh", "JobStreet", "hybrid", "mid")
    assert str(job.meta.posted_date) == "2026-09-20"


def test_main_rejects_duplicate_url(tmp_path, monkeypatch, capsys):
    (tmp_path / "existing.md").write_text(add_job.render(
        {"title": "Lama", "source_name": "JobStreet", "source_url": "https://id.jobstreet.com/id/job/555",
         "collected_at": date(2026, 9, 1)}, TEXT))
    answers = iter(["https://id.jobstreet.com/id/job/555/?utm_source=share"])
    monkeypatch.setattr("builtins.input", lambda _="": next(answers))
    monkeypatch.setattr("sys.stdin", io.StringIO(TEXT))
    monkeypatch.setattr("sys.argv", ["add_job.py", "--dir", str(tmp_path), "--paste"])
    assert add_job.main() == 1
    assert "sudah ada: existing.md" in capsys.readouterr().out
    assert len(list(tmp_path.glob("*.md"))) == 1
