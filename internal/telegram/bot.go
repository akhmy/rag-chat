package telegram

import (
	"context"
	"errors"
	"log/slog"
	"rag-chat/internal/appctx"
	"rag-chat/internal/domain"
	"rag-chat/internal/extract"
	tgerrors "rag-chat/internal/telegram/errors"
	"rag-chat/internal/telegram/middleware"
	"rag-chat/internal/usecase"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type UserService interface {
	CreateUser(ctx context.Context, telegramID *int64) (domain.User, error)
	GetUserByTelegramID(ctx context.Context, telegramID int64) (domain.User, error)
	SetActiveSession(ctx context.Context, userID int64, sessionID int64) error
}

type IngestUseCase interface {
	Execute(ctx context.Context, source usecase.Source, userID int64) error
}

type handler struct {
	ask        AskUseCase
	userSvc    UserService
	sessionSvc SessionService
	downloader *DocumentDownloader
	ingest     IngestUseCase
}

func NewBot(
	token string,
	webhookSecret string,
	downloader *DocumentDownloader,
	askUC AskUseCase,
	userSvc UserService,
	sessionSvc SessionService,
	ingestUC IngestUseCase,
	logger *slog.Logger,
) (*bot.Bot, error) {
	h := &handler{
		ask:        askUC,
		userSvc:    userSvc,
		sessionSvc: sessionSvc,
		downloader: downloader,
		ingest:     ingestUC,
	}

	b, err := bot.New(
		token,
		bot.WithWebhookSecretToken(webhookSecret),
		bot.WithDefaultHandler(h.handleDefault),
		bot.WithMiddlewares(middleware.Recover, middleware.Logger(logger), middleware.ResolveUser(userSvc)),
	)
	if err != nil {
		return nil, err
	}

	//b.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeExact, b.start)
	b.RegisterHandler(bot.HandlerTypeMessageText, "/sessions", bot.MatchTypeExact, h.handleSessions)
	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, "sessions:", bot.MatchTypePrefix, h.handleSessionsCallback)

	return b, nil
}

func (h *handler) handleDefault(ctx context.Context, b *bot.Bot, upd *models.Update) {
	if upd.Message.Document != nil {
		h.handleDocument(ctx, b, upd)
		return
	}

	h.handleAsk(ctx, b, upd)
}

func (h *handler) handleDocument(ctx context.Context, b *bot.Bot, upd *models.Update) {
	logger := appctx.Logger(ctx)
	user := appctx.User(ctx)

	doc := upd.Message.Document
	title := strings.TrimSpace(upd.Message.Caption)

	if title == "" {
		title = doc.FileName
	}

	file, err := b.GetFile(ctx, &bot.GetFileParams{FileID: doc.FileID})
	if err != nil {
		logger.Error("failed to get file", "error", err)
		tgerrors.Send(ctx, b, upd.Message.Chat.ID, tgerrors.ErrUnknown)
		return
	}

	path, err := h.downloader.DownloadTemp(ctx, b.FileDownloadLink(file))
	if err != nil {
		logger.Error("failed to download file", "error", err)
		if errors.Is(err, ErrFileTooLarge) {
			tgerrors.Send(ctx, b, upd.Message.Chat.ID, tgerrors.ErrFileTooLarge)
		} else {
			tgerrors.Send(ctx, b, upd.Message.Chat.ID, tgerrors.ErrUnknown)
		}
		return
	}

	source, err := extract.NewPDFSource(path, title)
	if err != nil {
		logger.Error("failed to extract pdf", "error", err)

		if errors.Is(err, extract.ErrIsNotPDF) {
			tgerrors.Send(ctx, b, upd.Message.Chat.ID, tgerrors.ErrFileTypeUnsupported)
		} else {
			tgerrors.Send(ctx, b, upd.Message.Chat.ID, tgerrors.ErrUnknown)
		}

		return
	}

	if err := h.ingest.Execute(ctx, source, user.ID); err != nil {
		logger.Error("failed to ingest file", "error", err)
		tgerrors.Send(ctx, b, upd.Message.Chat.ID, tgerrors.ErrUnknown)
		return
	}
}
