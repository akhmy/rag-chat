package telegram

import (
	"context"
	"fmt"
	"rag-chat/internal/appctx"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (b *Bot) handleSessions(ctx context.Context, sdk *bot.Bot, upd *models.Update) {
	logger := appctx.Logger(ctx)
	user := appctx.User(ctx)
	if user == nil {
		logger.Error("user is nil")
		return
	}

	sessions, err := b.sessionSvc.ListSessions(ctx, user.ID)
	if err != nil {
		logger.Error("failed to list sessions", "error", err)
		return
	}

	keyboard := make([][]models.InlineKeyboardButton, 0, len(sessions)+1)
	for _, session := range sessions {
		var text string
		if session.Title == "" {
			text = "unnamed session " + strconv.FormatInt(session.ID, 10)
		} else {
			text = session.Title
		}

		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{
				Text:         text,
				CallbackData: "sessions:" + strconv.FormatInt(session.ID, 10),
			},
		})
	}

	keyboard = append(keyboard, []models.InlineKeyboardButton{
		{
			Text:         "New session",
			CallbackData: "sessions:new",
		},
	})

	_, err = sdk.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      upd.Message.Chat.ID,
		Text:        fmt.Sprintf("%v", sessions),
		ReplyMarkup: models.InlineKeyboardMarkup{InlineKeyboard: keyboard},
	})
}

func (b *Bot) handleSessionsCallback(ctx context.Context, sdk *bot.Bot, upd *models.Update) {
	logger := appctx.Logger(ctx)
	user := appctx.User(ctx)

	q := upd.CallbackQuery

	if q.Message.Message != nil {
		msg := q.Message.Message

		if _, err := sdk.EditMessageReplyMarkup(ctx, &bot.EditMessageReplyMarkupParams{
			ChatID:    msg.Chat.ID,
			MessageID: msg.ID,
			ReplyMarkup: models.InlineKeyboardMarkup{
				InlineKeyboard: [][]models.InlineKeyboardButton{},
			},
		}); err != nil {
			logger.Error("failed to remove reply markup", "error", err)
		}
	}

	if user == nil {
		logger.Error("user is nil")
		return
	}

	value := strings.TrimPrefix(upd.CallbackQuery.Data, sessionsPrefix)

	switch {
	case value == "new":
		if _, err := b.sessionSvc.CreateSession(ctx, user.ID, ""); err != nil {
			logger.Error("failed to create session", "error", err)
		}
		if _, err := sdk.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: upd.CallbackQuery.ID,
			Text:            fmt.Sprintf("New session: %s", value),
		}); err != nil {
			replyError(ctx, sdk, upd.CallbackQuery.From.ID, "failed to create new session")
		}
	default:
		sessionID, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			logger.Error("failed to parse id", "error", err)
			replyError(ctx, sdk, upd.CallbackQuery.From.ID, "failed to parse session id")
			return
		}
		if err := b.userSvc.SetActiveSession(ctx, user.ID, sessionID); err != nil {
			logger.Error("failed to set active session", "error", err)
			replyError(ctx, sdk, upd.CallbackQuery.From.ID, "failed to set active session")
			return
		}
	}
}
