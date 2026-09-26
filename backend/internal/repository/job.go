package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ahmadhasan/ai-career-compass/backend/internal/model"
)

type JobRepository struct {
	db *pgxpool.Pool
}

func NewJobRepository(db *pgxpool.Pool) *JobRepository {
	return &JobRepository{db: db}
}

// Create menyimpan lowongan berstatus pending. Jika source_url sudah ada,
// mengembalikan *DuplicateError berisi id lowongan yang lama.
func (r *JobRepository) Create(ctx context.Context, in model.JobInput) (model.JobPosting, error) {
	var j model.JobPosting
	err := r.db.QueryRow(ctx, `
		INSERT INTO job_postings
			(title, company, level, location, work_type, source_name, source_url, posted_date, collected_at, raw_text)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, title, company, source_name, source_url, extraction_status, created_at`,
		in.Title, in.Company, in.Level, in.Location, in.WorkType, in.SourceName, in.SourceURL,
		in.PostedDate, in.CollectedAt, in.RawText).
		Scan(&j.ID, &j.Title, &j.Company, &j.SourceName, &j.SourceURL, &j.ExtractionStatus, &j.CreatedAt)

	if constraint, ok := uniqueViolation(err); ok && constraint == "uq_job_postings_source_url" {
		dup := &DuplicateError{Constraint: constraint}
		if lookupErr := r.db.QueryRow(ctx,
			`SELECT id::text FROM job_postings WHERE source_url = $1`, in.SourceURL).Scan(&dup.ExistingID); lookupErr != nil {
			return j, lookupErr
		}
		return j, dup
	}
	return j, err
}
