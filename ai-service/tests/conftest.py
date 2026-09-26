"""Fixture database test. Test yang memakai `db_url` di-skip jika TEST_DATABASE_URL tidak di-set.

Schema di-reset lalu migrasi asli (backend/migrations) dijalankan, dengan advisory lock yang sama
seperti test Go agar keduanya tidak saling menimpa saat berjalan bersamaan.
"""

import os
from pathlib import Path

import psycopg
import pytest

MIGRATIONS = Path(__file__).resolve().parents[2] / "backend" / "migrations"
ADVISORY_LOCK_KEY = 72_202_609

FIXTURE_SQL = """
INSERT INTO roles (name, slug) VALUES ('AI Engineer', 'ai-engineer'), ('ML Engineer', 'ml-engineer');
INSERT INTO skills (name, slug, category) VALUES
    ('Python', 'python', 'language'), ('Retrieval-Augmented Generation', 'rag', 'llm'),
    ('Docker', 'docker', 'mlops'), ('PostgreSQL', 'postgresql', 'data');
INSERT INTO skill_aliases (skill_id, alias)
    SELECT DISTINCT id, a FROM skills, unnest(ARRAY[lower(name), slug]) a;
INSERT INTO skill_aliases (skill_id, alias) SELECT id, 'postgres' FROM skills WHERE slug = 'postgresql';
"""

TEST_DB_URL = os.getenv("TEST_DATABASE_URL", "")
if TEST_DB_URL:
    # Settings ai-service membaca DATABASE_URL; arahkan ke database test sebelum modul app di-import.
    os.environ["DATABASE_URL"] = TEST_DB_URL
    os.environ["INTERNAL_TOKEN"] = "test-internal-token"


@pytest.fixture
def anyio_backend():
    return "asyncio"


@pytest.fixture(scope="session")
def _migrated_db():
    if not TEST_DB_URL:
        pytest.skip("TEST_DATABASE_URL tidak di-set; lewati test database")
    lock = psycopg.connect(TEST_DB_URL, autocommit=True)
    lock.execute("SELECT pg_advisory_lock(%s)", (ADVISORY_LOCK_KEY,))
    try:
        lock.execute("DROP SCHEMA public CASCADE; CREATE SCHEMA public;")
        for f in sorted(MIGRATIONS.glob("*.up.sql")):
            lock.execute(f.read_text())
        lock.execute(FIXTURE_SQL)
        yield TEST_DB_URL
    finally:
        lock.execute("SELECT pg_advisory_unlock(%s)", (ADVISORY_LOCK_KEY,))
        lock.close()


@pytest.fixture
def db_url(_migrated_db):
    with psycopg.connect(_migrated_db, autocommit=True) as conn:
        conn.execute("TRUNCATE user_sessions, job_postings, unmapped_skills, document_chunks, learning_resources CASCADE")
        conn.execute("REFRESH MATERIALIZED VIEW skill_demand")
    return _migrated_db
