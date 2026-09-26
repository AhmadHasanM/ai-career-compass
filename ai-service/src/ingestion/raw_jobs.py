"""Parser file lowongan mentah di data/raw_jobs/.

Satu file = satu lowongan: front matter YAML (metadata) + teks lowongan apa adanya.
Dipakai validator sekarang dan ingest_cli (Sprint 3) nanti.
"""

from __future__ import annotations

from datetime import date
from pathlib import Path
from typing import Literal

import yaml
from pydantic import BaseModel, Field, HttpUrl, ValidationError, model_validator

MIN_BODY_CHARS = 300


class RawJobMeta(BaseModel):
    model_config = {"extra": "forbid"}

    title: str = Field(min_length=3)
    company: str | None = None
    source_name: str = Field(min_length=2)
    source_url: HttpUrl | None = None
    posted_date: date | None = None
    collected_at: date
    location: str | None = None
    work_type: Literal["onsite", "remote", "hybrid"] | None = None
    level: Literal["intern", "junior", "mid", "senior", "lead"] | None = None

    @model_validator(mode="after")
    def _dates_make_sense(self) -> RawJobMeta:
        if self.collected_at > date.today():
            raise ValueError("collected_at tidak boleh di masa depan")
        if self.posted_date and self.posted_date > self.collected_at:
            raise ValueError("posted_date tidak boleh setelah collected_at")
        return self


class RawJob(BaseModel):
    path: Path
    meta: RawJobMeta
    raw_text: str


class RawJobError(Exception):
    pass


def parse_raw_job(path: Path) -> RawJob:
    text = path.read_text(encoding="utf-8")
    if not text.startswith("---\n"):
        raise RawJobError("file harus diawali front matter '---'")
    try:
        _, front, body = text.split("---\n", 2)
    except ValueError as e:
        raise RawJobError("penutup front matter '---' tidak ditemukan") from e

    try:
        data = yaml.safe_load(front) or {}
    except yaml.YAMLError as e:
        raise RawJobError(f"YAML tidak valid: {e}") from e
    if not isinstance(data, dict):
        raise RawJobError("front matter harus berupa mapping YAML")

    try:
        meta = RawJobMeta.model_validate(data)
    except ValidationError as e:
        issues = "; ".join(f"{'.'.join(map(str, err['loc'])) or 'meta'}: {err['msg']}" for err in e.errors())
        raise RawJobError(issues) from e

    body = body.strip()
    if len(body) < MIN_BODY_CHARS:
        raise RawJobError(f"teks lowongan terlalu pendek ({len(body)} < {MIN_BODY_CHARS} karakter)")
    return RawJob(path=path, meta=meta, raw_text=body)


def iter_raw_job_files(directory: Path) -> list[Path]:
    """File .md di folder, kecuali yang diawali '_' (template) dan README."""
    return sorted(
        p for p in directory.glob("*.md") if not p.name.startswith("_") and p.name.lower() != "readme.md"
    )
