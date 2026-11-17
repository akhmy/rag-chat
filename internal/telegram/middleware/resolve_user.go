package middleware

import (
	"context"
	"errors"
	"rag-chat/internal/appctx"
	"rag-chat/internal/domain"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type UserResolver interface {
	GetUserByTelegramID(ctx context.Context, telegramID int64) (domain.User, error)
	CreateUser(ctx context.Context, telegramID *int64) (domain.User, error)
}

var ErrUnsupportedUpdateType = errors.New("unsupported update type")

func resolveTelegramID(upd *models.Update) (int64, error) {
	switch {
	case upd.Message != nil:
		return upd.Message.Chat.ID, nil
	case upd.CallbackQuery != nil:
		return upd.CallbackQuery.From.ID, nil
	default:
		return 0, ErrUnsupportedUpdateType
	}
}

// ResolveUser middleware guarantees.
func ResolveUser(userSvc UserResolver) bot.Middleware {
	return func(next bot.HandlerFunc) bot.HandlerFunc {
		return func(ctx context.Context, b *bot.Bot, upd *models.Update) {
			id, err := resolveTelegramID(upd)
			if err != nil {
				panic(err)
			}

			user, err := userSvc.GetUserByTelegramID(ctx, id)
			if err != nil {
				if errors.Is(err, domain.ErrUserNotFound) {
					user, err = userSvc.CreateUser(ctx, &id)
					if err != nil {
						panic(err)
					}

					next(appctx.WithUser(ctx, user), b, upd)
					return
				}
				panic(err)
			}

			next(appctx.WithUser(ctx, user), b, upd)
		}
	}
}
