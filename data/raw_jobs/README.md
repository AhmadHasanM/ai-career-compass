# Lowongan mentah (S1-T5)

Target Sprint 1: **minimal 50 lowongan AI Engineer di Indonesia**, dikumpulkan manual.

## Cara cepat: `add_job`

1. Buka halaman **detail** lowongan di browser (bukan halaman hasil pencarian), `Ctrl+A`, `Ctrl+C`.
2. Jalankan di terminal:

   ```bash
   cd ai-service
   .venv/bin/python scripts/add_job.py
   ```

3. Jawab pertanyaan singkat (URL, perusahaan, lokasi, dll.; Enter untuk melewati yang opsional).
   Sumber terdeteksi otomatis dari URL, `collected_at` terisi tanggal hari ini, URL duplikat ditolak.

Clipboard dibaca lewat `wl-paste` (Wayland: `sudo apt install wl-clipboard`) atau `xclip`/`xsel`.
Tanpa itu, jalankan dengan `--paste` lalu tempel teks di terminal dan tekan `Ctrl+D`.

## Format

Satu lowongan = satu file `.md`: front matter YAML + teks lowongan apa adanya. Salin `_TEMPLATE.md`.

Nama file: `YYYYMMDD-perusahaan-judul.md` (tanggal = `collected_at`), misal `20260926-gojek-ai-engineer.md`.

| Field | Wajib | Isi |
| --- | --- | --- |
| `title` | ya | Judul lowongan seperti tertulis |
| `source_name` | ya | LinkedIn, Glints, JobStreet, Kalibrr, Karir.com, situs karier perusahaan, dll. |
| `collected_at` | ya | Tanggal kamu menyalin (YYYY-MM-DD) |
| `source_url` | disarankan | URL lowongan; dipakai untuk sitasi dan deteksi duplikat |
| `posted_date` | opsional | Tanggal lowongan diposting, jika tertera |
| `company` | opsional | Nama perusahaan |
| `location` | opsional | Kota, atau `Remote` |
| `work_type` | opsional | `onsite` / `remote` / `hybrid` |
| `level` | opsional | `intern` / `junior` / `mid` / `senior` / `lead` |

Teks lowongan minimal 300 karakter. Role dan skill **tidak** perlu diisi: keduanya diekstrak LLM di Sprint 3.

## Tips

- Variasikan sumber dan level agar statistik demand tidak bias ke satu platform.
- Judul "ML Engineer", "LLM Engineer", "GenAI Engineer" boleh masuk jika isinya pekerjaan AI Engineer.
- Hindari repost lowongan yang sama di beberapa platform; validator menandai `source_url` duplikat.
- File diawali `_` (template) dan README diabaikan oleh validator dan ingestion.

## Validasi

```bash
cd ai-service
python scripts/validate_raw_jobs.py        # progres x/50 + daftar file bermasalah
```
