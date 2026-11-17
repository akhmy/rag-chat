package telegram

import (
	"context"
	"fmt"
	"rag-chat/internal/appctx"
	"rag-chat/internal/domain"
	tgerrors "rag-chat/internal/telegram/errors"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

var (
	maxSessions = 4
)

type SessionService interface {
	CreateSession(ctx context.Context, userID int64, title string) (int64, error)
	ListSessions(ctx context.Context, userID int64, offset, limit int) ([]domain.Session, error)
	GetSession(ctx context.Context, sessionID int64) (domain.Session, error)
}

func (h *handler) handleSessions(ctx context.Context, b *bot.Bot, upd *models.Update) {
	h.handleSessionsList(ctx, b, upd, 0)
}

func (h *handler) handleSessionsList(ctx context.Context, b *bot.Bot, upd *models.Update, offset int) {
	logger := appctx.Logger(ctx)
	user := appctx.User(ctx)

	var (
		chatID    int64
		isMessage bool
	)

	if upd.Message != nil {
		chatID = upd.Message.Chat.ID
		isMessage = true
	} else if upd.CallbackQuery != nil && upd.CallbackQuery.Message.Message != nil {
		chatID = upd.CallbackQuery.Message.Message.Chat.ID
	} else {
		logger.Error("unexpected update type")
		return
	}

	sessions, err := h.sessionSvc.ListSessions(ctx, user.ID, offset, maxSessions+1)
	if err != nil {
		logger.Error("failed to list sessions", "error", err)
		if isMessage {
			tgerrors.Send(ctx, b, chatID, tgerrors.ErrUnknown)
		} else {
			tgerrors.AnswerCallbackQuery(ctx, b, upd.CallbackQuery.ID, "Failed")
		}
		return
	}
	keyboard, err := buildSessionKeyboard(sessions[:min(len(sessions), maxSessions)], offset, len(sessions) > maxSessions)
	if err != nil {
		logger.Error("failed to build session keyboard")
		if isMessage {
			tgerrors.Send(ctx, b, chatID, tgerrors.ErrUnknown)
		} else {
			tgerrors.AnswerCallbackQuery(ctx, b, upd.CallbackQuery.ID, "Failed")
		}
		return
	}

	if isMessage {
		if _, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        "Pick a session",
			ReplyMarkup: &models.InlineKeyboardMarkup{InlineKeyboard: keyboard},
		}); err != nil {
			logger.Error("failed to send message", "error", err)
			tgerrors.Send(ctx, b, chatID, tgerrors.ErrUnknown)
		}
	} else {
		if _, err := b.EditMessageReplyMarkup(ctx, &bot.EditMessageReplyMarkupParams{
			ChatID:      chatID,
			MessageID:   upd.CallbackQuery.Message.Message.ID,
			ReplyMarkup: &models.InlineKeyboardMarkup{InlineKeyboard: keyboard},
		}); err != nil {
			logger.Error("failed to edit message reply", "error", err)
			tgerrors.Send(ctx, b, chatID, tgerrors.ErrUnknown)
		}
	}
}

func (h *handler) handleSessionsNew(ctx context.Context, b *bot.Bot, upd *models.Update) {
	logger := appctx.Logger(ctx)
	user := appctx.User(ctx)

	if _, err := h.sessionSvc.CreateSession(ctx, user.ID, ""); err != nil {
		logger.Error("failed to create session", "error", err)
		if _, err := b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: upd.CallbackQuery.ID,
			Text:            "Failed to create session",
		}); err != nil {
			logger.Error("failed to answer new session", "error", err)
			tgerrors.Send(ctx, b, upd.Message.Chat.ID, tgerrors.ErrUnknown)
		}

		return
	}
	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    upd.CallbackQuery.Message.Message.Chat.ID,
		MessageID: upd.CallbackQuery.Message.Message.ID,
		Text:      "Created session",
	})
	b.EditMessageReplyMarkup(ctx, &bot.EditMessageReplyMarkupParams{
		ChatID:      upd.CallbackQuery.Message.Message.Chat.ID,
		MessageID:   upd.CallbackQuery.Message.Message.ID,
		ReplyMarkup: nil,
	})
}
func (h *handler) handleSessionsCallback(ctx context.Context, b *bot.Bot, upd *models.Update) {
	logger := appctx.Logger(ctx)

	cmd := strings.TrimPrefix(upd.CallbackQuery.Data, "sessions:")
	switch {
	case cmd == "new":
		h.handleSessionsNew(ctx, b, upd)
	case strings.HasPrefix(cmd, "list,"):
		offset, err := strconv.Atoi(strings.TrimPrefix(cmd, "list,"))
		if err != nil {
			logger.Error("failed to parse offset", "error", err)
			_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
				CallbackQueryID: upd.CallbackQuery.ID,
				Text:            "Failed to list sessions",
			})
			return
		}
		h.handleSessionsList(ctx, b, upd, offset)
	default:
		id, err := strconv.ParseInt(cmd, 10, 64)
		if err != nil {
			logger.Error("failed to parse session id", "error", err)
			tgerrors.AnswerCallbackQuery(ctx, b, upd.CallbackQuery.ID, tgerrors.ErrUnknown)
			return
		}
		h.handleSessionSelect(ctx, b, upd, id)
	}
}

func (h *handler) handleSessionSelect(ctx context.Context, b *bot.Bot, upd *models.Update, sessionID int64) {
	logger := appctx.Logger(ctx)
	user := appctx.User(ctx)

	if err := h.userSvc.SetActiveSession(ctx, user.ID, sessionID); err != nil {
		logger.Error("failed to set active session", "error", err)
		tgerrors.AnswerCallbackQuery(ctx, b, upd.CallbackQuery.ID, "")
		return
	}
	_, _ = b.EditMessageReplyMarkup(ctx, &bot.EditMessageReplyMarkupParams{
		ChatID:      upd.CallbackQuery.Message.Message.Chat.ID,
		MessageID:   upd.CallbackQuery.Message.Message.ID,
		ReplyMarkup: nil,
	})
}

func buildSessionKeyboard(
	sessions []domain.Session,
	offset int,
	renderOlder bool,
) ([][]models.InlineKeyboardButton, error) {
	keyboard := make([][]models.InlineKeyboardButton, 0, len(sessions))
	for _, session := range sessions {
		var text string
		if session.Title == "" {
			text = "Unnamed session " + session.CreatedAt.Format(time.RFC822)
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

	controls := make([]models.InlineKeyboardButton, 0, 1)

	if offset != 0 {
		controls = append(controls, models.InlineKeyboardButton{
			Text:         "Newer",
			CallbackData: fmt.Sprintf("sessions:list,%d", offset-maxSessions),
		})
	}

	controls = append(controls, models.InlineKeyboardButton{
		Text:         "New",
		CallbackData: "sessions:new",
	})

	if renderOlder {
		controls = append(controls, models.InlineKeyboardButton{
			Text:         "Older",
			CallbackData: fmt.Sprintf("sessions:list,%d", offset+maxSessions),
		})
	}

	return append(keyboard, controls), nil
}
