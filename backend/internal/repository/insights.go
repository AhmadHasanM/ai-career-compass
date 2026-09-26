package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ahmadhasan/ai-career-compass/backend/internal/model"
)

type InsightsRepository struct {
	db *pgxpool.Pool
}

func NewInsightsRepository(db *pgxpool.Pool) *InsightsRepository {
	return &InsightsRepository{db: db}
}

func (r *InsightsRepository) RoleBySlug(ctx context.Context, slug string) (model.Role, error) {
	var role model.Role
	err := r.db.QueryRow(ctx, `SELECT id, name, slug FROM roles WHERE slug = $1`, slug).
		Scan(&role.ID, &role.Name, &role.Slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return role, ErrNotFound
	}
	return role, err
}

func (r *InsightsRepository) RoleByID(ctx context.Context, id int16) (model.Role, error) {
	var role model.Role
	err := r.db.QueryRow(ctx, `SELECT id, name, slug FROM roles WHERE id = $1`, id).
		Scan(&role.ID, &role.Name, &role.Slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return role, ErrNotFound
	}
	return role, err
}

// Sample menghitung n lowongan (ekstraksi selesai) dan tanggal pengumpulan terbaru.
func (r *InsightsRepository) Sample(ctx context.Context, roleID int16, level *string) (int, *time.Time, error) {
	var (
		n        int
		snapshot *time.Time
	)
	err := r.db.QueryRow(ctx, `
		SELECT count(*), max(collected_at) FROM job_postings
		WHERE extraction_status = 'done' AND role_id = $1 AND ($2::text IS NULL OR level = $2)`,
		roleID, level).Scan(&n, &snapshot)
	return n, snapshot, err
}

// Demand membaca statistik per skill, urut permintaan tertinggi. limit <= 0 berarti semua.
// Tanpa filter level dipakai materialized view skill_demand; dengan level dihitung langsung
// dari lowongan karena view tidak punya dimensi level.
func (r *InsightsRepository) Demand(ctx context.Context, roleID int16, level *string, limit int) ([]model.DemandItem, error) {
	var lim *int
	if limit > 0 {
		lim = &limit
	}
	var (
		rows pgx.Rows
		err  error
	)
	if level == nil {
		rows, err = r.db.Query(ctx, `
			SELECT d.skill_id, s.name, s.slug, s.category, d.job_count, d.required_count,
			       d.demand_pct::float8, d.required_pct::float8
			FROM skill_demand d JOIN skills s ON s.id = d.skill_id
			WHERE d.role_id = $1
			ORDER BY d.demand_pct DESC, d.required_pct DESC, s.name
			LIMIT $2`, roleID, lim)
	} else {
		rows, err = r.db.Query(ctx, `
			WITH jobs AS (
				SELECT id FROM job_postings
				WHERE extraction_status = 'done' AND role_id = $1 AND level = $2
			), n AS (SELECT count(*) AS total FROM jobs)
			SELECT js.skill_id, s.name, s.slug, s.category,
			       count(*)::int,
			       (count(*) FILTER (WHERE js.requirement_type = 'required'))::int,
			       round(100.0 * count(*) / n.total, 2)::float8,
			       round(100.0 * count(*) FILTER (WHERE js.requirement_type = 'required') / n.total, 2)::float8
			FROM job_skills js
			JOIN jobs ON jobs.id = js.job_id
			JOIN skills s ON s.id = js.skill_id
			CROSS JOIN n
			GROUP BY js.skill_id, s.name, s.slug, s.category, n.total
			ORDER BY 7 DESC, 8 DESC, s.name
			LIMIT $3`, roleID, *level, lim)
	}
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[model.DemandItem])
}

func (r *InsightsRepository) Skill(ctx context.Context, id int) (model.Skill, error) {
	var s model.Skill
	err := r.db.QueryRow(ctx, `
		SELECT id, name, slug, category, description, '{}'::text[], '{}'::int[] FROM skills WHERE id = $1`, id).
		Scan(&s.ID, &s.Name, &s.Slug, &s.Category, &s.Description, &s.Aliases, &s.PrerequisiteIDs)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, ErrNotFound
	}
	return s, err
}

// EvidenceJobs: lowongan yang menyebut skill (bukti di balik persentase), terbaru dulu.
func (r *InsightsRepository) EvidenceJobs(ctx context.Context, skillID int, roleID int16, level *string, limit, offset int) ([]model.EvidenceJob, int, error) {
	var total int
	if err := r.db.QueryRow(ctx, `
		SELECT count(*) FROM job_skills js JOIN job_postings j ON j.id = js.job_id
		WHERE js.skill_id = $1 AND j.role_id = $2 AND j.extraction_status = 'done'
		  AND ($3::text IS NULL OR j.level = $3)`, skillID, roleID, level).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.Query(ctx, `
		SELECT j.id, j.title, j.company, j.level, j.location, j.work_type, j.source_name, j.source_url,
		       j.posted_date, j.collected_at, js.requirement_type
		FROM job_skills js JOIN job_postings j ON j.id = js.job_id
		WHERE js.skill_id = $1 AND j.role_id = $2 AND j.extraction_status = 'done'
		  AND ($3::text IS NULL OR j.level = $3)
		ORDER BY js.requirement_type = 'required' DESC, j.collected_at DESC, j.title
		LIMIT $4 OFFSET $5`, skillID, roleID, level, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	jobs, err := pgx.CollectRows(rows, pgx.RowToStructByPos[model.EvidenceJob])
	return jobs, total, err
}
