package appctx

import (
	"context"
	"rag-chat/internal/domain"
)

func WithUser(ctx context.Context, user *domain.User) context.Context {
	return context.WithValue(ctx, userKey, user)
}

func User(ctx context.Context) *domain.User {
	return ctx.Value(userKey).(*domain.User)
}
