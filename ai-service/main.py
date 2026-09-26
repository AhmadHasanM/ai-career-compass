"""Entry point ai-service. Hanya diakses backend Go di jaringan internal Docker."""

import asyncio
import logging
import os
from contextlib import asynccontextmanager

from fastapi import FastAPI

from src.api import chat, health, internal, roadmap
from src.embeddings.embedder import get_embedder
from src.ingestion.factory import build_chat_service, build_processor, build_resource_embedder
from src.utils.db import close_pool, open_pool
from src.utils.logging import setup_logging

setup_logging(os.getenv("LOG_LEVEL", "INFO"))


log = logging.getLogger("main")


async def warm_up_embedder() -> None:
    """Muat model embedding lokal di awal agar chat / ingestion pertama tidak menunggu ±20 detik."""
    try:
        await get_embedder().embed_query("warm up")
        log.info("model embedding siap")
    except Exception:  # noqa: BLE001 - gagal warm-up tidak fatal; model dimuat saat dipakai
        log.exception("warm-up model embedding gagal")


@asynccontextmanager
async def lifespan(app: FastAPI):
    pool = await open_pool()
    app.state.processor = build_processor(pool)
    app.state.resource_embedder = build_resource_embedder(pool)
    app.state.chat_service = build_chat_service(pool)
    warmup = asyncio.create_task(warm_up_embedder())
    yield
    warmup.cancel()
    await close_pool()


app = FastAPI(title="AI Career Compass - ai-service", lifespan=lifespan)
app.include_router(health.router)
app.include_router(internal.router)
app.include_router(roadmap.router)
app.include_router(chat.router)
