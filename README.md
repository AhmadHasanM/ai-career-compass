# AI Career Compass

Roadmap belajar AI yang dibangun dari data lowongan kerja nyata di Indonesia. Setiap rekomendasi skill bisa ditelusuri ke lowongan sumbernya. MVP fokus ke role **AI Engineer**.

## Arsitektur

| Service | Stack | Port host | Peran |
| --- | --- | --- | --- |
| `frontend` | Next.js 16 (App Router), Tailwind v4 | 3000 | UI |
| `backend` | Go, Gin, pgx | 8080 | API publik, logika bisnis, pemilik skema DB |
| `ai-service` | Python, FastAPI | tidak diekspos | Ekstraksi, embedding, retrieval, chat RAG |
| `db` | PostgreSQL 16 + pgvector | 5433 | Data relasional, vektor, full-text search |

Ada juga dua service sekali jalan: `migrate` (golang-migrate) dan `seed` (taxonomy dari `data/taxonomy/skills.yaml`). Backend dan ai-service berkomunikasi lewat REST di jaringan Docker `internal` dengan header `X-Internal-Token`.

## Menjalankan

```bash
cp .env.example .env          # isi LLM_* dan ganti token sebelum dipakai serius
docker compose up -d --build  # db -> migrate -> seed -> backend, ai-service, frontend
```

Cek kesehatan:

```bash
curl localhost:8080/health    # backend + database
curl localhost:3000/health    # frontend
docker compose exec backend wget -qO- http://ai-service:8000/health   # ai-service (internal)
```

## Database

Migrasi ada di `backend/migrations/` dan hanya dijalankan oleh backend/`migrate`; ai-service tidak menjalankan migrasi.

```bash
docker compose run --rm migrate                      # up
docker compose run --rm migrate -path=/migrations \
  -database="postgres://compass:compass@db:5432/compass?sslmode=disable" down 1   # rollback 1 langkah
docker compose run --rm seed                         # seed ulang taxonomy (idempotent)
```

`skill_demand` adalah materialized view yang di-refresh setelah setiap ingestion:
`REFRESH MATERIALIZED VIEW CONCURRENTLY skill_demand;`

## API backend

Semua error berbentuk `{"error": {"code", "message", "fields?"}}`. Endpoint pengguna butuh header `X-Session-Id` (dari `POST /api/sessions`); endpoint admin butuh `X-Admin-Token`.

| Method | Path | Auth | Keterangan |
| --- | --- | --- | --- |
| GET | `/health` | - | Status service + database |
| POST | `/api/sessions` | - | Buat sesi anonim (dibatasi per IP per menit) |
| GET | `/api/roles` | - | Daftar role target |
| GET | `/api/skills?category=` | - | Taxonomy + alias + prerequisite |
| GET / PUT | `/api/profile` | sesi | Profil + skill; PUT mengganti seluruh profil |
| GET | `/api/insights/skill-demand?role=&level=&limit=` | - | Top skill + n + tanggal snapshot (`small_sample` jika n < 30) |
| GET | `/api/insights/skill-demand/{skill_id}/jobs` | - | Lowongan asal sebuah persentase (bukti) |
| GET | `/api/gap` | sesi | Skill gap (demand ≥ 20%, skor = demand × bobot kategori) + cakupan |
| POST | `/api/roadmap/generate` | sesi | Roadmap baru: topological sort di Go, alasan + estimasi dari LLM (fallback jika gagal) |
| GET | `/api/roadmap` | sesi | Roadmap terbaru + edge prasyarat + sumber belajar per node |
| GET | `/api/resources?skill_id=` | - | Sumber belajar per skill |
| POST | `/api/chat` | sesi | Chat RAG, respons SSE: `token` berulang lalu `done` (jawaban, sitasi lengkap, usage) atau `error` |
| GET | `/api/chat/history?limit=` | sesi | Riwayat chat beserta sitasi |
| POST | `/api/admin/jobs` | admin | Tambah lowongan (status `pending`); 409 jika `source_url` sudah ada |
| POST | `/api/admin/resources` | admin | Tambah sumber belajar (`skill_id` atau `skill_slug`) lalu embed |

Endpoint publik dan pengguna dibatasi rate limit per IP (`RATE_LIMIT_RPS`, `RATE_LIMIT_BURST`; generate roadmap `ROADMAP_GENERATE_PER_MINUTE`); respons 429 menyertakan `Retry-After`. Endpoint admin tidak ikut dibatasi karena dilindungi token dan dipakai ingestion massal.

## Ingestion lowongan

Alur: `ingest_cli send` → `POST /api/admin/jobs` (Go, simpan `pending`) → `POST /internal/jobs/{id}/process` (ai-service, 202) →
di background: ekstraksi LLM → normalisasi skill ke taxonomy → chunking per bagian → embedding → `document_chunks` → refresh `skill_demand`.

Status akhir: `done` (role AI/ML + ada skill cocok), `review` (role lain atau tidak ada skill cocok), `failed` (lihat `extraction_error`).

```bash
# 1. isi LLM_BASE_URL, LLM_API_KEY, LLM_MODEL di .env, lalu restart ai-service
docker compose exec ai-service python scripts/check_llm.py        # cek chat, JSON mode, embedding

# 2. kirim lowongan dan pantau
docker compose exec ai-service python scripts/ingest_cli.py send --dry-run
docker compose exec ai-service python scripts/ingest_cli.py send
docker compose exec ai-service python scripts/ingest_cli.py status

# sumber belajar terkurasi (data/resources/learning_resources.yaml)
docker compose exec ai-service python scripts/ingest_cli.py resources --embed-missing

# 3. validasi manual 20 lowongan (target akurasi ≥ 85%)
docker compose exec ai-service python scripts/ingest_cli.py review -n 20
docker compose cp ai-service:/app/logs/extraction_review.md .
```

## Chat RAG

`POST /api/chat` → Go menyusun konteks (profil, 5 gap teratas, roadmap, statistik pasar, 6 pesan terakhir) → `POST /internal/chat` →
hybrid retrieval (pgvector + full-text OR-query, digabung Reciprocal Rank Fusion, top-k 6) → prompt dengan sumber bernomor `[n]` →
stream jawaban. Go meneruskan tiap event SSE (flush per event) dan menyimpan pertanyaan + jawaban + sitasi saat event `done`.
Setiap query dicatat sebagai log JSON `chat` (pertanyaan, chunk ter-retrieve + rank, sitasi, latensi, token).

```bash
docker compose exec ai-service python scripts/ask_cli.py      # 20 pertanyaan uji (data/eval/chat_questions.yaml)
docker compose cp ai-service:/app/logs/chat_review.md .       # lembar penilaian manual
```

Embedding default memakai `intfloat/multilingual-e5-base` lokal (diunduh ±1 GB saat pertama dipakai, disimpan di volume `hf_cache`). Jika `check_llm.py` menunjukkan endpoint menyediakan model embedding 768 dimensi, set `EMBEDDING_PROVIDER=api` dan `EMBEDDING_MODEL`.

## Pengembangan lokal

```bash
# backend (test butuh container db jalan; membuat database compass_test otomatis)
cd backend && make test
go run ./seed -dry-run -file ../data/taxonomy/skills.yaml

# ai-service
cd ai-service && uv venv .venv && uv pip install -p .venv -r requirements-dev.txt
.venv/bin/python -m pytest                      # test DB: set TEST_DATABASE_URL seperti di backend/Makefile
.venv/bin/python scripts/validate_raw_jobs.py   # cek data/raw_jobs/

# frontend
cd frontend && npm install && npm run dev
```

## Struktur

```
data/        raw_jobs/ (lowongan mentah), taxonomy/skills.yaml, eval/
backend/     cmd/api, internal/{config,handler,middleware,service,repository,aiclient,model}, migrations/, seed/
ai-service/  main.py, config.yaml, src/{ingestion,extraction,chunking,embeddings,vectordb,retrieval,prompts,llm,evaluation,api,utils}, scripts/, tests/
frontend/    app/, components/, lib/
```
