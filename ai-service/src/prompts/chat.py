"""Prompt chatbot karier: sumber bernomor, konteks pengguna, aturan sitasi, penolakan di luar domain."""

from __future__ import annotations

SYSTEM_PROMPT = """Kamu adalah AI Career Compass, asisten karier untuk orang yang ingin berkarier di bidang AI,
machine learning, dan data di Indonesia. Jawabanmu berbasis data lowongan kerja nyata dan sumber belajar terkurasi.

ATURAN
1. Domain: hanya karier dan belajar di bidang teknologi (AI, ML, data, software engineering): skill, lowongan,
   roadmap, sumber belajar, portofolio, interview. Untuk pertanyaan di luar itu, balas singkat dan sopan bahwa
   kamu hanya membantu seputar karier tech, lalu tawarkan contoh pertanyaan yang relevan. Jangan beri sitasi.
2. Fakta tentang pasar kerja, lowongan, perusahaan, angka, dan sumber belajar HANYA boleh diambil dari SUMBER.
   Setiap kalimat yang memakai sumber diakhiri sitasi nomornya, misal [2] atau [1][3]. Jangan menyitir nomor
   yang tidak ada. Jangan mengarang lowongan, gaji, perusahaan, persentase, atau URL.
3. Jika SUMBER tidak cukup untuk menjawab, katakan terus terang datanya belum ada, lalu beri saran umum
   yang jelas ditandai sebagai saran umum (tanpa sitasi).
4. Gunakan PROFIL PENGGUNA untuk personalisasi (skill yang sudah/belum dimiliki, roadmap), tapi profil
   bukan sumber dan tidak disitir.
5. Jika DATA PASAR ditandai sampel kecil, sebutkan bahwa angkanya berasal dari sampel terbatas.
6. Jawab dalam bahasa yang dipakai pengguna (default bahasa Indonesia), ringkas (maksimal ±200 kata),
   pakai poin bila membantu. Jangan mengulang pertanyaan."""

_SOURCE_LABEL = {"job_posting": "Lowongan", "learning_resource": "Sumber belajar", "market_data": "Data pasar"}
_SECTION_LABEL = {"overview": "Ringkasan", "about": "Tentang perusahaan", "responsibilities": "Tanggung jawab",
                  "qualifications": "Kualifikasi", "benefits": "Benefit", "resource": "Sumber belajar"}
MAX_SOURCE_CHARS = 1200


def strip_chunk_header(content: str) -> str:
    # Chunk lowongan diawali header "Judul — Perusahaan\nBagian: X\n\n"; header sudah ada di label sumber.
    head, sep, body = content.partition("\n\n")
    return body if sep and head.count("\n") <= 1 and "Bagian:" in head else content


def format_source(n: int, src: dict) -> str:
    kind = _SOURCE_LABEL.get(src["source_type"], src["source_type"])
    if src["source_type"] == "market_data":
        return f"[{n}] ({kind}) {src['text']}"
    meta = src.get("metadata") or {}
    parts = [meta.get("title") or "-"]
    if meta.get("company"):
        parts[0] += f" — {meta['company']}"
    if src.get("section") and src["section"] in _SECTION_LABEL:
        parts.append(f"bagian: {_SECTION_LABEL[src['section']]}")
    if meta.get("posted_date"):
        parts.append(f"diposting {meta['posted_date']}")
    if meta.get("skill_name"):
        parts.append(f"skill: {meta['skill_name']}")
    body = strip_chunk_header(src["content"]).strip()
    if len(body) > MAX_SOURCE_CHARS:
        body = body[:MAX_SOURCE_CHARS] + " …"
    return f"[{n}] ({kind}) {' · '.join(parts)}\n{body}"


def format_market(market: dict) -> str:
    items = ", ".join(
        f"{it['name']} {it['demand_pct']:g}% (wajib {it['required_pct']:g}%)" for it in market["items"]
    )
    note = " SAMPEL KECIL." if market.get("small_sample") else ""
    snap = f", snapshot {market['snapshot_date']}" if market.get("snapshot_date") else ""
    return (f"Statistik skill yang diminta lowongan {market['role']} dari {market['total_jobs']} lowongan{snap}."
            f"{note} Persentase = porsi lowongan yang menyebut skill: {items}.")


def format_user(user: dict | None) -> str:
    if not user:
        return "Belum mengisi profil."
    lines = []
    for label, key in (("Pendidikan", "education"), ("Pekerjaan saat ini", "current_job"),
                       ("Target role", "target_role")):
        if user.get(key):
            lines.append(f"{label}: {user[key]}")
    if user.get("hours_per_week"):
        lines.append(f"Waktu belajar: {user['hours_per_week']} jam/minggu")
    for label, key in (("Skill dikuasai", "skills"), ("Skill yang belum dimiliki (prioritas)", "gaps"),
                       ("Roadmap saat ini", "roadmap")):
        if user.get(key):
            lines.append(f"{label}: {', '.join(user[key])}")
    return "\n".join(lines) or "Belum mengisi profil."


def build_messages(*, question: str, sources: list[dict], user: dict | None, history: list[dict]) -> list[dict]:
    source_block = "\n\n".join(format_source(i, s) for i, s in enumerate(sources, start=1)) or "(tidak ada sumber relevan)"
    context = f"SUMBER\n{source_block}\n\nPROFIL PENGGUNA\n{format_user(user)}"
    messages = [{"role": "system", "content": SYSTEM_PROMPT + "\n\n" + context}]
    messages += [{"role": h["role"], "content": h["content"]} for h in history]
    messages.append({"role": "user", "content": question})
    return messages
