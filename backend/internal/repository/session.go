package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ahmadhasan/ai-career-compass/backend/internal/model"
)

type SessionRepository struct {
	db *pgxpool.Pool
}

func NewSessionRepository(db *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) Create(ctx context.Context) (model.Session, error) {
	var s model.Session
	err := r.db.QueryRow(ctx, `
		INSERT INTO user_sessions DEFAULT VALUES
		RETURNING id, created_at, last_active_at`).
		Scan(&s.ID, &s.CreatedAt, &s.LastActiveAt)
	return s, err
}

// Touch mengecek sesi ada dan memperbarui last_active_at paling sering sekali per menit,
// agar setiap request tidak selalu menulis ke database.
func (r *SessionRepository) Touch(ctx context.Context, id uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		WITH touched AS (
			UPDATE user_sessions SET last_active_at = now()
			WHERE id = $1 AND last_active_at < now() - interval '1 minute'
		)
		SELECT EXISTS (SELECT 1 FROM user_sessions WHERE id = $1)`, id).Scan(&exists)
	return exists, err
}
