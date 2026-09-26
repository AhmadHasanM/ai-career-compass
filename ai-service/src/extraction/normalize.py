"""Normalisasi nama skill hasil ekstraksi ke taxonomy lewat tabel skill_aliases."""

from __future__ import annotations

import re
from dataclasses import dataclass, field

_PAREN = re.compile(r"\s*\([^)]*\)")
_VERSION = re.compile(r"\s+v?\d+(?:\.(?:\d+|x))*\+?$")
_SUFFIX = re.compile(r"\s+(?:programming(?: language)?|language|framework|library|libraries|development)$")


def alias_candidates(raw: str) -> list[str]:
    """Bentuk-bentuk yang dicoba berurutan: persis, tanpa kurung, tanpa versi, tanpa akhiran generik.

    "Python 3.x" -> python; "PyTorch (Deep Learning)" -> pytorch; "React framework" -> react.
    """
    base = " ".join(raw.lower().split()).strip(" .,;:")
    out: list[str] = []
    for c in (base, _PAREN.sub("", base).strip()):
        for variant in (c, _VERSION.sub("", c), _SUFFIX.sub("", c), _SUFFIX.sub("", _VERSION.sub("", c))):
            variant = variant.strip(" .,;:")
            if variant and variant not in out:
                out.append(variant)
    return out


@dataclass
class NormalizedSkills:
    # skill_id -> "required" | "preferred"
    skills: dict[int, str] = field(default_factory=dict)
    # nama mentah yang tidak cocok dengan alias mana pun (unik, urutan asli)
    unmapped: list[str] = field(default_factory=list)


class SkillNormalizer:
    def __init__(self, alias_to_skill: dict[str, int]) -> None:
        self.alias_to_skill = alias_to_skill

    def lookup(self, raw: str) -> int | None:
        for cand in alias_candidates(raw):
            if (sid := self.alias_to_skill.get(cand)) is not None:
                return sid
        return None

    def normalize(self, required: list[str], preferred: list[str]) -> NormalizedSkills:
        result = NormalizedSkills()
        unmapped_seen: set[str] = set()
        # required diproses dulu: skill yang muncul di keduanya tetap "required".
        for requirement, names in (("required", required), ("preferred", preferred)):
            for name in names:
                sid = self.lookup(name)
                if sid is None:
                    key = name.strip().lower()
                    if key and key not in unmapped_seen:
                        unmapped_seen.add(key)
                        result.unmapped.append(name.strip())
                elif sid not in result.skills:
                    result.skills[sid] = requirement
        return result


async def load_alias_map(conn) -> dict[str, int]:
    cur = await conn.execute("SELECT alias, skill_id FROM skill_aliases")
    return {alias: sid for alias, sid in await cur.fetchall()}
