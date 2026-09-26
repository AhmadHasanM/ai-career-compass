package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/ahmadhasan/ai-career-compass/backend/internal/model"
	"github.com/ahmadhasan/ai-career-compass/backend/internal/repository"
)

type SessionService struct {
	repo *repository.SessionRepository
}

func NewSessionService(repo *repository.SessionRepository) *SessionService {
	return &SessionService{repo: repo}
}

func (s *SessionService) Create(ctx context.Context) (model.Session, error) {
	return s.repo.Create(ctx)
}

// Touch mengembalikan false jika sesi tidak ada.
func (s *SessionService) Touch(ctx context.Context, id uuid.UUID) (bool, error) {
	return s.repo.Touch(ctx, id)
}
