package usecase

import (
	"context"
	"rag-chat/internal/domain"
)

type UserRepository interface {
	Create(ctx context.Context, u domain.User) (domain.User, error)
}

type CreateUser struct {
	repo UserRepository
}

func NewCreateUser(repo UserRepository) *CreateUser {
	return &CreateUser{repo: repo}
}

func (uc *CreateUser) Execute(ctx context.Context, telegramID *int64) (domain.User, error) {
	return uc.repo.Create(ctx, domain.NewUser(telegramID))
}
