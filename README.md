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

## Pengembangan lokal

```bash
# backend
cd backend && go test ./... && go run ./seed -dry-run -file ../data/taxonomy/skills.yaml

# ai-service
cd ai-service && uv venv .venv && uv pip install -p .venv -r requirements-dev.txt
.venv/bin/python -m pytest
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
