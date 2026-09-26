# AI Career Compass

Roadmap belajar karier AI yang disusun dari **data lowongan kerja nyata di Indonesia**. Setiap persentase bisa
ditelusuri ke lowongan asalnya, dan setiap jawaban chatbot membawa sitasi yang bisa diklik. MVP fokus ke role
**AI Engineer**.

> Video demo (2–3 menit): **[LINK VIDEO DEMO]**

## Fitur MVP

| Layar | Apa yang dilakukan |
| --- | --- |
| **Profil** | Latar belakang, target role, jam belajar/minggu, dan skill dari taxonomy (pencarian mengenali alias: `postgres`, `k8s`, …) |
| **Market insight** | 15 skill paling diminta dengan `n` lowongan + tanggal snapshot, filter level, dan panel bukti berisi lowongan asal tiap persentase |
| **Skill gap** | Skill yang diminta ≥ 20% lowongan tapi belum dimiliki, diurutkan skor prioritas, plus cakupan permintaan yang sudah dikuasai |
| **Roadmap** | Graf React Flow: urutan belajar deterministik dari prasyarat, dengan alasan, estimasi minggu, dan sumber belajar per node |
| **Tanya** | Chat RAG streaming: jawaban dari lowongan, sumber belajar, dan statistik pasar, dengan penanda sitasi `[n]` dan panel sumber |

## Arsitektur

```mermaid
flowchart LR
  FE["frontend<br/>Next.js 16"] -->|REST + SSE| BE["backend<br/>Go / Gin"]
  BE -->|REST + X-Internal-Token| AI["ai-service<br/>FastAPI"]
  BE --> DB[("PostgreSQL 16<br/>+ pgvector")]
  AI --> DB
  AI -->|OpenAI-compatible| LLM["LLM<br/>(Groq · gpt-oss-120b)"]
  AI --> EMB["multilingual-e5-base<br/>(lokal, CPU)"]
```

| Service | Stack | Port host | Tanggung jawab | Tabel yang ditulis |
| --- | --- | --- | --- | --- |
| `frontend` | Next.js 16, Tailwind v4, React Flow | 3000 | UI sesuai `frontend/DESIGN.md` | - |
| `backend` | Go, Gin, pgx, golang-migrate | 8080 | API publik, sesi & profil, taxonomy, skill demand, gap scoring, topological sort roadmap, admin, rate limit, proxy SSE chat, **pemilik skema** | semua selain milik ai-service |
| `ai-service` | Python, FastAPI, Pydantic | tidak diekspos | Ekstraksi lowongan (LLM), normalisasi skill, chunking, embedding, hybrid retrieval, prompt, chat RAG, penjelasan roadmap | `document_chunks`, `job_skills`, `unmapped_skills`, status ekstraksi |
| `db` | PostgreSQL 16 + pgvector | 5433 | Data relasional, vektor (HNSW), full-text (GIN) | - |

Service sekali jalan: `migrate` (golang-migrate) dan `seed` (taxonomy dari `data/taxonomy/skills.yaml`).

### Alur utama

- **Ingestion**: `ingest_cli send` → `POST /api/admin/jobs` (Go simpan `pending`) → `POST /internal/jobs/{id}/process` (202) →
  background: ekstraksi LLM (JSON + Pydantic, retry dengan umpan balik error) → normalisasi ke `skill_aliases` (tak dikenal ke
  `unmapped_skills`) → chunk per bagian lowongan → embedding → `document_chunks` → refresh materialized view `skill_demand`.
- **Roadmap**: Go ambil gap → tambah prasyarat yang belum dimiliki (maks. 20 node) → topological sort (seri dipecah skor prioritas)
  → `POST /internal/roadmap/explain` untuk alasan + estimasi → Go **menolak skill di luar input** → simpan versi baru.
- **Chat**: Go susun konteks (profil, 5 gap teratas, roadmap, statistik pasar, 6 pesan terakhir) → `POST /internal/chat` →
  hybrid retrieval (vector + full-text, Reciprocal Rank Fusion, top-k 6) → prompt bersumber bernomor → stream SSE. Go meneruskan
  tiap event (flush per event) dan menyimpan pertanyaan + jawaban + sitasi saat event `done`.

## Keputusan desain

| Keputusan | Alasan |
| --- | --- |
| **Urutan roadmap ditentukan kode, bukan LLM** | Deterministik dan bisa diuji; LLM hanya menjelaskan. Output LLM divalidasi, dan roadmap tetap jadi dengan penjelasan fallback bila LLM gagal atau lambat. |
| **Prasyarat taxonomy = urutan belajar praktis** | Versi awal yang terlalu teoretis (RAG → deep learning → aljabar linear) menghasilkan 16 node; disederhanakan menjadi jalur AI Engineer terapan. |
| **Skill demand di materialized view** | Dashboard dan gap tidak menghitung ulang dari nol; di-refresh setiap ingestion selesai. Filter level dihitung langsung karena view tidak berdimensi level. |
| **Full-text dengan tsquery OR + stopword** | `websearch_to_tsquery` memakai AND, sehingga pertanyaan panjang hampir tidak pernah cocok; OR + `ts_rank_cd` tetap memeringkat chunk yang cocok lebih banyak kata. |
| **Sitasi hanya dari nomor yang disebut jawaban** | Panel sumber menampilkan yang benar-benar dipakai, bukan semua chunk ter-retrieve. |
| **`document_chunks` polymorphic + trigger** | Lowongan dan sumber belajar berbagi satu index hybrid; trigger menghapus chunk saat sumbernya dihapus. |
| **Embedding lokal `multilingual-e5-base`** | Mendukung bahasa Indonesia, tanpa biaya per request; prefix `query:` / `passage:` sesuai model e5. |
| **Sesi anonim tanpa login** | Cukup untuk MVP; `X-Session-Id` divalidasi ke database pada setiap request pengguna. |
| **ai-service tidak diekspos** | Hanya backend yang memanggilnya, dengan `X-Internal-Token` (constant-time compare). |

## Metrik (terukur pada 2026-09-27)

| Metrik | Hasil | Target |
| --- | --- | --- |
| Isi profil → roadmap tampil (browser, Gemini) | 5.1 detik | ≤ 30 detik |
| Generate roadmap saat ai-service mati (fallback) | 0.2 detik | tetap jadi |
| Latensi jawaban chat lewat `/api/chat` (Groq `gpt-oss-120b`) | token pertama 0.7–1.9 detik, total 1.5–2.3 detik | p95 ≤ 8 detik |
| Acceptance criteria MVP di browser (Playwright) | 11/11 cek lolos untuk AC1–AC3 dan penanganan error. AC4 (sitasi) dan AC5 (penolakan di luar domain) terverifikasi lewat API dengan Groq; **belum diuji ulang di browser** | semua |
| Test otomatis | 71 test Go (+ race detector), 77 test Python | lolos |
| Akurasi ekstraksi skill | **[BELUM DIUKUR]** — butuh validasi manual 20 lowongan nyata (`ingest_cli review`) | ≥ 85% |

## Menjalankan

```bash
cp .env.example .env          # isi LLM_BASE_URL, LLM_API_KEY, LLM_MODEL; ganti ADMIN_TOKEN dan INTERNAL_TOKEN
docker compose up -d --build  # db -> migrate -> seed -> backend, ai-service, frontend
```

Buka http://localhost:3000. Cek kesehatan: `curl localhost:8080/health`, `curl localhost:3000/health`.

LLM memakai endpoint OpenAI-compatible. Default: Groq (`LLM_BASE_URL=https://api.groq.com/openai/v1`,
`LLM_MODEL=openai/gpt-oss-120b`, free tier ±1.000 request/hari dan 8.000 token/menit). `reasoning_effort` di
`ai-service/config.yaml` menjaga model reasoning tetap hemat token; parameter opsional yang ditolak model dikirim ulang tanpanya.

### Mengisi data

```bash
# 1. kumpulkan lowongan (salin teks dari halaman detail lowongan di browser)
cd ai-service && .venv/bin/python scripts/add_job.py --paste

# 2. cek koneksi LLM + embedding, kirim lowongan, pantau
docker compose exec ai-service python scripts/check_llm.py
docker compose exec ai-service python scripts/ingest_cli.py send
docker compose exec ai-service python scripts/ingest_cli.py status

# 3. sumber belajar terkurasi (98 sumber, URL dicek) dan evaluasi
docker compose exec ai-service python scripts/ingest_cli.py resources --embed-missing   # --reembed-all setelah format teks berubah
docker compose exec ai-service python scripts/ingest_cli.py review -n 20   # validasi manual ekstraksi
docker compose exec ai-service python scripts/ask_cli.py                  # 20 pertanyaan uji chatbot
```

## API backend

Error berbentuk `{"error": {"code", "message", "fields?"}}`. Endpoint pengguna butuh `X-Session-Id` (dari `POST /api/sessions`),
endpoint admin butuh `X-Admin-Token`.

| Method | Path | Auth | Keterangan |
| --- | --- | --- | --- |
| GET | `/health` | - | Status service + database |
| POST | `/api/sessions` | - | Buat sesi anonim (dibatasi per IP per menit) |
| GET | `/api/roles`, `/api/skills?category=` | - | Role target dan taxonomy (alias + prasyarat) |
| GET / PUT | `/api/profile` | sesi | Profil + skill; PUT mengganti seluruh profil |
| GET | `/api/insights/skill-demand?role=&level=&limit=` | - | Top skill + n + snapshot (`small_sample` jika n < 30) |
| GET | `/api/insights/skill-demand/{skill_id}/jobs` | - | Lowongan asal sebuah persentase |
| GET | `/api/gap` | sesi | Skill gap + skor prioritas + cakupan |
| POST / GET | `/api/roadmap/generate`, `/api/roadmap` | sesi | Generate roadmap baru / ambil versi terbaru |
| GET | `/api/resources?skill_id=` | - | Sumber belajar per skill |
| POST | `/api/chat` | sesi | Chat RAG, SSE: `token` berulang lalu `done` (jawaban, sitasi, usage) atau `error` |
| GET | `/api/chat/history` | sesi | Riwayat chat beserta sitasi |
| POST | `/api/admin/jobs`, `/api/admin/resources` | admin | Tambah lowongan / sumber belajar, lalu picu pemrosesan |

Rate limit per IP untuk endpoint publik (`RATE_LIMIT_RPS`, `RATE_LIMIT_BURST`), generate roadmap per IP, chat per sesi; 429
menyertakan `Retry-After`. Endpoint admin tidak ikut dibatasi karena dilindungi token dan dipakai ingestion massal.

## Pengembangan

```bash
cd backend && make test                          # butuh container db; database compass_test dibuat otomatis
cd ai-service && uv venv .venv && uv pip install -p .venv -r requirements-dev.txt
TEST_DATABASE_URL=postgres://compass:compass@localhost:5433/compass_test?sslmode=disable .venv/bin/python -m pytest
cd frontend && npm install && npm run lint && npm run dev
```

Test Go dan Python memakai migrasi asli dan advisory lock yang sama, sehingga bisa dijalankan bersamaan tanpa saling menimpa.

## Keterbatasan yang diketahui

- **Data**: statistik baru bermakna setelah ≥ 30 lowongan nyata terkumpul; UI menampilkan peringatan sampel kecil di bawah itu.
- **Kuota LLM**: free tier Groq ±1.000 request/hari dan 8.000 token/menit (±8 ekstraksi atau ±2 jawaban chat per menit);
  SDK mengulang otomatis saat 429 dan UI menampilkan pesan jelas saat kuota habis. Gemini free tier (±20 request/hari) tidak cukup.
- **Observability**: setiap query chat dicatat sebagai log JSON (pertanyaan, chunk + rank, sitasi, latensi, token); integrasi Langfuse
  menunggu kunci.
- **Rate limit** disimpan in-memory; cukup untuk satu instance backend.

## Struktur

```
data/        raw_jobs/ (lowongan mentah), taxonomy/skills.yaml, resources/learning_resources.yaml, eval/chat_questions.yaml
backend/     cmd/api, internal/{config,handler,middleware,service,repository,aiclient,model,httpx,logger,testutil}, migrations/, seed/
ai-service/  main.py, config.yaml, src/{ingestion,extraction,chunking,embeddings,vectordb,retrieval,chat,roadmap,prompts,llm,api,utils},
             scripts/{add_job,ingest_cli,ask_cli,check_llm,validate_raw_jobs}.py, tests/
frontend/    DESIGN.md, app/{page,dashboard,gap,roadmap,chat}, components/, lib/api.ts
```
