package errors

import (
	"context"

	"github.com/go-telegram/bot"
)

var (
	ErrUnknown             = "Something went wrong. Please try again later."
	ErrFileTooLarge        = "File too large."
	ErrFileTypeUnsupported = "File type unsupported."
)

func Send(ctx context.Context, b *bot.Bot, chatID int64, text string) {
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	})
}

func AnswerCallbackQuery(ctx context.Context, b *bot.Bot, callbackQueryID string, text string) {
	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackQueryID,
		Text:            text,
	})
}
