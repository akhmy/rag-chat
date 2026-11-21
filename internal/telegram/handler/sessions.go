package handler

import (
	"context"
	"errors"
	"fmt"
	"rag-chat/internal/telegram/cache"
	"strconv"
	"strings"
	"time"
	"uuid"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"rag-chat/internal/appctx"
	"rag-chat/internal/domain"
	"rag-chat/internal/telegram/middleware"
	"rag-chat/internal/telegram/reply"
)

const (
	SessionsCommand = "/sessions"

	SessionsCallbackPrefix = "sessions:"
	sessionsListPrefix     = SessionsCallbackPrefix + "list,"
	sessionsNewPrefix      = SessionsCallbackPrefix + "new"
	sessionsGetPrefix      = SessionsCallbackPrefix + "get,"
	sessionsRenamePrefix   = SessionsCallbackPrefix + "rename,"
	sessionsDeletePrefix   = SessionsCallbackPrefix + "delete,"

	sessionsPageSize = 4
)

type SessionService interface {
	CreateSession(ctx context.Context, userID int64, title string) (uuid.UUID, error)
	GetSession(ctx context.Context, sessionID uuid.UUID, userID int64) (domain.Session, error)
	ListSessions(ctx context.Context, userID int64, offset, limit int) ([]domain.Session, error)
	DeleteSession(ctx context.Context, sessionID uuid.UUID, userID int64) error
	RenameSession(ctx context.Context, sessionID uuid.UUID, newTitle string, userID int64) error
}

type SessionHandler struct {
	svc   SessionService
	cache *cache.InMemoryRenameCache
}

func NewSessionHandler(svc SessionService, cache *cache.InMemoryRenameCache) *SessionHandler {
	return &SessionHandler{
		svc:   svc,
		cache: cache,
	}
}

func (h *SessionHandler) Handle(ctx context.Context, b *bot.Bot, upd *models.Update) {
	var (
		logger = appctx.Logger(ctx)
		chatID = upd.Message.Chat.ID
	)

	sessions, err := h.svc.ListSessions(ctx, middleware.ContextUserID(ctx), 0, sessionsPageSize+1)
	if err != nil {
		logger.Error("failed to list sessions", "error", err)
		reply.SendMessage(ctx, b, chatID, reply.UnknownError)
		return
	}

	keyboard := buildSessionsKeyboard(sessions, 0)

	if len(sessions) == 0 {
		if _, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        "No sessions found",
			ReplyMarkup: keyboard,
		}); err != nil {
			logger.Error("failed to send message to user", "error", err)
			reply.SendMessage(ctx, b, chatID, reply.UnknownError)
		}

		return
	}

	if _, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        "Pick a session",
		ReplyMarkup: keyboard,
	}); err != nil {
		appctx.Logger(ctx).Error("failed to show sessions", "error", err)
		reply.SendMessage(ctx, b, chatID, reply.UnknownError)
		return
	}
}

func (h *SessionHandler) HandleCallback(ctx context.Context, b *bot.Bot, upd *models.Update) {
	logger := appctx.Logger(ctx)

	if upd.CallbackQuery.Message.Message == nil {
		logger.Error("invalid callback query", "cq", upd.CallbackQuery)
		reply.AnswerCallbackQuery(ctx, b, upd.CallbackQuery.ID, reply.UnknownError)

		return
	}

	if err := h.applyCallback(ctx, b, upd.CallbackQuery); err != nil {
		logger.Error("failed to apply callback", "error", err)
		reply.AnswerCallbackQuery(ctx, b, upd.CallbackQuery.ID, reply.UnknownError)

		return
	}

	if _, err := b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: upd.CallbackQuery.ID,
	}); err != nil {
		logger.Error("failed to answer callback", "error", err)
		reply.AnswerCallbackQuery(ctx, b, upd.CallbackQuery.ID, reply.UnknownError)
	}
}

func (h *SessionHandler) applyCallback(ctx context.Context, b *bot.Bot, cq *models.CallbackQuery) error {
	userID := middleware.ContextUserID(ctx)

	data := cq.Data

	switch {
	case strings.HasPrefix(data, sessionsListPrefix):
		offsetString, ok := strings.CutPrefix(data, sessionsListPrefix)
		if !ok {
			return errors.New("invalid callback query")
		}

		offset, err := strconv.Atoi(offsetString)
		if err != nil {
			return errors.New("invalid callback query")
		}

		sessions, err := h.svc.ListSessions(ctx, userID, offset, sessionsPageSize+1)
		if err != nil {
			return fmt.Errorf("list sessions: %w", err)
		}

		if _, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:      cq.From.ID,
			MessageID:   cq.Message.Message.ID,
			Text:        "Pick a session",
			ReplyMarkup: buildSessionsKeyboard(sessions, offset),
		}); err != nil {
			return fmt.Errorf("edit message: %w", err)
		}

	case strings.HasPrefix(data, sessionsNewPrefix):
		_, err := h.svc.CreateSession(ctx, userID, "")
		if err != nil {
			return fmt.Errorf("create session: %w", err)
		}

		if _, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    cq.From.ID,
			MessageID: cq.Message.Message.ID,
			Text:      "New session have been created, try uploading a document.",
		}); err != nil {
			return fmt.Errorf("edit message: %w", err)
		}

	case strings.HasPrefix(data, sessionsGetPrefix):
		id, err := uuid.Parse(strings.TrimPrefix(data, sessionsGetPrefix))
		if err != nil {
			return fmt.Errorf("parse session id (%s): %w", data, err)
		}

		session, err := h.svc.GetSession(ctx, id, userID)
		if err != nil {
			return fmt.Errorf("get session: %w", err)
		}

		if _, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:      cq.From.ID,
			MessageID:   cq.Message.Message.ID,
			Text:        "Session \"" + getSessionTitle(session) + "\" picked.",
			ReplyMarkup: buildSessionKeyboard(session.ID, sessionsListPrefix+"0"),
		}); err != nil {
			return fmt.Errorf("edit message: %w", err)
		}

	case strings.HasPrefix(data, sessionsDeletePrefix):
		id, err := uuid.Parse(strings.TrimPrefix(data, sessionsDeletePrefix))
		if err != nil {
			return fmt.Errorf("parse session id (%s): %w", data, err)
		}

		if err := h.svc.DeleteSession(ctx, id, userID); err != nil {
			return fmt.Errorf("delete session: %w", err)
		}

		if _, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    cq.From.ID,
			MessageID: cq.Message.Message.ID,
			Text:      "Session deleted.",
			ReplyMarkup: &models.InlineKeyboardMarkup{
				InlineKeyboard: [][]models.InlineKeyboardButton{
					{{Text: "<", CallbackData: sessionsListPrefix + "0"}},
				},
			},
		}); err != nil {
			return fmt.Errorf("edit message: %w", err)
		}
	case strings.HasPrefix(data, sessionsRenamePrefix):
		id, err := uuid.Parse(strings.TrimPrefix(data, sessionsRenamePrefix))
		if err != nil {
			return fmt.Errorf("parse session id (%s): %w", data, err)
		}

		prompt, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: cq.From.ID,
			Text:   "Type the new name",
			ReplyMarkup: &models.ForceReply{
				ForceReply:            true,
				InputFieldPlaceholder: "New name",
			},
		})

		h.cache.Put(cq.From.ID, userID, prompt.ID, id)
	}

	return nil
}

func getSessionTitle(session domain.Session) string {
	if session.Title != "" {
		return session.Title
	}
	return "Unnamed session (" + session.CreatedAt.Format(time.RFC3339) + ")"
}

func buildSessionKeyboard(id uuid.UUID, back string) *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "Rename", CallbackData: sessionsRenamePrefix + id.String()},
			},
			{
				{Text: "Delete", CallbackData: sessionsDeletePrefix + id.String()},
			},
			{
				{Text: "<", CallbackData: back},
			},
		},
	}
}

func buildSessionsKeyboard(sessions []domain.Session, offset int) *models.InlineKeyboardMarkup {
	page := sessions[:min(len(sessions), sessionsPageSize)]
	keyboard := make([][]models.InlineKeyboardButton, 0, len(page)+1)
	for _, session := range page {
		title := session.Title
		if title == "" {
			title = "Unnamed session (" + session.CreatedAt.Format(time.RFC3339) + ")"
		}
		keyboard = append(keyboard, []models.InlineKeyboardButton{{
			Text:         title,
			CallbackData: sessionsGetPrefix + session.ID.String(),
		}})
	}

	controls := make([]models.InlineKeyboardButton, 0, 3)
	if offset > 0 {
		controls = append(controls, models.InlineKeyboardButton{
			Text:         "<",
			CallbackData: sessionsListPrefix + strconv.Itoa(max(0, offset-sessionsPageSize)),
		})
	}
	controls = append(controls, models.InlineKeyboardButton{
		Text:         "+",
		CallbackData: sessionsNewPrefix,
	})
	if len(sessions) > sessionsPageSize {
		controls = append(controls, models.InlineKeyboardButton{
			Text:         ">",
			CallbackData: sessionsListPrefix + strconv.Itoa(offset+sessionsPageSize),
		})
	}

	return &models.InlineKeyboardMarkup{InlineKeyboard: append(keyboard, controls)}
}
