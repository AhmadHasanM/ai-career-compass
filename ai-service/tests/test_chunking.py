from src.chunking.job_chunker import chunk_job, chunk_text, split_sections

JOB = """PT Contoh sedang mencari AI Engineer untuk tim produk.

Tanggung Jawab:
- Membangun pipeline RAG
- Deploy model ke produksi

**Kualifikasi**
- Menguasai Python dan FastAPI
- Pengalaman dengan PyTorch

Nice to have:
- Pengalaman Kubernetes

Benefit
- Asuransi kesehatan
"""


def test_split_sections_detects_headings_id_and_en():
    secs = split_sections(JOB)
    assert [s.name for s in secs] == ["overview", "responsibilities", "qualifications", "benefits"]
    # "Nice to have" digabung ke kualifikasi yang berurutan
    assert "Kubernetes" in secs[2].text and "PyTorch" in secs[2].text
    assert "Tanggung Jawab" not in secs[1].text


def test_split_sections_without_headings_is_single_overview():
    secs = split_sections("Kami mencari engineer.\nWajib bisa Python.")
    assert len(secs) == 1 and secs[0].name == "overview"


def test_long_line_is_not_mistaken_for_heading():
    line = "Kualifikasi " + "x" * 80
    assert split_sections(line)[0].name == "overview"


def test_chunk_text_respects_size_and_overlap():
    lines = [f"baris nomor {i:03d} dengan isi yang cukup panjang" for i in range(60)]
    chunks = chunk_text("\n".join(lines), size=300, overlap=60)
    assert len(chunks) > 1
    assert all(len(c) <= 300 for c in chunks)
    # baris terakhir chunk sebelumnya muncul lagi di awal chunk berikutnya
    for prev, nxt in zip(chunks, chunks[1:]):
        assert prev.splitlines()[-1] in nxt
    # tidak ada baris yang hilang
    joined = "\n".join(chunks)
    assert all(line in joined for line in lines)


def test_chunk_text_splits_single_huge_line():
    chunks = chunk_text("a" * 2000, size=800, overlap=100)
    assert len(chunks) >= 3 and all(len(c) <= 800 for c in chunks)


def test_chunk_job_adds_context_header_and_sequential_index():
    chunks = chunk_job(title="AI Engineer", company="PT Contoh", raw_text=JOB, size=800, overlap=100)
    assert [c.index for c in chunks] == list(range(len(chunks)))
    assert chunks[0].content.startswith("AI Engineer — PT Contoh\nBagian: Ringkasan")
    quals = next(c for c in chunks if c.section == "qualifications")
    assert "Bagian: Kualifikasi" in quals.content and "FastAPI" in quals.content
