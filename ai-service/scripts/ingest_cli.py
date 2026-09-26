"""CLI ingestion lowongan.

Jalankan di dalam container (sudah punya akses ke backend, database, dan /data):

    docker compose exec ai-service python scripts/ingest_cli.py send            # kirim data/raw_jobs ke backend
    docker compose exec ai-service python scripts/ingest_cli.py status          # ringkasan status ekstraksi
    docker compose exec ai-service python scripts/ingest_cli.py review -n 20    # lembar validasi manual

`send` memanggil POST /api/admin/jobs (backend Go), yang lalu memicu ai-service memproses tiap lowongan.
"""

from __future__ import annotations

import argparse
import os
import random
import sys
from collections import Counter
from datetime import date
from pathlib import Path

import httpx
import psycopg

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from src.ingestion.raw_jobs import RawJobError, iter_raw_job_files, parse_raw_job  # noqa: E402
from src.utils.config import get_settings  # noqa: E402

REPO_RAW_JOBS = Path(__file__).resolve().parents[2] / "data" / "raw_jobs"
DEFAULT_DIR = Path(os.getenv("RAW_JOBS_DIR", REPO_RAW_JOBS))
DEFAULT_BACKEND = os.getenv("BACKEND_URL", "http://localhost:8080")


def cmd_send(args: argparse.Namespace) -> int:
    token = os.getenv("ADMIN_TOKEN", "")
    if not token and not args.dry_run:
        print("ADMIN_TOKEN belum di-set", file=sys.stderr)
        return 2

    files = iter_raw_job_files(args.dir)[: args.limit or None]
    counts: Counter[str] = Counter()
    with httpx.Client(base_url=args.backend, timeout=30, headers={"X-Admin-Token": token}) as client:
        for path in files:
            try:
                job = parse_raw_job(path)
            except RawJobError as e:
                counts["invalid"] += 1
                print(f"✗ {path.name}: {e}")
                continue
            if args.dry_run:
                counts["valid"] += 1
                continue

            m = job.meta
            payload = {
                "title": m.title,
                "company": m.company,
                "source_name": m.source_name,
                "source_url": str(m.source_url) if m.source_url else None,
                "posted_date": m.posted_date.isoformat() if m.posted_date else None,
                "collected_at": m.collected_at.isoformat(),
                "location": m.location,
                "work_type": m.work_type,
                "level": m.level,
                "raw_text": job.raw_text,
            }
            try:
                r = client.post("/api/admin/jobs", json=payload)
            except httpx.HTTPError as e:
                counts["error"] += 1
                print(f"✗ {path.name}: backend tidak terjangkau ({e})")
                continue

            if r.status_code == 201:
                queued = r.json().get("processing_queued")
                counts["created" if queued else "created_not_queued"] += 1
                print(f"✓ {path.name}" + ("" if queued else "  (tersimpan, ai-service tidak terjangkau)"))
            elif r.status_code == 409:
                counts["duplicate"] += 1
                print(f"= {path.name}: sudah ada ({r.json().get('existing_id')})")
            else:
                counts["error"] += 1
                print(f"✗ {path.name}: {r.status_code} {r.text[:300]}")

    print("\n" + ", ".join(f"{k} {v}" for k, v in sorted(counts.items())) + f" dari {len(files)} file")
    if not args.dry_run and (counts["created"] or counts["created_not_queued"]):
        print("Ekstraksi berjalan di background; cek progres dengan: ingest_cli.py status")
    return 1 if counts["invalid"] or counts["error"] else 0


def cmd_status(_: argparse.Namespace) -> int:
    with psycopg.connect(get_settings().database_url) as conn:
        rows = conn.execute(
            "SELECT extraction_status, count(*) FROM job_postings GROUP BY 1 ORDER BY 1"
        ).fetchall()
        print("Status ekstraksi: " + (", ".join(f"{s} {n}" for s, n in rows) or "belum ada lowongan"))

        failed = conn.execute(
            """SELECT title, extraction_error FROM job_postings
               WHERE extraction_status = 'failed' ORDER BY updated_at DESC LIMIT 5"""
        ).fetchall()
        for title, err in failed:
            print(f"  failed: {title}: {err}")

        n_unmapped = conn.execute("SELECT count(*) FROM unmapped_skills WHERE status = 'pending'").fetchone()[0]
        top_unmapped = conn.execute(
            """SELECT lower(raw_name), count(*) FROM unmapped_skills WHERE status = 'pending'
               GROUP BY 1 ORDER BY 2 DESC, 1 LIMIT 15"""
        ).fetchall()
        print(f"Unmapped skill (pending): {n_unmapped}")
        if top_unmapped:
            print("  terbanyak: " + ", ".join(f"{name} ({n})" for name, n in top_unmapped))

        top = conn.execute(
            """SELECT s.name, d.demand_pct, d.job_count, d.total_jobs FROM skill_demand d
               JOIN skills s ON s.id = d.skill_id JOIN roles r ON r.id = d.role_id
               WHERE r.slug = 'ai-engineer' ORDER BY d.demand_pct DESC, s.name LIMIT 15"""
        ).fetchall()
        if top:
            print(f"\nTop skill AI Engineer (n = {top[0][3]}):")
            for name, pct, cnt, _ in top:
                print(f"  {pct:6.2f}%  {name} ({cnt})")
    return 0


def cmd_review(args: argparse.Namespace) -> int:
    with psycopg.connect(get_settings().database_url) as conn:
        jobs = conn.execute(
            """SELECT j.id, j.title, j.company, j.source_url, j.extraction_status, r.slug
               FROM job_postings j LEFT JOIN roles r ON r.id = j.role_id
               WHERE j.extraction_status IN ('done', 'review') ORDER BY j.id"""
        ).fetchall()
        if not jobs:
            print("Belum ada lowongan yang selesai diekstrak.", file=sys.stderr)
            return 1
        sample = random.Random(args.seed).sample(jobs, min(args.n, len(jobs)))

        lines = [
            f"# Validasi ekstraksi ({len(sample)} lowongan, seed {args.seed}, {date.today()})",
            "",
            "Buka tiap lowongan, lalu isi kolom **Benar?** dengan `y` atau `n` (skill tidak ada di lowongan,",
            "atau salah tipe wajib/opsional). Tulis skill yang terlewat di baris *Terlewat*.",
            "",
            "Akurasi = jumlah `y` / (jumlah skill diekstrak + jumlah skill terlewat). Target ≥ 85%.",
            "",
        ]
        for i, (job_id, title, company, url, status, role) in enumerate(sample, 1):
            skills = conn.execute(
                """SELECT s.name, js.requirement_type FROM job_skills js JOIN skills s ON s.id = js.skill_id
                   WHERE js.job_id = %s ORDER BY js.requirement_type DESC, s.name""",
                (job_id,),
            ).fetchall()
            unmapped = [r[0] for r in conn.execute(
                "SELECT raw_name FROM unmapped_skills WHERE job_id = %s ORDER BY raw_name", (job_id,)
            ).fetchall()]
            lines += [
                f"## {i}. {title}" + (f" — {company}" if company else ""),
                "",
                f"- id: `{job_id}` · status: {status} · role: {role or 'other'}",
                f"- sumber: {url or '-'}",
                f"- unmapped: {', '.join(unmapped) or '-'}",
                "",
                "| Skill | Tipe | Benar? |",
                "| --- | --- | --- |",
                *[f"| {name} | {req} |  |" for name, req in skills],
                "",
                "*Terlewat:* ",
                "",
            ]
    args.out.write_text("\n".join(lines), encoding="utf-8")
    print(f"Lembar validasi ditulis ke {args.out}")
    return 0


def main() -> int:
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = p.add_subparsers(dest="cmd", required=True)

    s = sub.add_parser("send", help="kirim data/raw_jobs ke backend")
    s.add_argument("--dir", type=Path, default=DEFAULT_DIR)
    s.add_argument("--backend", default=DEFAULT_BACKEND)
    s.add_argument("--limit", type=int, default=0, help="batasi jumlah file (0 = semua)")
    s.add_argument("--dry-run", action="store_true", help="validasi file saja, tanpa mengirim")
    s.set_defaults(func=cmd_send)

    st = sub.add_parser("status", help="ringkasan status ekstraksi dan skill demand")
    st.set_defaults(func=cmd_status)

    rv = sub.add_parser("review", help="buat lembar validasi manual untuk sampel lowongan")
    rv.add_argument("-n", type=int, default=20)
    rv.add_argument("--seed", type=int, default=42)
    rv.add_argument("--out", type=Path, default=Path("logs/extraction_review.md"))
    rv.set_defaults(func=cmd_review)

    args = p.parse_args()
    return args.func(args)


if __name__ == "__main__":
    sys.exit(main())
