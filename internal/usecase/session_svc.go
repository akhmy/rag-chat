package usecase

import (
	"context"
)

type SessionRepository interface {
	Create(ctx context.Context, userID int64, title string) (int64, error)
}
type SessionService struct {
	repo SessionRepository
}

func NewSessionService(repo SessionRepository) *SessionService {
	return &SessionService{repo: repo}
}

func (s *SessionService) Create(ctx context.Context, userID int64, title string) (int64, error) {
	return s.repo.Create(ctx, userID, title)
}
