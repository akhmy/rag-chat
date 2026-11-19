package telegram

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"rag-chat/internal/appctx"
	"rag-chat/internal/extract"
	tgerrors "rag-chat/internal/telegram/errors"
	"rag-chat/internal/usecase"
)

type IngestUseCase interface {
	Execute(ctx context.Context, source usecase.DocumentSource, userID int64) error
}

func (h *handler) handleDocument(ctx context.Context, b *bot.Bot, msg *models.Message) {
	logger := appctx.Logger(ctx)
	doc := msg.Document
	chatID := msg.Chat.ID

	title := strings.TrimSpace(msg.Caption)

	if title == "" {
		title = doc.FileName
	}

	file, err := b.GetFile(ctx, &bot.GetFileParams{FileID: doc.FileID})
	if err != nil {
		logger.Error("failed to get file", "error", err)
		tgerrors.Send(ctx, b, chatID, tgerrors.ErrUnknown)
		return
	}

	path, err := h.downloader.DownloadTemp(ctx, b.FileDownloadLink(file))
	if err != nil {
		logger.Error("failed to download file", "error", err)
		if errors.Is(err, ErrFileTooLarge) {
			tgerrors.Send(ctx, b, chatID, tgerrors.ErrFileTooLarge)
		} else {
			tgerrors.Send(ctx, b, chatID, tgerrors.ErrUnknown)
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
			tgerrors.Send(ctx, b, chatID, tgerrors.ErrFileTypeUnsupported)
		} else {
			tgerrors.Send(ctx, b, chatID, tgerrors.ErrUnknown)
		}

		return
	}

	if err := h.ingest.Execute(ctx, source, appctx.User(ctx).ID); err != nil {
		logger.Error("failed to ingest file", "error", err)
		tgerrors.Send(ctx, b, chatID, tgerrors.ErrUnknown)
	}
}
