"""Connection pool Postgres bersama untuk ai-service."""

from pgvector.psycopg import register_vector_async
from psycopg_pool import AsyncConnectionPool

from src.utils.config import get_settings

_pool: AsyncConnectionPool | None = None


async def _configure(conn) -> None:
    # Tipe vector harus didaftarkan per koneksi agar parameter Vector bisa dikirim.
    await register_vector_async(conn)


async def open_pool(dsn: str | None = None) -> AsyncConnectionPool:
    global _pool
    if _pool is None:
        _pool = AsyncConnectionPool(
            dsn or get_settings().database_url, min_size=1, max_size=5, open=False, configure=_configure
        )
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
