package errors

import (
	"context"

	"github.com/go-telegram/bot"

	"rag-chat/internal/appctx"
)

const (
	ErrUnknown             = "Something went wrong. Please try again later."
	ErrFileTooLarge        = "File too large."
	ErrFileTypeUnsupported = "File type unsupported."
)

func Send(ctx context.Context, b *bot.Bot, chatID int64, text string) {
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	})
	if err != nil {
		appctx.Logger(ctx).Warn("failed to send error message", "error", err)
	}
}

func AnswerCallbackQuery(ctx context.Context, b *bot.Bot, callbackQueryID string, text string) {
	_, err := b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackQueryID,
		Text:            text,
	})
	if err != nil {
		appctx.Logger(ctx).Warn("failed to answer callback query", "error", err)
	}
}
