package service

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func ptr[T any](v T) *T { return &v }

func validJobRequest() JobRequest {
	return JobRequest{
		Title:       "  AI Engineer  ",
		Company:     ptr("PT Contoh"),
		SourceName:  "Glints",
		SourceURL:   ptr("https://example.com/job/1"),
		PostedDate:  ptr("2026-09-20"),
		CollectedAt: ptr("2026-09-25"),
		WorkType:    ptr("hybrid"),
		Level:       ptr("mid"),
		RawText:     strings.Repeat("a", minRawTextChars),
	}
}

func TestJobValidateOK(t *testing.T) {
	s := &JobService{now: func() time.Time { return time.Date(2026, 9, 26, 15, 0, 0, 0, time.UTC) }}
	in, err := s.validate(validJobRequest())
	if err != nil {
		t.Fatal(err)
	}
	if in.Title != "AI Engineer" {
		t.Errorf("title tidak di-trim: %q", in.Title)
	}
	if in.CollectedAt.Format(dateLayout) != "2026-09-25" || in.PostedDate.Format(dateLayout) != "2026-09-20" {
		t.Errorf("tanggal = %v / %v", in.CollectedAt, in.PostedDate)
	}
}

func TestJobValidateDefaultsCollectedAtToToday(t *testing.T) {
	s := &JobService{now: func() time.Time { return time.Date(2026, 9, 26, 23, 30, 0, 0, time.UTC) }}
	req := validJobRequest()
	req.CollectedAt, req.PostedDate = nil, ptr("  ")
	in, err := s.validate(req)
	if err != nil {
		t.Fatal(err)
	}
	if in.CollectedAt.Format(dateLayout) != "2026-09-26" || in.PostedDate != nil {
		t.Errorf("collected_at = %v, posted_date = %v", in.CollectedAt, in.PostedDate)
	}
}

func TestJobValidateErrors(t *testing.T) {
	s := &JobService{now: func() time.Time { return time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC) }}
	cases := map[string]func(*JobRequest){
		"title":        func(r *JobRequest) { r.Title = "AI" },
		"source_name":  func(r *JobRequest) { r.SourceName = "" },
		"source_url":   func(r *JobRequest) { r.SourceURL = ptr("javascript:alert(1)") },
		"level":        func(r *JobRequest) { r.Level = ptr("principal") },
		"work_type":    func(r *JobRequest) { r.WorkType = ptr("wfh") },
		"raw_text":     func(r *JobRequest) { r.RawText = "pendek" },
		"collected_at": func(r *JobRequest) { r.CollectedAt = ptr("2026-09-27") },
		"posted_date":  func(r *JobRequest) { r.PostedDate = ptr("2026-09-26") },
		"company":      func(r *JobRequest) { r.Company = ptr(strings.Repeat("x", maxTextField+1)) },
	}
	for field, mutate := range cases {
		t.Run(field, func(t *testing.T) {
			req := validJobRequest()
			mutate(&req)
			_, err := s.validate(req)
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("err = %v, ingin ValidationError", err)
			}
			if _, ok := ve.Fields[field]; !ok || len(ve.Fields) != 1 {
				t.Errorf("fields = %v, ingin hanya %q", ve.Fields, field)
			}
		})
	}
}
