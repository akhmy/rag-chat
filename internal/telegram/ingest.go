package telegram

import (
	"context"
	"errors"
	"os"
	"rag-chat/internal/appctx"
	"rag-chat/internal/extract"
	"rag-chat/internal/telegram/middleware"
	"rag-chat/internal/telegram/reply"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (h *mainHandler) handleIngest(ctx context.Context, b *bot.Bot, msg *models.Message) {
	var (
		logger = appctx.Logger(ctx)
		chatID = msg.Chat.ID
		doc    = msg.Document
	)

	title := strings.TrimSpace(msg.Caption)
	if title == "" {
		title = doc.FileName
	}

	file, err := b.GetFile(ctx, &bot.GetFileParams{FileID: doc.FileID})
	if err != nil {
		logger.Error("failed to get file", "error", err)
		reply.SendMessage(ctx, b, chatID, reply.UnknownError)
		return
	}

	path, err := h.downloader.DownloadTemp(ctx, b.FileDownloadLink(file))
	if err != nil {
		logger.Error("failed to download file", "error", err)
		if errors.Is(err, ErrFileTooLarge) {
			reply.SendMessage(ctx, b, chatID, reply.FileTooLarge)
		} else {
			reply.SendMessage(ctx, b, chatID, reply.UnknownError)
		}
		return
	}

	defer func() {
		if err := os.Remove(path); err != nil {
			logger.Warn("failed to remove temp file", "error", err)
		}
	}()

	source, err := extract.NewPDFSource(path, title)
	if err != nil {
		logger.Error("failed to open PDF", "error", err)
		if errors.Is(err, extract.ErrIsNotPDF) {
			reply.SendMessage(ctx, b, chatID, reply.FileTypeUnsupported)
		} else {
			reply.SendMessage(ctx, b, chatID, reply.UnknownError)
		}

		return
	}

	if err := h.ingest.Execute(ctx, source, middleware.ContextUserID(ctx)); err != nil {
		logger.Error("failed to ingest file", "error", err)
		reply.SendMessage(ctx, b, chatID, reply.UnknownError)
	}
}
