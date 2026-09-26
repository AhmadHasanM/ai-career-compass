"""Skema output ekstraksi lowongan. Output LLM wajib lolos validasi ini sebelum ditulis ke database."""

from __future__ import annotations

from typing import Literal

from pydantic import BaseModel, Field, ValidationInfo, field_validator

Level = Literal["intern", "junior", "mid", "senior", "lead"]
WorkType = Literal["onsite", "remote", "hybrid"]

MAX_SKILLS = 40
_EMPTY_VALUES = {"", "null", "none", "unknown", "-", "n/a"}


def _clean_skill_list(values: list[str]) -> list[str]:
    seen: set[str] = set()
    out: list[str] = []
    for v in values:
        v = " ".join(str(v).split()).strip(" .,;:-")
        if v and len(v) <= 80 and v.lower() not in seen:
            seen.add(v.lower())
            out.append(v)
    return out[:MAX_SKILLS]


class JobExtraction(BaseModel):
    model_config = {"extra": "ignore"}

    role: str = Field(description="slug role dari daftar yang diberikan, atau 'other'")
    level: Level | None = None
    location: str | None = Field(default=None, max_length=120)
    work_type: WorkType | None = None
    required_skills: list[str] = Field(default_factory=list)
    preferred_skills: list[str] = Field(default_factory=list)

    @field_validator("required_skills", "preferred_skills", mode="before")
    @classmethod
    def _skills(cls, v):
        if v is None:
            return []
        if not isinstance(v, list):
            raise ValueError("harus berupa list string")
        return _clean_skill_list(v)

    @field_validator("level", "work_type", "location", mode="before")
    @classmethod
    def _normalize_optional(cls, v, info: ValidationInfo):
        if not isinstance(v, str):
            return v
        s = v.strip()
        if s.lower() in _EMPTY_VALUES:
            return None
        # level dan work_type berupa enum lowercase; lokasi dibiarkan apa adanya.
        return s if info.field_name == "location" else s.lower()
