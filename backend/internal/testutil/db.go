// Package testutil menyiapkan database test: reset schema, jalankan migrasi asli, dan isi fixture taxonomy.
//
// Pakai dari TestMain:
//
//	func TestMain(m *testing.M) { os.Exit(testutil.RunWithDB(m)) }
//
// lalu di test: db := testutil.DB(t). Tanpa TEST_DATABASE_URL, test yang butuh DB di-skip.
package testutil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// advisoryLockKey menyerialkan package test yang memakai database yang sama.
const advisoryLockKey = 72_202_609

var pool *pgxpool.Pool

func RunWithDB(m *testing.M) int {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		return m.Run()
	}
	ctx := context.Background()

	lockConn, err := pgx.Connect(ctx, url)
	if err != nil {
		fmt.Fprintln(os.Stderr, "testutil: koneksi:", err)
		return 1
	}
	defer lockConn.Close(ctx)
	if _, err := lockConn.Exec(ctx, "SELECT pg_advisory_lock($1)", advisoryLockKey); err != nil {
		fmt.Fprintln(os.Stderr, "testutil: advisory lock:", err)
		return 1
	}
	defer lockConn.Exec(ctx, "SELECT pg_advisory_unlock($1)", advisoryLockKey) //nolint:errcheck

	if err := migrateFresh(ctx, lockConn); err != nil {
		fmt.Fprintln(os.Stderr, "testutil: migrasi:", err)
		return 1
	}
	pool, err = pgxpool.New(ctx, url)
	if err != nil {
		fmt.Fprintln(os.Stderr, "testutil: pool:", err)
		return 1
	}
	defer pool.Close()
	return m.Run()
}

// DB mengembalikan pool dengan data bersih (hanya fixture taxonomy).
func DB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if pool == nil {
		t.Skip("TEST_DATABASE_URL tidak di-set; lewati test database")
	}
	_, err := pool.Exec(context.Background(), `
		TRUNCATE user_sessions, job_postings, unmapped_skills, document_chunks, chat_messages CASCADE;
		REFRESH MATERIALIZED VIEW skill_demand;`)
	if err != nil {
		t.Fatalf("reset data: %v", err)
	}
	return pool
}

func migrationsDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "migrations")
}

func migrateFresh(ctx context.Context, conn *pgx.Conn) error {
	if _, err := conn.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`); err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(migrationsDir(), "*.up.sql"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("tidak ada file migrasi di %s", migrationsDir())
	}
	sort.Strings(files)
	for _, f := range files {
		sql, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		if _, err := conn.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(f), err)
		}
	}
	_, err = conn.Exec(ctx, fixtureSQL)
	return err
}

// Fixture taxonomy kecil: 2 role, 4 skill, alias, dan prerequisite.
const fixtureSQL = `
INSERT INTO roles (name, slug) VALUES ('AI Engineer', 'ai-engineer'), ('ML Engineer', 'ml-engineer');
INSERT INTO skills (name, slug, category, description) VALUES
	('Python', 'python', 'language', 'Bahasa utama AI'),
	('Retrieval-Augmented Generation', 'rag', 'llm', NULL),
	('Docker', 'docker', 'mlops', NULL),
	('PostgreSQL', 'postgresql', 'data', NULL);
INSERT INTO skill_aliases (skill_id, alias)
	SELECT DISTINCT id, a FROM skills, unnest(ARRAY[lower(name), slug]) a;
INSERT INTO skill_aliases (skill_id, alias) SELECT id, 'postgres' FROM skills WHERE slug = 'postgresql';
INSERT INTO skill_prerequisites (skill_id, prerequisite_skill_id)
	SELECT s.id, p.id FROM skills s, skills p WHERE s.slug = 'rag' AND p.slug = 'python';
`

func RoleID(t *testing.T, slug string) int16 {
	t.Helper()
	var id int16
	if err := pool.QueryRow(context.Background(), `SELECT id FROM roles WHERE slug = $1`, slug).Scan(&id); err != nil {
		t.Fatalf("role %s: %v", slug, err)
	}
	return id
}

func SkillID(t *testing.T, slug string) int {
	t.Helper()
	var id int
	if err := pool.QueryRow(context.Background(), `SELECT id FROM skills WHERE slug = $1`, slug).Scan(&id); err != nil {
		t.Fatalf("skill %s: %v", slug, err)
	}
	return id
}

// LongText menghasilkan teks lowongan yang lolos batas minimal karakter.
func LongText() string {
	return strings.Repeat("Kualifikasi: Python, RAG, Docker, dan PostgreSQL. ", 10)
}
