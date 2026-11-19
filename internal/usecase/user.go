package usecase

import (
	"context"
	"uuid"

	"rag-chat/internal/domain"
)

type userStore interface {
	Create(ctx context.Context, u domain.User) (domain.User, error)
	GetByTelegramID(ctx context.Context, telegramID int64) (domain.User, error)
	SetActiveSession(ctx context.Context, userID int64, sessionID uuid.UUID) error
}

type UserService struct {
	repo userStore
}

func NewUserService(repo userStore) *UserService {
	return &UserService{repo: repo}
}

func (svc *UserService) CreateUser(ctx context.Context, telegramID *int64) (domain.User, error) {
	return svc.repo.Create(ctx, domain.NewUser(telegramID))
}

func (svc *UserService) GetUserByTelegramID(ctx context.Context, telegramID int64) (domain.User, error) {
	return svc.repo.GetByTelegramID(ctx, telegramID)
}

func (svc *UserService) SetActiveSession(ctx context.Context, userID int64, sessionID uuid.UUID) error {
	return svc.repo.SetActiveSession(ctx, userID, sessionID)
}
