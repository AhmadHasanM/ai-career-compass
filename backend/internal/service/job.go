package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/ahmadhasan/ai-career-compass/backend/internal/model"
	"github.com/ahmadhasan/ai-career-compass/backend/internal/repository"
)

const (
	minRawTextChars = 300 // sama dengan validator data/raw_jobs di ai-service
	maxRawTextChars = 100_000
	dateLayout      = "2006-01-02"
)

// JobRequest adalah body POST /api/admin/jobs; field-nya mengikuti front matter data/raw_jobs.
type JobRequest struct {
	Title       string  `json:"title"`
	Company     *string `json:"company"`
	Level       *string `json:"level"`
	Location    *string `json:"location"`
	WorkType    *string `json:"work_type"`
	SourceName  string  `json:"source_name"`
	SourceURL   *string `json:"source_url"`
	PostedDate  *string `json:"posted_date"`
	CollectedAt *string `json:"collected_at"`
	RawText     string  `json:"raw_text"`
}

// JobProcessor memicu ekstraksi + embedding lowongan di ai-service.
type JobProcessor interface {
	ProcessJob(ctx context.Context, jobID uuid.UUID) error
}

type JobService struct {
	jobs      *repository.JobRepository
	processor JobProcessor // nil = tidak memicu pemrosesan (misal saat test)
	log       *slog.Logger
	now       func() time.Time
}

func NewJobService(jobs *repository.JobRepository, processor JobProcessor, log *slog.Logger) *JobService {
	return &JobService{jobs: jobs, processor: processor, log: log, now: time.Now}
}

type CreateJobResult struct {
	model.JobPosting
	// ProcessingQueued false berarti ai-service tidak bisa dihubungi; lowongan tetap tersimpan
	// berstatus pending dan bisa diproses ulang nanti.
	ProcessingQueued bool `json:"processing_queued"`
}

// Create menyimpan lowongan berstatus pending lalu meminta ai-service memprosesnya.
func (s *JobService) Create(ctx context.Context, req JobRequest) (CreateJobResult, error) {
	in, err := s.validate(req)
	if err != nil {
		return CreateJobResult{}, err
	}
	job, err := s.jobs.Create(ctx, in)
	var dup *repository.DuplicateError
	if errors.As(err, &dup) {
		return CreateJobResult{}, &ConflictError{Message: "lowongan dengan source_url ini sudah ada", ExistingID: dup.ExistingID}
	}
	if err != nil {
		return CreateJobResult{}, err
	}

	res := CreateJobResult{JobPosting: job}
	if s.processor != nil {
		// Lowongan sudah tersimpan: pemicu tetap dikirim walau klien memutus request.
		if err := s.processor.ProcessJob(context.WithoutCancel(ctx), job.ID); err != nil {
			s.log.Warn("gagal memicu pemrosesan lowongan", "job_id", job.ID, "error", err)
		} else {
			res.ProcessingQueued = true
		}
	}
	return res, nil
}

func (s *JobService) validate(req JobRequest) (model.JobInput, error) {
	var v validator
	in := model.JobInput{
		Title:      strings.TrimSpace(req.Title),
		Company:    cleanOptional(req.Company),
		Level:      cleanOptional(req.Level),
		Location:   cleanOptional(req.Location),
		WorkType:   cleanOptional(req.WorkType),
		SourceName: strings.TrimSpace(req.SourceName),
		SourceURL:  cleanOptional(req.SourceURL),
		RawText:    strings.TrimSpace(req.RawText),
	}

	switch n := utf8.RuneCountInString(in.Title); {
	case n < 3:
		v.add("title", "minimal 3 karakter")
	case n > 300:
		v.add("title", "maksimal 300 karakter")
	}
	switch n := utf8.RuneCountInString(in.SourceName); {
	case n < 2:
		v.add("source_name", "minimal 2 karakter")
	case n > 100:
		v.add("source_name", "maksimal 100 karakter")
	}
	for field, val := range map[string]*string{"company": in.Company, "location": in.Location} {
		if tooLong(val, maxTextField) {
			v.add(field, fmt.Sprintf("maksimal %d karakter", maxTextField))
		}
	}
	if !oneOf(in.Level, model.JobLevels) {
		v.add("level", "harus salah satu dari: "+strings.Join(model.JobLevels, ", "))
	}
	if !oneOf(in.WorkType, model.JobWorkTypes) {
		v.add("work_type", "harus salah satu dari: "+strings.Join(model.JobWorkTypes, ", "))
	}
	if in.SourceURL != nil && !validHTTPURL(*in.SourceURL) {
		v.add("source_url", "harus URL http(s) yang valid")
	}
	switch n := utf8.RuneCountInString(in.RawText); {
	case n < minRawTextChars:
		v.add("raw_text", fmt.Sprintf("terlalu pendek (%d < %d karakter)", n, minRawTextChars))
	case n > maxRawTextChars:
		v.add("raw_text", fmt.Sprintf("maksimal %d karakter", maxRawTextChars))
	}

	today := truncateDay(s.now())
	in.CollectedAt = today
	if c := cleanOptional(req.CollectedAt); c != nil {
		d, err := time.Parse(dateLayout, *c)
		switch {
		case err != nil:
			v.add("collected_at", "format tanggal harus YYYY-MM-DD")
		case d.After(today):
			v.add("collected_at", "tidak boleh di masa depan")
		default:
			in.CollectedAt = d
		}
	}
	if p := cleanOptional(req.PostedDate); p != nil {
		d, err := time.Parse(dateLayout, *p)
		switch {
		case err != nil:
			v.add("posted_date", "format tanggal harus YYYY-MM-DD")
		case d.After(in.CollectedAt):
			v.add("posted_date", "tidak boleh setelah collected_at")
		default:
			in.PostedDate = &d
		}
	}

	return in, v.err()
}

func validHTTPURL(raw string) bool {
	if len(raw) > 2000 {
		return false
	}
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func truncateDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
