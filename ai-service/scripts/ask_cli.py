"""Jalankan pertanyaan uji chatbot lewat backend (POST /api/chat) dan tulis lembar penilaian manual (S5-T6).

    docker compose exec ai-service python scripts/ask_cli.py [--limit 5]
    docker compose cp ai-service:/app/logs/chat_review.md .
"""

from __future__ import annotations

import argparse
import json
import os
import sys
import time
from datetime import date
from pathlib import Path

import httpx
import yaml

REPO_QUESTIONS = Path(__file__).resolve().parents[2] / "data" / "eval" / "chat_questions.yaml"
DEFAULT_QUESTIONS = Path(os.getenv("CHAT_QUESTIONS_FILE", REPO_QUESTIONS))
DEFAULT_BACKEND = os.getenv("BACKEND_URL", "http://localhost:8080")
EXPECT_LABEL = {"cited": "harus bersitasi", "refuse": "harus menolak sopan", "honest": "harus jujur bila data tidak ada"}


def parse_sse(lines) -> tuple[str, dict | None, str | None]:
    """Kembalikan (teks dari token, data event done, pesan error)."""
    text, done, error, event = [], None, None, None
    for line in lines:
        if line.startswith("event:"):
            event = line[6:].strip()
        elif line.startswith("data:"):
            data = json.loads(line[5:].strip())
            if event == "token":
                text.append(data["text"])
            elif event == "done":
                done = data
            elif event == "error":
                error = data.get("message")
    return "".join(text), done, error


def create_session(client: httpx.Client) -> str:
    """POST /api/sessions dibatasi per IP per menit; tunggu sesuai Retry-After bila kena limit."""
    while True:
        r = client.post("/api/sessions")
        if r.status_code != 429:
            r.raise_for_status()
            return r.json()["session_id"]
        wait = float(r.headers.get("Retry-After", "5"))
        print(f"   batas pembuatan sesi, menunggu {wait:g} detik...")
        time.sleep(min(wait, 60))


def setup_session(client: httpx.Client, profile: dict | None) -> str:
    sid = create_session(client)
    if profile:
        roles = client.get("/api/roles").json()["roles"]
        skills = {s["slug"]: s["id"] for s in client.get("/api/skills").json()["skills"]}
        body = {
            "education": profile.get("education"), "current_job": profile.get("current_job"),
            "target_role_id": roles[0]["id"], "hours_per_week": profile.get("hours_per_week", 5),
            "skills": [{"skill_id": skills[s], "proficiency": "intermediate"} for s in profile.get("skills", []) if s in skills],
        }
        client.put("/api/profile", json=body, headers={"X-Session-Id": sid}).raise_for_status()
    return sid


def main() -> int:
    p = argparse.ArgumentParser()
    p.add_argument("--file", type=Path, default=DEFAULT_QUESTIONS)
    p.add_argument("--backend", default=DEFAULT_BACKEND)
    p.add_argument("--limit", type=int, default=0)
    p.add_argument("--out", type=Path, default=Path("logs/chat_review.md"))
    args = p.parse_args()

    spec = yaml.safe_load(args.file.read_text(encoding="utf-8"))
    questions = spec["questions"][: args.limit or None]
    lines = [
        f"# Uji awal chatbot ({len(questions)} pertanyaan, {date.today()})",
        "",
        "Isi **Nilai** tiap jawaban: `ok`, `kurang` (benar tapi lemah), atau `salah` (mengarang, sitasi keliru,",
        "atau tidak menolak pertanyaan di luar domain). Cek sitasi dengan membuka sumbernya.",
        "",
    ]
    latencies = []
    with httpx.Client(base_url=args.backend, timeout=120) as client:
        sid = ""
        for i, item in enumerate(questions, 1):
            if not item.get("follow_up"):
                sid = setup_session(client, spec.get("profile"))  # sesi baru agar riwayat tidak mencampur
            start = time.perf_counter()
            with client.stream("POST", "/api/chat", json={"message": item["q"]}, headers={"X-Session-Id": sid}) as r:
                if r.status_code != 200:
                    answer, done, error = "", None, f"HTTP {r.status_code}: {r.read().decode()[:200]}"
                else:
                    answer, done, error = parse_sse(r.iter_lines())
            elapsed = time.perf_counter() - start
            latencies.append(elapsed)
            cites = (done or {}).get("citations", [])
            print(f"{i:>2}. {elapsed:5.1f}s  {len(cites)} sitasi  {item['q'][:60]}" + (f"  ERROR {error}" if error else ""))

            lines += [f"## {i}. {item['q']}", "",
                      f"*Harapan: {EXPECT_LABEL.get(item.get('expect'), '-')}"
                      + (" · lanjutan pertanyaan sebelumnya" if item.get("follow_up") else "")
                      + f" · {elapsed:.1f} detik*", ""]
            lines += [f"> **Error:** {error}", ""] if error else [answer, ""]
            if cites:
                lines.append("**Sitasi:**")
                for c in cites:
                    where = c.get("url") or ("dashboard skill demand" if c["source_type"] == "market_data" else "-")
                    lines.append(f"- [{c['n']}] {c['source_type']}: {c.get('title') or '-'} ({where})")
                lines.append("")
            lines += ["**Nilai:** ", "", "---", ""]

    if latencies:
        lat = sorted(latencies)
        p95 = lat[min(len(lat) - 1, int(round(0.95 * (len(lat) - 1))))]
        lines.insert(2, f"Latensi total per jawaban: median {lat[len(lat) // 2]:.1f} s, p95 {p95:.1f} s (target p95 ≤ 8 s).\n")
        print(f"\nmedian {lat[len(lat) // 2]:.1f}s, p95 {p95:.1f}s")
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text("\n".join(lines), encoding="utf-8")
    print(f"Lembar penilaian ditulis ke {args.out}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
