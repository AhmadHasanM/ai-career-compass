package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ahmadhasan/ai-career-compass/backend/internal/model"
)

type TaxonomyRepository struct {
	db *pgxpool.Pool
}

func NewTaxonomyRepository(db *pgxpool.Pool) *TaxonomyRepository {
	return &TaxonomyRepository{db: db}
}

func (r *TaxonomyRepository) ListRoles(ctx context.Context) ([]model.Role, error) {
	rows, err := r.db.Query(ctx, `SELECT id, name, slug FROM roles ORDER BY id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[model.Role])
}

// ListSkills mengembalikan taxonomy beserta alias dan prerequisite; category kosong = semua.
func (r *TaxonomyRepository) ListSkills(ctx context.Context, category string) ([]model.Skill, error) {
	rows, err := r.db.Query(ctx, `
		SELECT s.id, s.name, s.slug, s.category, s.description,
		       COALESCE((SELECT array_agg(a.alias ORDER BY a.alias)
		                 FROM skill_aliases a WHERE a.skill_id = s.id), '{}'),
		       COALESCE((SELECT array_agg(p.prerequisite_skill_id ORDER BY p.prerequisite_skill_id)
		                 FROM skill_prerequisites p WHERE p.skill_id = s.id), '{}')
		FROM skills s
		WHERE $1 = '' OR s.category = $1
		ORDER BY s.category, s.name`, category)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[model.Skill])
}

func (r *TaxonomyRepository) RoleExists(ctx context.Context, id int16) (bool, error) {
	var ok bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM roles WHERE id = $1)`, id).Scan(&ok)
	return ok, err
}

// MissingSkillIDs mengembalikan id dari daftar yang tidak ada di tabel skills.
func (r *TaxonomyRepository) MissingSkillIDs(ctx context.Context, ids []int) ([]int, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT t.id FROM unnest($1::int[]) AS t(id)
		WHERE NOT EXISTS (SELECT 1 FROM skills s WHERE s.id = t.id)
		ORDER BY t.id`, ids)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[int])
}
