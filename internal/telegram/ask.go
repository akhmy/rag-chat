package telegram

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"rag-chat/internal/appctx"
	"rag-chat/internal/telegram/middleware"
	"rag-chat/internal/usecase"
)

func (h *mainHandler) handleAsk(ctx context.Context, b *bot.Bot, msg *models.Message, updateID int64) {
	logger := appctx.Logger(ctx)
	chatID := msg.Chat.ID
	draftID := strconv.FormatInt(updateID, 10)

	var answer strings.Builder
	var lastDraft time.Time
	for delta, err := range h.ask.Execute(ctx, &usecase.AskInput{
		UserID:     middleware.ContextUserID(ctx),
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
