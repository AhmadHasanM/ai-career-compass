"""Connection pool Postgres bersama untuk ai-service."""

from psycopg_pool import AsyncConnectionPool

from src.utils.config import get_settings

_pool: AsyncConnectionPool | None = None


async def open_pool() -> AsyncConnectionPool:
    global _pool
    if _pool is None:
        _pool = AsyncConnectionPool(get_settings().database_url, min_size=1, max_size=5, open=False)
        await _pool.open()
    return _pool


async def close_pool() -> None:
    global _pool
    if _pool is not None:
        await _pool.close()
        _pool = None


def get_pool() -> AsyncConnectionPool:
    if _pool is None:
        raise RuntimeError("database pool belum dibuka")
    return _pool
