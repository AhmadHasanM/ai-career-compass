"""Validasi semua lowongan mentah di data/raw_jobs/.

    python scripts/validate_raw_jobs.py [--dir ../data/raw_jobs] [--target 50]

Exit code 1 jika ada file tidak valid atau duplikat source_url.
"""

import argparse
import sys
from collections import Counter
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from src.ingestion.raw_jobs import RawJobError, iter_raw_job_files, parse_raw_job  # noqa: E402

DEFAULT_DIR = Path(__file__).resolve().parents[2] / "data" / "raw_jobs"


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--dir", type=Path, default=DEFAULT_DIR)
    parser.add_argument("--target", type=int, default=50)
    args = parser.parse_args()

    files = iter_raw_job_files(args.dir)
    jobs, failed = [], 0
    for path in files:
        try:
            jobs.append(parse_raw_job(path))
        except RawJobError as e:
            failed += 1
            print(f"✗ {path.name}: {e}")

    urls = Counter(str(j.meta.source_url) for j in jobs if j.meta.source_url)
    dupes = {u: n for u, n in urls.items() if n > 1}
    for url, n in dupes.items():
        print(f"✗ source_url duplikat ({n}x): {url}")

    sources = Counter(j.meta.source_name for j in jobs)
    print(f"\n{len(jobs)} valid, {failed} tidak valid, dari {len(files)} file")
    if sources:
        print("Per sumber: " + ", ".join(f"{s} {n}" for s, n in sources.most_common()))
    if len(jobs) < args.target:
        print(f"Progres: {len(jobs)}/{args.target} lowongan")

    return 1 if failed or dupes else 0


if __name__ == "__main__":
    sys.exit(main())
