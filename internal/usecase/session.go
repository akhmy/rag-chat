package usecase

import (
	"context"
	"uuid"

	"rag-chat/internal/domain"
)

type sessionStore interface {
	SetTitle(ctx context.Context, sessionID uuid.UUID, userID int64, title string) error
	Create(ctx context.Context, sessionID uuid.UUID, userID int64, title string) (uuid.UUID, error)
	List(ctx context.Context, userID int64, offset, limit int) ([]domain.Session, error)
	Get(ctx context.Context, sessionID uuid.UUID, userID int64) (domain.Session, error)
	Delete(ctx context.Context, sessionID uuid.UUID, userID int64) error
	Rename(ctx context.Context, sessionID uuid.UUID, newTitle string, userID int64) error
}
type SessionService struct {
	repo sessionStore
}

func NewSessionService(repo sessionStore) *SessionService {
	return &SessionService{repo: repo}
}

func (s *SessionService) CreateSession(ctx context.Context, userID int64, title string) (uuid.UUID, error) {
	return s.repo.Create(ctx, uuid.NewV4(), userID, title)
}

func (s *SessionService) ListSessions(ctx context.Context, userID int64, offset, limit int) ([]domain.Session, error) {
	return s.repo.List(ctx, userID, offset, limit)
}

func (s *SessionService) GetSession(ctx context.Context, sessionID uuid.UUID, userID int64) (domain.Session, error) {
	return s.repo.Get(ctx, sessionID, userID)
}

func (s *SessionService) SetSessionTitle(ctx context.Context, sessionID uuid.UUID, userID int64, title string) error {
	return s.repo.SetTitle(ctx, sessionID, userID, title)
}

func (s *SessionService) DeleteSession(ctx context.Context, sessionID uuid.UUID, userID int64) error {
	return s.repo.Delete(ctx, sessionID, userID)
}

func (s *SessionService) RenameSession(ctx context.Context, sessionID uuid.UUID, newTitle string, userID int64) error {
	return s.repo.Rename(ctx, sessionID, newTitle, userID)
}
