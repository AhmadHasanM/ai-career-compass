"""Entry point ai-service. Hanya diakses backend Go di jaringan internal Docker."""

import os
from contextlib import asynccontextmanager

from fastapi import FastAPI

from src.api import health, internal, roadmap
from src.ingestion.factory import build_processor, build_resource_embedder
from src.utils.db import close_pool, open_pool
from src.utils.logging import setup_logging

setup_logging(os.getenv("LOG_LEVEL", "INFO"))


@asynccontextmanager
async def lifespan(app: FastAPI):
    pool = await open_pool()
    app.state.processor = build_processor(pool)
    app.state.resource_embedder = build_resource_embedder(pool)
    yield
    await close_pool()


app = FastAPI(title="AI Career Compass - ai-service", lifespan=lifespan)
app.include_router(health.router)
app.include_router(internal.router)
app.include_router(roadmap.router)
