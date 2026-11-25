package telegram

import (
	"context"
	"iter"
	"log/slog"
	"rag-chat/internal/appctx"
	"rag-chat/internal/telegram/cache"
	"rag-chat/internal/telegram/handler"
	"rag-chat/internal/telegram/reply"
	"rag-chat/internal/usecase"
	"strings"
	"uuid"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"rag-chat/internal/telegram/middleware"
)

type AskUseCase interface {
	Execute(ctx context.Context, in *usecase.AskInput) iter.Seq2[string, error]
}

type IngestUseCase interface {
	Execute(ctx context.Context, source usecase.DocumentSource, userID int64) error
}

type UserService interface {
	middleware.UserResolver
	SetActiveSession(ctx context.Context, userID int64, sessionID uuid.UUID) error
}

type sessionRenamer interface {
	RenameSession(ctx context.Context, sessionID uuid.UUID, newTitle string, userID int64) error
}

type sessionService interface {
	handler.SessionService
	sessionRenamer
}

type mainHandler struct {
	downloader     *DocumentDownloader
	sessions       *handler.SessionHandler
	documents      *handler.DocumentHandler
	sessionRenamer sessionRenamer
	renameCache    *cache.InMemoryRenameCache
	ask            AskUseCase
	ingest         IngestUseCase
	userSvc        UserService
}

func NewBot(
	token string,
	webhookSecret string,
	downloader *DocumentDownloader,
	ask AskUseCase,
	ingest IngestUseCase,
	userSvc UserService,
	sessionSvc sessionService,
	documentSvc handler.DocumentService,
	logger *slog.Logger,
) (*bot.Bot, error) {
	renameCache := cache.NewInMemoryRenameCache()

	h := mainHandler{
		downloader:     downloader,
		sessions:       handler.NewSessionHandler(sessionSvc, renameCache),
		documents:      handler.NewDocumentHandler(documentSvc),
		sessionRenamer: sessionSvc,
		renameCache:    renameCache,
		ask:            ask,
		ingest:         ingest,
		userSvc:        userSvc,
	}

	b, err := bot.New(
		token,
		bot.WithWebhookSecretToken(webhookSecret),
		bot.WithDefaultHandler(h.handleDefault),
		bot.WithMiddlewares(
			middleware.Recover,
			middleware.Logger(logger),
			middleware.ResolveUser(userSvc),
		),
	)
	if err != nil {
		return nil, err
	}

	b.RegisterHandler(
		bot.HandlerTypeMessageText,
		handler.SessionsCommand,
		bot.MatchTypeExact,
		h.sessions.Handle,
	)
	b.RegisterHandler(
		bot.HandlerTypeCallbackQueryData,
		handler.SessionsCallbackPrefix,
		bot.MatchTypePrefix,
		h.sessions.HandleCallback,
	)
	b.RegisterHandler(
		bot.HandlerTypeMessageText,
		handler.DocumentsCommand,
		bot.MatchTypeExact,
		h.documents.Handle,
	)
	b.RegisterHandler(
		bot.HandlerTypeCallbackQueryData,
		handler.DocumentsCallbackPrefix,
		bot.MatchTypePrefix,
		h.documents.HandleCallback,
	)

	return b, nil
}

func (h *mainHandler) handleDefault(ctx context.Context, b *bot.Bot, upd *models.Update) {
	if upd.Message == nil {
		return
	}

	var (
		logger = appctx.Logger(ctx)
		userID = middleware.ContextUserID(ctx)
	)

	if upd.Message.ReplyToMessage != nil {
		if sessionID, ok := h.renameCache.Get(upd.Message.Chat.ID, userID, upd.Message.ReplyToMessage.ID); ok {
			title := upd.Message.Text
			if strings.TrimSpace(title) == "" {
				logger.Error("rename title is empty")
				reply.SendMessage(ctx, b, upd.Message.Chat.ID, reply.NewTitleIsEmpty)
				return
			}
			if err := h.sessionRenamer.RenameSession(ctx, sessionID, title, userID); err != nil {
				logger.Error("failed to rename session", "error", err)
				reply.SendMessage(ctx, b, upd.Message.Chat.ID, reply.UnknownError)
			}

			return
		}
	}

	switch {
	case upd.Message.Document != nil:
		h.handleIngest(ctx, b, upd.Message)
	case strings.TrimSpace(upd.Message.Text) != "":
		h.handleAsk(ctx, b, upd.Message, upd.ID)
	}
}
