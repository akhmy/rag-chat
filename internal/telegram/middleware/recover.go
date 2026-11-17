package middleware

import (
	"context"
	"rag-chat/internal/telegram/errors"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func Recover(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, upd *models.Update) {
		defer func() {
			if rec := recover(); rec != nil {
				var chatID int64
				switch {
				case upd.Message != nil:
					chatID = upd.Message.Chat.ID
				case upd.CallbackQuery != nil:
					chatID = upd.CallbackQuery.From.ID
				default:
					return
				}

				errors.Send(ctx, b, chatID, errors.ErrUnknown)
			}
		}()

		next(ctx, b, upd)
	}
}
