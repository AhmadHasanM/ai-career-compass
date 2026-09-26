package service

import (
	"context"
	"slices"
	"strings"

	"github.com/ahmadhasan/ai-career-compass/backend/internal/model"
	"github.com/ahmadhasan/ai-career-compass/backend/internal/repository"
)

type TaxonomyService struct {
	repo *repository.TaxonomyRepository
}

func NewTaxonomyService(repo *repository.TaxonomyRepository) *TaxonomyService {
	return &TaxonomyService{repo: repo}
}

func (s *TaxonomyService) ListRoles(ctx context.Context) ([]model.Role, error) {
	return s.repo.ListRoles(ctx)
}

func (s *TaxonomyService) ListSkills(ctx context.Context, category string) ([]model.Skill, error) {
	if category != "" && !slices.Contains(model.SkillCategories, category) {
		return nil, &ValidationError{Fields: map[string]string{
			"category": "harus salah satu dari: " + strings.Join(model.SkillCategories, ", "),
		}}
	}
	return s.repo.ListSkills(ctx, category)
}
