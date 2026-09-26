from fastapi import APIRouter
from fastapi.responses import JSONResponse

from src.utils.db import get_pool

router = APIRouter()


@router.get("/health")
async def health() -> JSONResponse:
    try:
        async with get_pool().connection(timeout=2) as conn:
            await conn.execute("SELECT 1")
    except Exception:
        return JSONResponse(
            status_code=503,
            content={"status": "degraded", "service": "ai-service", "database": "unreachable"},
        )
    return JSONResponse(content={"status": "ok", "service": "ai-service", "database": "ok"})
