// Command seed memuat data/taxonomy/skills.yaml ke tabel roles, skills,
// skill_aliases, dan skill_prerequisites. Idempotent: aman dijalankan ulang.
//
//	go run ./seed -file ../data/taxonomy/skills.yaml
//	go run ./seed -file ../data/taxonomy/skills.yaml -dry-run   # validasi saja
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
)

func main() {
	file := flag.String("file", "../data/taxonomy/skills.yaml", "path ke skills.yaml")
	dryRun := flag.Bool("dry-run", false, "validasi YAML tanpa menulis ke database")
	flag.Parse()

	tax, err := LoadTaxonomy(*file)
	if err != nil {
		log.Fatal(err)
	}
	if err := tax.Validate(); err != nil {
		log.Fatal(err)
	}
	log.Printf("taxonomy valid: %d role, %d skill", len(tax.Roles), len(tax.Skills))
	if *dryRun {
		return
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL wajib diisi")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		log.Fatalf("koneksi database: %v", err)
	}
	defer conn.Close(ctx)

	stats, err := seed(ctx, conn, tax)
	if err != nil {
		log.Fatalf("seed gagal (rollback): %v", err)
	}
	log.Printf("seed selesai: %d role, %d skill, %d alias, %d prerequisite",
		stats.roles, stats.skills, stats.aliases, stats.prereqs)
	if len(stats.orphans) > 0 {
		log.Printf("peringatan: %d skill di database tidak ada di YAML (tidak dihapus): %v",
			len(stats.orphans), stats.orphans)
	}
}

type seedStats struct {
	roles, skills, aliases, prereqs int
	orphans                         []string
}

func seed(ctx context.Context, conn *pgx.Conn, tax *Taxonomy) (*seedStats, error) {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	stats := &seedStats{}

	for _, r := range tax.Roles {
		if _, err := tx.Exec(ctx, `
			INSERT INTO roles (name, slug) VALUES ($1, $2)
			ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name`,
			r.Name, r.Slug); err != nil {
			return nil, fmt.Errorf("role %s: %w", r.Slug, err)
		}
		stats.roles++
	}

	ids := make(map[string]int, len(tax.Skills))
	slugs := make([]string, 0, len(tax.Skills))
	for _, s := range tax.Skills {
		var id int
		if err := tx.QueryRow(ctx, `
			INSERT INTO skills (name, slug, category, description) VALUES ($1, $2, $3, $4)
			ON CONFLICT (slug) DO UPDATE
			SET name = EXCLUDED.name, category = EXCLUDED.category, description = EXCLUDED.description
			RETURNING id`,
			s.Name, s.Slug, s.Category, s.Description).Scan(&id); err != nil {
			return nil, fmt.Errorf("skill %s: %w", s.Slug, err)
		}
		ids[s.Slug] = id
		slugs = append(slugs, s.Slug)
		stats.skills++
	}

	// Alias dan prerequisite skill di YAML ditulis ulang penuh agar YAML jadi sumber kebenaran.
	skillIDs := make([]int, 0, len(ids))
	for _, id := range ids {
		skillIDs = append(skillIDs, id)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM skill_aliases WHERE skill_id = ANY($1)`, skillIDs); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM skill_prerequisites WHERE skill_id = ANY($1)`, skillIDs); err != nil {
		return nil, err
	}

	for _, s := range tax.Skills {
		for _, a := range s.AllAliases() {
			if _, err := tx.Exec(ctx, `INSERT INTO skill_aliases (skill_id, alias) VALUES ($1, $2)`,
				ids[s.Slug], a); err != nil {
				return nil, fmt.Errorf("alias %q (%s): %w", a, s.Slug, err)
			}
			stats.aliases++
		}
		for _, p := range s.Prerequisites {
			if _, err := tx.Exec(ctx, `
				INSERT INTO skill_prerequisites (skill_id, prerequisite_skill_id) VALUES ($1, $2)`,
				ids[s.Slug], ids[p]); err != nil {
				return nil, fmt.Errorf("prerequisite %s -> %s: %w", s.Slug, p, err)
			}
			stats.prereqs++
		}
	}

	rows, err := tx.Query(ctx, `SELECT slug FROM skills WHERE NOT (slug = ANY($1)) ORDER BY slug`, slugs)
	if err != nil {
		return nil, err
	}
	stats.orphans, err = pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}

	return stats, tx.Commit(ctx)
}
