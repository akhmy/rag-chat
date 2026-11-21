package middleware

import (
	"context"
	"errors"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"rag-chat/internal/domain"
)

type userIDKey struct{}

// ContextWithUserID attaches the resolved userID to ctx.
func ContextWithUserID(ctx context.Context, user int64) context.Context {
	return context.WithValue(ctx, userIDKey{}, user)
}

// ContextUserID returns the userID attached by ResolveUser.
func ContextUserID(ctx context.Context) int64 {
	return ctx.Value(userIDKey{}).(int64)
}

type UserResolver interface {
	GetUserByTelegramID(ctx context.Context, telegramID int64) (domain.User, error)
	CreateUser(ctx context.Context, telegramID *int64) (domain.User, error)
}

func resolveTelegramID(upd *models.Update) (int64, bool) {
	switch {
	case upd.Message != nil && upd.Message.From != nil:
		return upd.Message.From.ID, true
	case upd.CallbackQuery != nil:
		return upd.CallbackQuery.From.ID, true
	default:
		return 0, false
	}
}

// ResolveUser attaches the user associated with a message or callback.
func ResolveUser(userSvc UserResolver) bot.Middleware {
	return func(next bot.HandlerFunc) bot.HandlerFunc {
		return func(ctx context.Context, b *bot.Bot, upd *models.Update) {
			id, ok := resolveTelegramID(upd)
			if !ok {
				return
			}

			user, err := userSvc.GetUserByTelegramID(ctx, id)
			if errors.Is(err, domain.ErrUserNotFound) {
				user, err = userSvc.CreateUser(ctx, &id)
			}
			if err != nil {
				panic(err)
			}

			next(ContextWithUserID(ctx, user.ID), b, upd)
		}
	}
}
