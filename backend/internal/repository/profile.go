package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ahmadhasan/ai-career-compass/backend/internal/model"
)

type ProfileRepository struct {
	db *pgxpool.Pool
}

func NewProfileRepository(db *pgxpool.Pool) *ProfileRepository {
	return &ProfileRepository{db: db}
}

type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (r *ProfileRepository) GetBySession(ctx context.Context, sessionID uuid.UUID) (*model.Profile, error) {
	return getProfile(ctx, r.db, sessionID)
}

// Upsert menyimpan profil sesi dan mengganti daftar skill-nya dalam satu transaksi.
// Skill yang sudah ada mempertahankan source-nya (manual/cv); skill baru bersumber manual.
func (r *ProfileRepository) Upsert(ctx context.Context, sessionID uuid.UUID, in model.ProfileInput) (*model.Profile, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var profileID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO user_profiles (session_id, education, current_job, target_role_id, hours_per_week)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (session_id) DO UPDATE SET
			education      = EXCLUDED.education,
			current_job    = EXCLUDED.current_job,
			target_role_id = EXCLUDED.target_role_id,
			hours_per_week = EXCLUDED.hours_per_week,
			updated_at     = now()
		RETURNING id`,
		sessionID, in.Education, in.CurrentJob, in.TargetRoleID, in.HoursPerWeek).Scan(&profileID); err != nil {
		return nil, err
	}

	skillIDs := make([]int, len(in.Skills))
	proficiencies := make([]string, len(in.Skills))
	for i, s := range in.Skills {
		skillIDs[i], proficiencies[i] = s.SkillID, s.Proficiency
	}

	if _, err := tx.Exec(ctx, `
		DELETE FROM user_skills WHERE profile_id = $1 AND NOT (skill_id = ANY($2::int[]))`,
		profileID, skillIDs); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO user_skills (profile_id, skill_id, proficiency, source)
		SELECT $1, s.skill_id, s.proficiency, 'manual'
		FROM unnest($2::int[], $3::text[]) AS s(skill_id, proficiency)
		ON CONFLICT (profile_id, skill_id) DO UPDATE SET proficiency = EXCLUDED.proficiency`,
		profileID, skillIDs, proficiencies); err != nil {
		return nil, err
	}

	p, err := getProfile(ctx, tx, sessionID)
	if err != nil {
		return nil, err
	}
	return p, tx.Commit(ctx)
}

func getProfile(ctx context.Context, q querier, sessionID uuid.UUID) (*model.Profile, error) {
	var (
		p                  model.Profile
		roleID             *int16
		roleName, roleSlug *string
	)
	err := q.QueryRow(ctx, `
		SELECT p.id, p.education, p.current_job, p.hours_per_week, p.updated_at, r.id, r.name, r.slug
		FROM user_profiles p
		LEFT JOIN roles r ON r.id = p.target_role_id
		WHERE p.session_id = $1`, sessionID).
		Scan(&p.ID, &p.Education, &p.CurrentJob, &p.HoursPerWeek, &p.UpdatedAt, &roleID, &roleName, &roleSlug)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if roleID != nil {
		p.TargetRole = &model.Role{ID: *roleID, Name: *roleName, Slug: *roleSlug}
	}

	rows, err := q.Query(ctx, `
		SELECT s.id, s.name, s.slug, s.category, us.proficiency, us.source
		FROM user_skills us
		JOIN skills s ON s.id = us.skill_id
		WHERE us.profile_id = $1
		ORDER BY s.category, s.name`, p.ID)
	if err != nil {
		return nil, err
	}
	p.Skills, err = pgx.CollectRows(rows, pgx.RowToStructByPos[model.UserSkill])
	if err != nil {
		return nil, err
	}
	return &p, nil
}
