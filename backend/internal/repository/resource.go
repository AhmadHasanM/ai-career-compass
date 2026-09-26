package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ahmadhasan/ai-career-compass/backend/internal/model"
)

type ResourceRepository struct {
	db *pgxpool.Pool
}

func NewResourceRepository(db *pgxpool.Pool) *ResourceRepository {
	return &ResourceRepository{db: db}
}

const resourceColumns = `id, skill_id, title, url, type, level, language, is_free, est_hours::float8`

// Create menyimpan sumber belajar; URL yang sudah ada mengembalikan *DuplicateError.
func (r *ResourceRepository) Create(ctx context.Context, in model.ResourceInput) (model.LearningResource, error) {
	rows, err := r.db.Query(ctx, `
		INSERT INTO learning_resources (skill_id, title, url, type, level, language, is_free, est_hours)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING `+resourceColumns,
		in.SkillID, in.Title, in.URL, in.Type, in.Level, in.Language, in.IsFree, in.EstHours)
	if err != nil {
		return model.LearningResource{}, err
	}
	res, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByPos[model.LearningResource])
	if constraint, ok := uniqueViolation(err); ok && constraint == "learning_resources_url_key" {
		dup := &DuplicateError{Constraint: constraint}
		if lookupErr := r.db.QueryRow(ctx, `SELECT id::text FROM learning_resources WHERE url = $1`, in.URL).
			Scan(&dup.ExistingID); lookupErr != nil {
			return res, lookupErr
		}
		return res, dup
	}
	return res, err
}

// BySkills mengembalikan maksimal perSkill sumber per skill: gratis dulu, lalu level pemula dulu.
func (r *ResourceRepository) BySkills(ctx context.Context, skillIDs []int, perSkill int) (map[int][]model.LearningResource, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+resourceColumns+` FROM (
			SELECT lr.*, row_number() OVER (
				PARTITION BY skill_id
				ORDER BY is_free DESC,
				         CASE level WHEN 'beginner' THEN 0 WHEN 'intermediate' THEN 1 WHEN 'advanced' THEN 2 ELSE 3 END,
				         title
			) AS rn
			FROM learning_resources lr WHERE skill_id = ANY($1)
		) ranked
		WHERE $2 <= 0 OR rn <= $2
		ORDER BY skill_id, rn`, skillIDs, perSkill)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[model.LearningResource])
	if err != nil {
		return nil, err
	}
	out := map[int][]model.LearningResource{}
	for _, res := range list {
		out[res.SkillID] = append(out[res.SkillID], res)
	}
	return out, nil
}
