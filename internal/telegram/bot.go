package telegram

import (
	"context"
	"log/slog"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"rag-chat/internal/telegram/middleware"
)

type UserService interface {
	middleware.UserResolver
	SetActiveSession(ctx context.Context, userID int64, sessionID int64) error
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
	ask AskUseCase,
	ingest IngestUseCase,
	userSvc UserService,
	sessionSvc SessionService,
	logger *slog.Logger,
) (*bot.Bot, error) {
	h := &handler{
		downloader: downloader,
		ask:        ask,
		ingest:     ingest,
		userSvc:    userSvc,
		sessionSvc: sessionSvc,
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

	b.RegisterHandler(bot.HandlerTypeMessageText, "/sessions", bot.MatchTypeExact, h.handleSessions)
	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, sessionCallbackPrefix, bot.MatchTypePrefix, h.handleSessionsCallback)

	return b, nil
}

func (h *handler) handleDefault(ctx context.Context, b *bot.Bot, upd *models.Update) {
	msg := upd.Message
	if msg == nil {
		return
	}

	switch {
	case msg.Document != nil:
		h.handleDocument(ctx, b, msg)
	case strings.TrimSpace(msg.Text) != "":
		h.handleAsk(ctx, b, upd.ID, msg)
	}
}
