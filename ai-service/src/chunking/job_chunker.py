"""Pecah teks lowongan per bagian (tanggung jawab, kualifikasi, benefit, ...) lalu per ukuran chunk.

Deteksi bagian memakai heading umum lowongan Indonesia/Inggris; tidak butuh LLM sehingga deterministik.
"""

from __future__ import annotations

import re
from dataclasses import dataclass

SECTION_LABELS = {
    "overview": "Ringkasan",
    "about": "Tentang perusahaan",
    "responsibilities": "Tanggung jawab",
    "qualifications": "Kualifikasi",
    "benefits": "Benefit",
}

_HEADINGS: dict[str, tuple[str, ...]] = {
    "responsibilities": (
        "tanggung jawab", "deskripsi pekerjaan", "uraian pekerjaan", "job description", "job desc",
        "responsibilities", "key responsibilities", "responsibility", "what you'll do", "what you will do",
        "your role", "the role", "tugas", "jobdesk", "job responsibilities",
    ),
    "qualifications": (
        "kualifikasi", "persyaratan", "syarat", "requirements", "requirement", "qualifications",
        "qualification", "what we're looking for", "what we are looking for", "who you are", "what you need",
        "must have", "nice to have", "preferred qualifications", "minimum qualifications", "skills",
        "skills and experience", "keahlian",
    ),
    "benefits": (
        "benefit", "benefits", "keuntungan", "fasilitas", "what we offer", "perks", "kompensasi",
        "why join us", "perks and benefits",
    ),
    "about": (
        "tentang perusahaan", "tentang kami", "about us", "about the company", "company overview", "who we are",
    ),
}
_HEADING_LOOKUP = {h: section for section, hs in _HEADINGS.items() for h in hs}
_HEADING_CLEAN = re.compile(r"^[\s#*_>\-•·]+|[\s*_:：\-]+$")


def _heading_section(line: str) -> str | None:
    if len(line) > 60:
        return None
    key = _HEADING_CLEAN.sub("", line).strip().lower()
    return _HEADING_LOOKUP.get(key)


@dataclass
class Section:
    name: str
    text: str


def split_sections(text: str) -> list[Section]:
    sections: list[Section] = []
    current, buf = "overview", []

    def flush():
        body = "\n".join(buf).strip()
        if body:
            if sections and sections[-1].name == current:
                sections[-1].text += "\n" + body
            else:
                sections.append(Section(current, body))

    for line in text.splitlines():
        if (sec := _heading_section(line.strip())) is not None:
            flush()
            current, buf = sec, []
        else:
            buf.append(line)
    flush()
    return sections


def _split_long_line(line: str, size: int, overlap: int) -> list[str]:
    step = max(size - overlap, 1)
    return [line[i : i + size] for i in range(0, len(line), step) if line[i : i + size].strip()]


def chunk_text(text: str, size: int, overlap: int) -> list[str]:
    """Gabungkan baris sampai ~size karakter; chunk berikutnya diawali ekor chunk sebelumnya (~overlap)."""
    if len(text) <= size:
        return [text]
    lines: list[str] = []
    for line in text.splitlines():
        lines.extend(_split_long_line(line, size, overlap) if len(line) > size else [line])

    chunks: list[str] = []
    cur: list[str] = []
    cur_len = 0
    for line in lines:
        if cur and cur_len + len(line) + 1 > size:
            chunks.append("\n".join(cur).strip())
            tail: list[str] = []
            tail_len = 0
            for prev in reversed(cur):
                if tail_len + len(prev) + 1 > overlap:
                    break
                tail.insert(0, prev)
                tail_len += len(prev) + 1
            cur, cur_len = tail, tail_len
        cur.append(line)
        cur_len += len(line) + 1
    if "\n".join(cur).strip():
        chunks.append("\n".join(cur).strip())
    return [c for c in chunks if c]


@dataclass
class Chunk:
    index: int
    section: str
    content: str  # termasuk header judul/perusahaan/bagian agar retrieval punya konteks


def chunk_job(*, title: str, company: str | None, raw_text: str, size: int = 800, overlap: int = 100) -> list[Chunk]:
    header_base = f"{title}" + (f" — {company}" if company else "")
    out: list[Chunk] = []
    for sec in split_sections(raw_text):
        header = f"{header_base}\nBagian: {SECTION_LABELS[sec.name]}\n\n"
        for piece in chunk_text(sec.text, size, overlap):
            out.append(Chunk(index=len(out), section=sec.name, content=header + piece))
    return out
