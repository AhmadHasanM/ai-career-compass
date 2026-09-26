"""Entry point ai-service. Hanya diakses backend Go di jaringan internal Docker."""

import logging
from contextlib import asynccontextmanager

from fastapi import FastAPI

from src.api import health
from src.utils.db import close_pool, open_pool

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s: %(message)s")


@asynccontextmanager
async def lifespan(_: FastAPI):
    await open_pool()
    yield
    await close_pool()


app = FastAPI(title="AI Career Compass - ai-service", lifespan=lifespan)
app.include_router(health.router)
