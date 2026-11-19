package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"rag-chat/internal/appctx"
	"rag-chat/internal/domain"
	tgerrors "rag-chat/internal/telegram/errors"
)

const (
	sessionPageSize       = 4
	sessionCallbackPrefix = "sessions:"
	sessionNewCallback    = sessionCallbackPrefix + "new"
	sessionPagePrefix     = sessionCallbackPrefix + "list,"
)

type SessionService interface {
	CreateSession(ctx context.Context, userID int64, title string) (int64, error)
	ListSessions(ctx context.Context, userID int64, offset, limit int) ([]domain.Session, error)
}

func (h *handler) handleSessions(ctx context.Context, b *bot.Bot, upd *models.Update) {
	chatID := upd.Message.Chat.ID
	keyboard, err := h.sessionKeyboard(ctx, 0)
	if err == nil {
		_, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        "Pick a session",
			ReplyMarkup: keyboard,
		})
	}
	if err != nil {
		appctx.Logger(ctx).Error("failed to show sessions", "error", err)
		tgerrors.Send(ctx, b, chatID, tgerrors.ErrUnknown)
	}
}

func (h *handler) handleSessionsCallback(ctx context.Context, b *bot.Bot, upd *models.Update) {
	query := upd.CallbackQuery
	msg := query.Message.Message
	if msg == nil {
		tgerrors.AnswerCallbackQuery(ctx, b, query.ID, "Open /sessions to try again.")
		return
	}

	var response string
	if err := h.applySessionCallback(ctx, b, query.Data, msg); err != nil {
		appctx.Logger(ctx).Error("failed to handle session callback", "error", err)
		response = tgerrors.ErrUnknown
	}
	tgerrors.AnswerCallbackQuery(ctx, b, query.ID, response)
}

func (h *handler) applySessionCallback(ctx context.Context, b *bot.Bot, data string, msg *models.Message) error {
	switch {
	case data == sessionNewCallback:
		if _, err := h.sessionSvc.CreateSession(ctx, appctx.User(ctx).ID, ""); err != nil {
			return fmt.Errorf("create session: %w", err)
		}

		_, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:      msg.Chat.ID,
			MessageID:   msg.ID,
			Text:        "Created session",
			ReplyMarkup: &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{}},
		})
		return err
	case strings.HasPrefix(data, sessionPagePrefix):
		offset, err := strconv.Atoi(strings.TrimPrefix(data, sessionPagePrefix))
		if err != nil || offset < 0 {
			return fmt.Errorf("invalid session page offset")
		}

		keyboard, err := h.sessionKeyboard(ctx, offset)
		if err != nil {
			return err
		}

		_, err = b.EditMessageReplyMarkup(ctx, &bot.EditMessageReplyMarkupParams{
			ChatID:      msg.Chat.ID,
			MessageID:   msg.ID,
			ReplyMarkup: keyboard,
		})
		return err
	default:
		id, err := strconv.ParseInt(strings.TrimPrefix(data, sessionCallbackPrefix), 10, 64)
		if err != nil {
			return fmt.Errorf("parse session ID: %w", err)
		}
		if err := h.userSvc.SetActiveSession(ctx, appctx.User(ctx).ID, id); err != nil {
			return fmt.Errorf("set active session: %w", err)
		}

		_, err = b.EditMessageReplyMarkup(ctx, &bot.EditMessageReplyMarkupParams{
			ChatID:      msg.Chat.ID,
			MessageID:   msg.ID,
			ReplyMarkup: &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{}},
		})
		return err
	}
}

func (h *handler) sessionKeyboard(ctx context.Context, offset int) (*models.InlineKeyboardMarkup, error) {
	sessions, err := h.sessionSvc.ListSessions(ctx, appctx.User(ctx).ID, offset, sessionPageSize+1)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	return buildSessionKeyboard(sessions, offset), nil
}

func buildSessionKeyboard(sessions []domain.Session, offset int) *models.InlineKeyboardMarkup {
	page := sessions[:min(len(sessions), sessionPageSize)]
	keyboard := make([][]models.InlineKeyboardButton, 0, len(page)+1)
	for _, session := range page {
		title := session.Title
		if title == "" {
			title = "Unnamed session " + session.CreatedAt.Format(time.RFC822)
		}
		keyboard = append(keyboard, []models.InlineKeyboardButton{{
			Text:         title,
			CallbackData: sessionCallbackPrefix + strconv.FormatInt(session.ID, 10),
		}})
	}

	controls := make([]models.InlineKeyboardButton, 0, 3)
	if offset > 0 {
		controls = append(controls, models.InlineKeyboardButton{
			Text:         "Newer",
			CallbackData: sessionPagePrefix + strconv.Itoa(max(0, offset-sessionPageSize)),
		})
	}
	controls = append(controls, models.InlineKeyboardButton{
		Text:         "New",
		CallbackData: sessionNewCallback,
	})
	if len(sessions) > sessionPageSize {
		controls = append(controls, models.InlineKeyboardButton{
			Text:         "Older",
			CallbackData: sessionPagePrefix + strconv.Itoa(offset+sessionPageSize),
		})
	}

	return &models.InlineKeyboardMarkup{InlineKeyboard: append(keyboard, controls)}
}
