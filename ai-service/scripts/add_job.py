"""Tambah satu lowongan ke data/raw_jobs/ dengan cepat (S1-T5).

Alur: buka halaman DETAIL lowongan di browser, salin seluruh teksnya (Ctrl+A, Ctrl+C), lalu jalankan:

    cd ai-service && .venv/bin/python scripts/add_job.py

Teks dibaca dari clipboard (wl-paste / xclip / xsel). Tanpa alat clipboard, tempel teks di terminal
lalu tekan Ctrl+D. Script menanyakan URL dan metadata, menolak URL duplikat, memvalidasi, lalu menulis
file .md sesuai data/raw_jobs/_TEMPLATE.md.
"""

from __future__ import annotations

import argparse
import re
import shutil
import subprocess
import sys
import unicodedata
from datetime import date
from pathlib import Path
from urllib.parse import urlparse

import yaml

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from src.ingestion.raw_jobs import RawJobError, iter_raw_job_files, parse_raw_job  # noqa: E402

RAW_JOBS_DIR = Path(__file__).resolve().parents[2] / "data" / "raw_jobs"
TARGET = 50

KNOWN_SOURCES = {
    "linkedin.com": "LinkedIn",
    "jobstreet.com": "JobStreet",
    "jobstreet.co.id": "JobStreet",
    "glints.com": "Glints",
    "kalibrr.com": "Kalibrr",
    "karir.com": "Karir.com",
    "dealls.com": "Dealls",
    "techinasia.com": "Tech in Asia",
    "indeed.com": "Indeed",
    "loker.id": "Loker.id",
}
LEVELS = ["intern", "junior", "mid", "senior", "lead"]
WORK_TYPES = ["onsite", "remote", "hybrid"]


def detect_source(url: str) -> str | None:
    host = (urlparse(url).hostname or "").lower()
    for domain, name in KNOWN_SOURCES.items():
        if host == domain or host.endswith("." + domain):
            return name
    return None


def slugify(text: str, max_len: int = 40) -> str:
    text = unicodedata.normalize("NFKD", text).encode("ascii", "ignore").decode().lower()
    text = re.sub(r"[^a-z0-9]+", "-", text).strip("-")
    return text[:max_len].rstrip("-") or "lowongan"


def build_filename(directory: Path, collected: date, company: str | None, title: str) -> Path:
    base = f"{collected:%Y%m%d}-{slugify(company or 'perusahaan', 25)}-{slugify(title, 40)}"
    path, n = directory / f"{base}.md", 2
    while path.exists():
        path, n = directory / f"{base}-{n}.md", n + 1
    return path


def render(meta: dict, raw_text: str) -> str:
    """Front matter YAML (field kosong dilewati) + teks lowongan apa adanya."""
    clean = {k: v for k, v in meta.items() if v not in (None, "")}
    front = yaml.safe_dump(clean, allow_unicode=True, sort_keys=False, default_flow_style=False)
    return f"---\n{front}---\n{raw_text.strip()}\n"


def existing_urls(directory: Path) -> dict[str, str]:
    urls = {}
    for path in iter_raw_job_files(directory):
        try:
            job = parse_raw_job(path)
        except RawJobError:
            continue
        if job.meta.source_url:
            urls[normalize_url(str(job.meta.source_url))] = path.name
    return urls


def normalize_url(url: str) -> str:
    """URL tanpa query/fragment dan slash akhir, agar link yang sama dengan parameter tracking terdeteksi duplikat."""
    p = urlparse(url.strip())
    return f"{p.scheme}://{(p.hostname or '').lower()}{p.path.rstrip('/')}"


def read_clipboard() -> str | None:
    for cmd in (["wl-paste", "--no-newline"], ["xclip", "-selection", "clipboard", "-o"], ["xsel", "--clipboard", "--output"]):
        if shutil.which(cmd[0]):
            try:
                out = subprocess.run(cmd, capture_output=True, text=True, timeout=5)
            except (subprocess.SubprocessError, OSError):
                continue
            if out.returncode == 0 and out.stdout.strip():
                return out.stdout
    return None


def read_pasted() -> str:
    print("Tempel teks lowongan di sini, lalu tekan Enter dan Ctrl+D:")
    return sys.stdin.read()


def ask(prompt: str, default: str | None = None, choices: list[str] | None = None, required: bool = False) -> str | None:
    hint = f" [{default}]" if default else ""
    if choices:
        hint += f" ({'/'.join(choices)}, Enter = lewati)" if not default else f" ({'/'.join(choices)})"
    while True:
        value = input(f"{prompt}{hint}: ").strip() or (default or "")
        if not value and not required:
            return None
        if not value:
            print("  wajib diisi")
            continue
        if choices and value.lower() not in choices:
            print(f"  pilih salah satu: {', '.join(choices)}")
            continue
        return value.lower() if choices else value


def first_line(text: str) -> str:
    return next((ln.strip() for ln in text.splitlines() if ln.strip()), "")[:120]


def main() -> int:
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--dir", type=Path, default=RAW_JOBS_DIR)
    p.add_argument("--paste", action="store_true", help="jangan baca clipboard; tempel teks di terminal")
    args = p.parse_args()

    raw_text = None if args.paste else read_clipboard()
    if raw_text:
        preview = " ".join(raw_text.split())[:120]
        print(f"Teks dari clipboard ({len(raw_text)} karakter): {preview}…")
        if (ask("Pakai teks ini? (y/n)", "y") or "y").lower() != "y":
            raw_text = read_pasted()
    else:
        if not args.paste:
            print("Clipboard tidak terbaca (pasang wl-clipboard: sudo apt install wl-clipboard).")
        raw_text = read_pasted()
    if len(raw_text.strip()) < 300:
        print(f"✗ Teks terlalu pendek ({len(raw_text.strip())} karakter). Salin seluruh isi halaman lowongan.")
        return 1

    known = existing_urls(args.dir)
    while True:
        url = ask("URL halaman detail lowongan", required=False)
        if not url:
            print("  (tanpa URL, lowongan tidak bisa disitir dan duplikat tidak terdeteksi)")
            break
        if not url.startswith(("http://", "https://")):
            print("  URL harus diawali http:// atau https://")
            continue
        if "/jobs?" in url or url.rstrip("/").endswith(("-jobs", "/jobs", "/lowongan")):
            print("  Sepertinya ini halaman HASIL PENCARIAN. Buka satu lowongan dan salin URL halaman detailnya.")
            continue
        if (dup := known.get(normalize_url(url))) is not None:
            print(f"✗ Lowongan ini sudah ada: {dup}")
            return 1
        break

    meta = {
        "title": ask("Judul", first_line(raw_text), required=True),
        "company": ask("Perusahaan"),
        "source_name": ask("Sumber", detect_source(url) if url else None, required=True),
        "source_url": url,
        "posted_date": None,
        "collected_at": date.today(),
        "location": ask("Lokasi (kota / Remote)"),
        "work_type": ask("Tipe kerja", choices=WORK_TYPES),
        "level": ask("Level", choices=LEVELS),
    }
    while (posted := ask("Tanggal posting YYYY-MM-DD")) is not None:
        try:
            meta["posted_date"] = date.fromisoformat(posted)
            break
        except ValueError:
            print("  format harus YYYY-MM-DD")

    args.dir.mkdir(parents=True, exist_ok=True)
    path = build_filename(args.dir, meta["collected_at"], meta["company"], meta["title"])
    path.write_text(render(meta, raw_text), encoding="utf-8")
    try:
        parse_raw_job(path)
    except RawJobError as e:
        path.unlink()
        print(f"✗ Tidak disimpan, data tidak valid: {e}")
        return 1

    valid = sum(1 for f in iter_raw_job_files(args.dir) if _is_valid(f))
    print(f"✓ Tersimpan: {path.name}")
    print(f"Progres: {valid}/{TARGET} lowongan")
    return 0


def _is_valid(path: Path) -> bool:
    try:
        parse_raw_job(path)
        return True
    except RawJobError:
        return False


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (KeyboardInterrupt, EOFError):
        print("\nDibatalkan.")
        sys.exit(130)
