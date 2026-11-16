package usecase

import (
	"context"
	"rag-chat/internal/domain"
)

type UserRepository interface {
	Create(ctx context.Context, u domain.User) (domain.User, error)
	GetByTelegramID(ctx context.Context, telegramID int64) (domain.User, error)
	SetActiveSession(ctx context.Context, userID int64, sessionID int64) error
	GetActiveSession(ctx context.Context, userID int64) (*domain.Session, error)
}

type UserService struct {
	repo UserRepository
}

func NewUserService(repo UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (svc *UserService) CreateUser(ctx context.Context, telegramID *int64) (domain.User, error) {
	return svc.repo.Create(ctx, domain.NewUser(telegramID))
}

func (svc *UserService) GetUserByTelegramID(ctx context.Context, telegramID int64) (domain.User, error) {
	return svc.repo.GetByTelegramID(ctx, telegramID)
}

func (svc *UserService) SetActiveSession(ctx context.Context, userID int64, sessionID int64) error {
	return svc.repo.SetActiveSession(ctx, userID, sessionID)
}

func (svc *UserService) GetActiveSession(ctx context.Context, userID int64) (*domain.Session, error) {
	return svc.repo.GetActiveSession(ctx, userID)
}
