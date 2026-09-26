package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/ahmadhasan/ai-career-compass/backend/internal/model"
	"github.com/ahmadhasan/ai-career-compass/backend/internal/repository"
)

// ResourceEmbedder memicu embedding sumber belajar di ai-service agar bisa dicari chatbot.
type ResourceEmbedder interface {
	EmbedResource(ctx context.Context, resourceID uuid.UUID) error
}

// ResourceRequest adalah body POST /api/admin/resources. Skill boleh lewat id atau slug.
type ResourceRequest struct {
	SkillID   int      `json:"skill_id"`
	SkillSlug string   `json:"skill_slug"`
	Title     string   `json:"title"`
	URL       string   `json:"url"`
	Type      string   `json:"type"`
	Level     *string  `json:"level"`
	Language  string   `json:"language"`
	IsFree    *bool    `json:"is_free"`
	EstHours  *float64 `json:"est_hours"`
}

type ResourceService struct {
	resources *repository.ResourceRepository
	taxonomy  *repository.TaxonomyRepository
	embedder  ResourceEmbedder // nil = tidak memicu embedding
	log       *slog.Logger
}

func NewResourceService(resources *repository.ResourceRepository, taxonomy *repository.TaxonomyRepository,
	embedder ResourceEmbedder, log *slog.Logger) *ResourceService {
	return &ResourceService{resources: resources, taxonomy: taxonomy, embedder: embedder, log: log}
}

type CreateResourceResult struct {
	model.LearningResource
	EmbeddingQueued bool `json:"embedding_queued"`
}

func (s *ResourceService) Create(ctx context.Context, req ResourceRequest) (CreateResourceResult, error) {
	in, err := s.validate(ctx, req)
	if err != nil {
		return CreateResourceResult{}, err
	}
	res, err := s.resources.Create(ctx, in)
	var dup *repository.DuplicateError
	if errors.As(err, &dup) {
		return CreateResourceResult{}, &ConflictError{Message: "sumber belajar dengan URL ini sudah ada", ExistingID: dup.ExistingID}
	}
	if err != nil {
		return CreateResourceResult{}, err
	}
	out := CreateResourceResult{LearningResource: res}
	if s.embedder != nil {
		if err := s.embedder.EmbedResource(context.WithoutCancel(ctx), res.ID); err != nil {
			s.log.Warn("gagal memicu embedding sumber belajar", "resource_id", res.ID, "error", err)
		} else {
			out.EmbeddingQueued = true
		}
	}
	return out, nil
}

// ListBySkill: GET /api/resources?skill_id=
func (s *ResourceService) ListBySkill(ctx context.Context, skillID int) ([]model.LearningResource, error) {
	if skillID <= 0 {
		return nil, &ValidationError{Fields: map[string]string{"skill_id": "wajib diisi"}}
	}
	res, err := s.resources.BySkills(ctx, []int{skillID}, 0)
	if err != nil {
		return nil, err
	}
	if list := res[skillID]; list != nil {
		return list, nil
	}
	return []model.LearningResource{}, nil
}

func (s *ResourceService) validate(ctx context.Context, req ResourceRequest) (model.ResourceInput, error) {
	var v validator
	in := model.ResourceInput{
		SkillID:  req.SkillID,
		Title:    strings.TrimSpace(req.Title),
		URL:      strings.TrimSpace(req.URL),
		Type:     strings.TrimSpace(req.Type),
		Level:    cleanOptional(req.Level),
		Language: strings.TrimSpace(req.Language),
		IsFree:   req.IsFree == nil || *req.IsFree,
		EstHours: req.EstHours,
	}
	if in.Language == "" {
		in.Language = "en"
	}

	switch n := utf8.RuneCountInString(in.Title); {
	case n < 3:
		v.add("title", "minimal 3 karakter")
	case n > 300:
		v.add("title", "maksimal 300 karakter")
	}
	if !validHTTPURL(in.URL) {
		v.add("url", "harus URL http(s) yang valid")
	}
	if !oneOf(&in.Type, model.ResourceTypes) {
		v.add("type", "harus salah satu dari: "+strings.Join(model.ResourceTypes, ", "))
	}
	if !oneOf(in.Level, model.Proficiencies) {
		v.add("level", "harus salah satu dari: "+strings.Join(model.Proficiencies, ", "))
	}
	if !oneOf(&in.Language, model.ResourceLanguages) {
		v.add("language", "harus salah satu dari: "+strings.Join(model.ResourceLanguages, ", "))
	}
	if in.EstHours != nil && (*in.EstHours <= 0 || *in.EstHours > 1000) {
		v.add("est_hours", "harus antara 0 dan 1000")
	}

	if in.SkillID == 0 && strings.TrimSpace(req.SkillSlug) != "" {
		id, err := s.taxonomy.SkillIDBySlug(ctx, strings.TrimSpace(req.SkillSlug))
		if errors.Is(err, repository.ErrNotFound) {
			v.add("skill_slug", fmt.Sprintf("skill %q tidak ada di taxonomy", req.SkillSlug))
		} else if err != nil {
			return in, err
		}
		in.SkillID = id
	}
	if in.SkillID <= 0 {
		v.add("skill_id", "isi skill_id atau skill_slug")
	} else if missing, err := s.taxonomy.MissingSkillIDs(ctx, []int{in.SkillID}); err != nil {
		return in, err
	} else if len(missing) > 0 {
		v.add("skill_id", "skill tidak ditemukan")
	}
	return in, v.err()
}
