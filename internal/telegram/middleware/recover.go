package middleware

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	tgerrors "rag-chat/internal/telegram/reply"
)

func Recover(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, upd *models.Update) {
		defer func() {
			if rec := recover(); rec != nil {
				switch {
				case upd.CallbackQuery != nil:
					tgerrors.AnswerCallbackQuery(ctx, b, upd.CallbackQuery.ID, tgerrors.UnknownError)
				case upd.Message != nil:
					tgerrors.SendMessage(ctx, b, upd.Message.Chat.ID, tgerrors.UnknownError)
				}
			}
		}()

		next(ctx, b, upd)
	}
}
