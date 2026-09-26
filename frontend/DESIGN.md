# Evidence Ledger — design system AI Career Compass

Acuan semua UI frontend. Sumber: canvas "AI Career Compass — Design System" (artboard Design System,
Market Insight, Chat RAG). Token diterapkan di `app/globals.css` (`@theme`) dan komponen di `components/`.

Antarmuka terasa seperti laporan riset yang rapi: tenang, padat informasi, dan setiap angka bisa ditelusuri
ke lowongan asalnya.

## Prinsip

1. **Setiap angka punya sumber.** Persentase selalu disertai n lowongan dan tanggal snapshot. Jawaban chatbot
   selalu membawa sitasi yang bisa diklik.
2. **Satu aksen, dipakai hemat.** Hijau hanya untuk aksi utama, skill yang sudah dimiliki, dan data yang sedang
   disorot. Selebihnya tinta dan kertas.
3. **Garis, bukan bayangan.** Hierarki dari border 1 px, spasi, dan bobot huruf. Bayangan hanya untuk elemen
   yang benar-benar melayang (popover).

## Warna

| Token | Hex | Peran |
| --- | --- | --- |
| `paper` | `#F7F6F2` | Latar aplikasi |
| `surface` | `#FFFFFF` | Panel, kartu, input |
| `sunken` | `#EFEDE7` | Track bar, baris hover / terpilih, nav aktif |
| `line` | `#E2DFD7` | Border default |
| `line-strong` | `#C9C5BA` | Border kontrol |
| `ink` | `#16181D` | Teks utama, data netral |
| `ink-2` | `#3D424B` | Teks sekunder |
| `muted` | `#5E6470` | Caption, meta (5.4:1) |
| `compass` | `#1E6B52` | Aksen: aksi utama, skill dimiliki |
| `compass-strong` | `#17563F` | Hover aksen, teks di atas `compass-soft` |
| `compass-soft` | `#E4EFEA` | Latar aksen, focus ring |
| `gap` | `#9A5B0B` | Skill gap, peringatan |
| `gap-strong` | `#7A4508` | Teks di atas `gap-soft` |
| `gap-soft` | `#F6EBDA` | Latar chip gap / peringatan |
| `error` | `#A83A32` | Error, destruktif |

## Tipografi

IBM Plex Sans untuk semua teks UI (400 / 500 / 600). IBM Plex Mono hanya untuk angka, sitasi, kode, dan label
meta. Angka tabel memakai tabular figures.

| Gaya | Ukuran / line-height | Pakai |
| --- | --- | --- |
| display | 32/40, 600, -0.01em | Judul halaman besar |
| h1 | 24/32, 600 | Judul layar |
| h2 | 18/26, 600 | Judul panel |
| body | 15/24 | Teks |
| small | 13/20, `muted` | Meta: snapshot, n |
| data | Mono 14/20 | Persentase, sitasi, durasi |
| label meta | Mono 12–13, uppercase, letter-spacing 0.04–0.06em, `muted` | Eyebrow ("BUKTI", "JAWABAN · 3 SUMBER") |

## Spasi & radius

Grid 4 px: 4, 8, 12, 16, 24, 32, 48, 64. Radius 4 (chip), 6 (kontrol), 8 (panel). Popover: radius 8 +
`0 4px 16px rgba(22,24,29,0.08)`.

## Komponen

- **Tombol**: tinggi 40, padding 0 16, radius 6, 14/500. Primer = `compass` penuh; sekunder = putih + border
  `line-strong`; ghost = transparan. Satu tombol primer per layar. Label kata kerja spesifik ("Buat roadmap"),
  bukan "Submit".
- **Input**: tinggi 40, radius 6, border `line-strong`, 15 px. Label selalu di atas field (13/500 `ink-2`),
  hint 12 px `muted`. Fokus: border `compass` + ring 3 px `compass-soft`.
- **Chip skill**: tinggi 28, radius 4, 13/500. Dimiliki = `compass-soft` + teks `compass-strong` + ikon centang.
  Gap = `gap-soft` + teks `gap-strong` + kata "gap". Netral = border `line-strong`. Status tidak pernah dibedakan
  oleh warna saja.
- **Baris demand**: grid rank (mono) · nama · bar 8 px (track `sunken`, isi `compass` jika dimiliki, `ink` jika
  belum) · persen (mono, rata kanan). Baris adalah tombol; terpilih = latar `sunken`.
- **Sitasi**: penanda `[n]` berupa link nyata, Mono 12, border `line-strong`, radius 4, teks `compass-strong`;
  aktif = border `compass` + latar `compass-soft`. Klik menyorot kartu sumber di panel kanan.
- **Status node**: Belum (lingkaran kosong), Berjalan (setengah), Selesai (penuh, hijau). Bentuk ikon membawa
  arti yang sama dengan warnanya.
- **Panel**: latar `surface`, border `line`, radius 8, padding 24.

## Layout

App shell: sidebar 232 px (logo kompas, nav Profil · Market insight · Skill gap · Roadmap · Tanya, tanggal
snapshot di bawah), konten dengan padding 40/48. Layar chat: percakapan maks. 800 px di tengah, panel Sumber
360 px di kanan.

## Hindari

- Gradien latar, glassmorphism, glow
- Ikon sparkle / bintang untuk menandai AI
- Emoji di UI dan copy
- Kartu dengan border kiri berwarna
- Ungu-biru "AI" dan font Inter / Roboto
- Copy "Powered by AI", "Unlock your potential"
- Angka tanpa n dan tanggal snapshot
- Shadow besar dan radius di atas 8 px
