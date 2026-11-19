package telegram

import (
	"context"
	"iter"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"rag-chat/internal/appctx"
	"rag-chat/internal/usecase"
)

type AskUseCase interface {
	Execute(ctx context.Context, in *usecase.AskInput) iter.Seq2[string, error]
}

func (h *handler) handleAsk(ctx context.Context, b *bot.Bot, updateID int64, msg *models.Message) {
	logger := appctx.Logger(ctx)
	chatID := msg.Chat.ID
	draftID := strconv.FormatInt(updateID, 10)

	var answer strings.Builder
	var lastDraft time.Time
	for delta, err := range h.ask.Execute(ctx, &usecase.AskInput{
		UserID:     appctx.User(ctx).ID,
		Query:      msg.Text,
		TurnsLimit: 10,
		ChunkLimit: 10,
	}) {
		if err != nil {
			logger.Error("failed to execute ask", "error", err)
			if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
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

		if _, err := b.SendMessageDraft(ctx, &bot.SendMessageDraftParams{
			ChatID:  chatID,
			DraftID: draftID,
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

	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      formatted,
		ParseMode: models.ParseModeHTML,
	}); err != nil {
		logger.Error("failed to send message", "error", err)
	}
}
