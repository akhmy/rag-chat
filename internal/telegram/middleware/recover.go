package middleware

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	tgerrors "rag-chat/internal/telegram/errors"
)

func Recover(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, upd *models.Update) {
		defer func() {
			if rec := recover(); rec != nil {
				switch {
				case upd.CallbackQuery != nil:
					tgerrors.AnswerCallbackQuery(ctx, b, upd.CallbackQuery.ID, tgerrors.ErrUnknown)
				case upd.Message != nil:
					tgerrors.Send(ctx, b, upd.Message.Chat.ID, tgerrors.ErrUnknown)
				}
			}
		}()

		next(ctx, b, upd)
	}
}
