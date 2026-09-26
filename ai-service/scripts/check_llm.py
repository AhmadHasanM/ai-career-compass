"""Verifikasi koneksi LLM dan ketersediaan model embedding (S3-T1).

    docker compose exec ai-service python scripts/check_llm.py

Memeriksa: (1) chat completion dengan LLM_MODEL, (2) JSON mode, (3) embedding lewat API dengan
EMBEDDING_MODEL, (4) embedding lokal. Hasil (3) menentukan EMBEDDING_PROVIDER yang dipakai.
"""

import asyncio
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from src.embeddings.embedder import ApiEmbedder, LocalEmbedder  # noqa: E402
from src.llm.client import LLMNotConfiguredError, OpenAICompatibleLLM  # noqa: E402
from src.utils.config import get_settings  # noqa: E402


async def main() -> int:
    s = get_settings()
    ok = True

    print(f"LLM_BASE_URL={s.llm_base_url or '(kosong)'}  LLM_MODEL={s.llm_model or '(kosong)'}")
    try:
        llm = OpenAICompatibleLLM(s)
        r = await llm.chat([{"role": "user", "content": "Balas satu kata: siap"}], max_tokens=300)
        print(f"✓ chat: {r.content.strip()!r} ({r.model}, {r.latency_ms} ms)")
        r = await llm.chat([{"role": "user", "content": 'Balas JSON {"ok": true}'}], max_tokens=300, json_mode=True)
        mode = "didukung" if llm._json_mode_supported else "tidak didukung (pakai instruksi prompt)"
        print(f"✓ JSON mode {mode}: {r.content.strip()[:60]!r}")
    except LLMNotConfiguredError as e:
        ok = False
        print(f"✗ chat: {e}")
    except Exception as e:  # noqa: BLE001
        ok = False
        print(f"✗ chat: {type(e).__name__}: {e}")

    if s.llm_base_url and s.llm_api_key:
        try:
            emb = ApiEmbedder(s)
            v = await emb.embed_query("tes")
            print(f"✓ embedding API {s.embedding_model}: dimensi {len(v)} → bisa pakai EMBEDDING_PROVIDER=api")
        except Exception as e:  # noqa: BLE001
            print(f"- embedding API {s.embedding_model} tidak tersedia ({type(e).__name__}); pakai EMBEDDING_PROVIDER=local")

    try:
        local = LocalEmbedder("intfloat/multilingual-e5-base", 768)
        v = await local.embed_query("tes")
        print(f"✓ embedding lokal multilingual-e5-base: dimensi {len(v)}")
    except Exception as e:  # noqa: BLE001
        ok = False
        print(f"✗ embedding lokal: {type(e).__name__}: {e}")

    print(f"\nEMBEDDING_PROVIDER aktif: {s.embedding_provider}")
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(asyncio.run(main()))
