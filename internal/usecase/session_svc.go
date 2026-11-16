package usecase

import (
	"context"
	"rag-chat/internal/domain"
)

type SessionRepository interface {
	SetTitle(ctx context.Context, userID int64, title string) error
	Create(ctx context.Context, userID int64, title string) (int64, error)
	List(ctx context.Context, userID int64) ([]domain.Session, error)
	Get(ctx context.Context, sessionID int64) (domain.Session, error)
}
type SessionService struct {
	repo SessionRepository
}

func NewSessionService(repo SessionRepository) *SessionService {
	return &SessionService{repo: repo}
}

func (s *SessionService) CreateSession(ctx context.Context, userID int64, title string) (int64, error) {
	return s.repo.Create(ctx, userID, title)
}

func (s *SessionService) ListSessions(ctx context.Context, userID int64) ([]domain.Session, error) {
	return s.repo.List(ctx, userID)
}

func (s *SessionService) GetSession(ctx context.Context, sessionID int64) (domain.Session, error) {
	return s.repo.Get(ctx, sessionID)
}

func (s *SessionService) SetSessionTitle(ctx context.Context, userID int64, title string) error {
	return s.repo.SetTitle(ctx, userID, title)
}
