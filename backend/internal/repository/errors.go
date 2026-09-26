package repository

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

var ErrNotFound = errors.New("not found")

// DuplicateError dikembalikan saat insert melanggar unique constraint yang dikenali.
type DuplicateError struct {
	Constraint string
	ExistingID string
}

func (e *DuplicateError) Error() string { return "duplicate: " + e.Constraint }

func uniqueViolation(err error) (constraint string, ok bool) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return pgErr.ConstraintName, true
	}
	return "", false
}
