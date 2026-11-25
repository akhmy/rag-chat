package handler

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"rag-chat/internal/appctx"
	"rag-chat/internal/domain"
	"rag-chat/internal/telegram/middleware"
	"rag-chat/internal/telegram/reply"
)

const (
	DocumentsCommand = "/documents"

	DocumentsCallbackPrefix = "documents:"
	documentsListPrefix     = DocumentsCallbackPrefix + "list,"
	documentsGetPrefix      = DocumentsCallbackPrefix + "get,"
	documentsDeletePrefix   = DocumentsCallbackPrefix + "delete,"

	documentPageSize = 4
)

type DocumentService interface {
	GetDocument(ctx context.Context, userID int64, id int64) (domain.Document, error)
	ListDocuments(ctx context.Context, userID int64, offset, limit int) ([]domain.Document, error)
	DeleteDocument(ctx context.Context, id int64, userID int64) error
}

type DocumentHandler struct {
	svc DocumentService
}

func NewDocumentHandler(svc DocumentService) *DocumentHandler {
	return &DocumentHandler{svc: svc}
}

func (h *DocumentHandler) Handle(ctx context.Context, b *bot.Bot, upd *models.Update) {
	logger := appctx.Logger(ctx)
	chatID := upd.Message.Chat.ID

	documents, err := h.svc.ListDocuments(ctx, middleware.ContextUserID(ctx), 0, documentPageSize+1)
	if err != nil {
		logger.Error("failed to list documents", "error", err)
		reply.SendMessage(ctx, b, chatID, reply.UnknownError)
		return
	}

	params := &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "No documents found, create one by uploading a file",
	}
	if len(documents) > 0 {
		params.Text = "Pick the document you want to edit."
		params.ReplyMarkup = buildDocumentsKeyboard(documents, 0)
	}
	if _, err := b.SendMessage(ctx, params); err != nil {
		logger.Error("failed to send message", "error", err)
		reply.SendMessage(ctx, b, chatID, reply.UnknownError)
	}
}

func (h *DocumentHandler) HandleCallback(ctx context.Context, b *bot.Bot, upd *models.Update) {
	cq := upd.CallbackQuery
	if cq == nil {
		return
	}
	if cq.Message.Message == nil {
		reply.AnswerCallbackQuery(ctx, b, cq.ID, reply.UnknownError)
		return
	}

	var response string
	if err := h.applyCallback(ctx, b, cq); err != nil {
		appctx.Logger(ctx).Error("failed to apply document callback", "error", err)
		response = reply.UnknownError
		if errors.Is(err, domain.ErrDocumentNotFound) {
			response = reply.DocumentNotFound
		}
	}
	reply.AnswerCallbackQuery(ctx, b, cq.ID, response)
}

func (h *DocumentHandler) applyCallback(ctx context.Context, b *bot.Bot, cq *models.CallbackQuery) error {
	userID := middleware.ContextUserID(ctx)
	msg := cq.Message.Message

	var (
		text     string
		keyboard *models.InlineKeyboardMarkup
	)

	switch {
	case strings.HasPrefix(cq.Data, documentsListPrefix):
		offset, err := strconv.Atoi(strings.TrimPrefix(cq.Data, documentsListPrefix))
		if err != nil || offset < 0 {
			return fmt.Errorf("parse offset (%s): %w", cq.Data, err)
		}
		documents, err := h.svc.ListDocuments(ctx, userID, offset, documentPageSize+1)
		if err != nil {
			return fmt.Errorf("list documents: %w", err)
		}
		if len(documents) == 0 {
			text = "No documents found, create one by uploading a file."
		} else {
			text = "Pick the document you want to edit."
		}

		keyboard = buildDocumentsKeyboard(documents, offset)

	case strings.HasPrefix(cq.Data, documentsGetPrefix):
		id, err := strconv.ParseInt(strings.TrimPrefix(cq.Data, documentsGetPrefix), 10, 64)
		if err != nil {
			return fmt.Errorf("parse document ID (%s): %w", cq.Data, err)
		}
		document, err := h.svc.GetDocument(ctx, userID, id)
		if err != nil {
			return fmt.Errorf("get document: %w", err)
		}

		text = "Document \"" + document.Title + "\" picked."
		keyboard = buildDocumentKeyboard(document)

	case strings.HasPrefix(cq.Data, documentsDeletePrefix):
		id, err := strconv.ParseInt(strings.TrimPrefix(cq.Data, documentsDeletePrefix), 10, 64)
		if err != nil {
			return fmt.Errorf("parse document ID (%s): %w", cq.Data, err)
		}
		if err := h.svc.DeleteDocument(ctx, id, userID); err != nil {
			return fmt.Errorf("delete document: %w", err)
		}

		text = "Document deleted successfully."
		keyboard = &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{Text: "<", CallbackData: documentsListPrefix + "0"},
				},
			},
		}

	default:
		return fmt.Errorf("invalid document callback %q", cq.Data)
	}

	if _, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      msg.Chat.ID,
		MessageID:   msg.ID,
		Text:        text,
		ReplyMarkup: keyboard,
	}); err != nil {
		return fmt.Errorf("edit document keyboard: %w", err)
	}
	return nil
}

func buildDocumentKeyboard(document domain.Document) *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "Delete", CallbackData: documentsDeletePrefix + strconv.FormatInt(document.ID, 10)},
			},
			{
				{Text: "<", CallbackData: documentsListPrefix + "0"},
			},
		},
	}
}

func buildDocumentsKeyboard(documents []domain.Document, offset int) *models.InlineKeyboardMarkup {
	page := documents[:min(len(documents), documentPageSize)]
	keyboard := make([][]models.InlineKeyboardButton, 0, len(page)+1)
	for _, document := range page {
		keyboard = append(keyboard, []models.InlineKeyboardButton{{
			Text:         document.Title,
			CallbackData: documentsGetPrefix + strconv.FormatInt(document.ID, 10),
		}})
	}

	controls := make([]models.InlineKeyboardButton, 0, 3)
	if offset > 0 {
		controls = append(controls, models.InlineKeyboardButton{
			Text:         "Newer",
			CallbackData: documentsListPrefix + strconv.Itoa(max(0, offset-documentPageSize)),
		})
	}
	if len(documents) > documentPageSize {
		controls = append(controls, models.InlineKeyboardButton{
			Text:         "Older",
			CallbackData: documentsListPrefix + strconv.Itoa(offset+documentPageSize),
		})
	}

	return &models.InlineKeyboardMarkup{InlineKeyboard: append(keyboard, controls)}
}
