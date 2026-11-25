package telegram

import (
	"context"
	"rag-chat/internal/telegram/middleware"
	"rag-chat/internal/telegram/reply"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"rag-chat/internal/appctx"
	"rag-chat/internal/usecase"
)

const draftTick time.Duration = time.Second

func (h *mainHandler) handleAsk(ctx context.Context, b *bot.Bot, msg *models.Message, updID int64) {
	var (
		logger = appctx.Logger(ctx)
		userID = middleware.ContextUserID(ctx)

		chatID = msg.Chat.ID
		//draftID = strconv.FormatInt(updID, 10)

		answer    strings.Builder
		lastDraft time.Time
	)

	for delta, err := range h.ask.Execute(ctx, &usecase.AskInput{
		UserID:     userID,
		Query:      msg.Text,
		TurnLimit:  10,
		ChunkLimit: 10,
	}) {
		if err != nil {
			logger.Error("failed to execute ask", "error", err)
			reply.SendMessage(ctx, b, msg.Chat.ID, reply.UnknownError)
			return
		}

		answer.WriteString(delta)

		if time.Since(lastDraft) < draftTick {
			continue
		}

		if _, err := b.SendRichMessageDraft(ctx, &bot.SendRichMessageDraftParams{
			ChatID:  chatID,
			DraftID: int(updID),
			RichMessage: models.InputRichMessage{
				Markdown: answer.String(),
			},
		}); err != nil {
			logger.Error("failed to send draft", "error", err)
		}

		lastDraft = time.Now()
	}

	if _, err := b.SendRichMessage(ctx, &bot.SendRichMessageParams{
		ChatID: chatID,
		RichMessage: models.InputRichMessage{
			Markdown: answer.String(),
		},
	}); err != nil {
		logger.Error("failed to send message", "error", err)
		reply.SendMessage(ctx, b, msg.Chat.ID, reply.UnknownError)
		return
	}
}
