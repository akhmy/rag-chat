package telegram

import (
	"context"
	"iter"
	"rag-chat/internal/appctx"
	"rag-chat/internal/usecase"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type AskUseCase interface {
	Execute(ctx context.Context, in *usecase.AskInput) iter.Seq2[string, error]
}

func (b *Bot) handleAsk(ctx context.Context, sdk *bot.Bot, upd *models.Update) {
	logger := appctx.Logger(ctx)
	user := appctx.User(ctx)
	if user == nil {
		logger.Error("user is nil")
		return
	}

	var answer strings.Builder
	var lastDraft time.Time
	for delta, err := range b.ask.Execute(ctx, &usecase.AskInput{
		UserID:     user.ID,
		SessionID:  nil,
		Query:      upd.Message.Text,
		TurnsLimit: 10,
		ChunkLimit: 10,
	}) {
		if err != nil {
			logger.Error("failed to execute ask", "error", err)
			if _, err := sdk.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: upd.Message.Chat.ID,
				Text:   "error",
			}); err != nil {
				logger.Error("failed to send message", "error", err)
			}
			return
		}

		answer.WriteString(delta)

		if time.Since(lastDraft) < time.Second {
			continue
		}

		if _, err = sdk.SendMessageDraft(ctx, &bot.SendMessageDraftParams{
			ChatID:  upd.Message.Chat.ID,
			DraftID: strconv.FormatInt(upd.ID, 10),
			Text:    answer.String(),
		}); err != nil {
			logger.Error("failed to send draft", "error", err)
		}

		lastDraft = time.Now()
	}

	formatted, err := MarkdownToHTML(answer.String())
	if err != nil {
		logger.Error("failed to format html", "error", err)
		return
	}

	if _, err := sdk.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    upd.Message.Chat.ID,
		Text:      formatted,
		ParseMode: models.ParseModeHTML,
	}); err != nil {
		logger.Error("failed to send message", "error", err)
	}
}
