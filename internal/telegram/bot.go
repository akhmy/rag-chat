package telegram

import (
	"context"
	"log/slog"
	"rag-chat/internal/domain"

	"github.com/go-telegram/bot"
)

const (
	startCommand    = "/start"
	commandSessions = "/sessions"
	sessionsPrefix  = "sessions:"
)

type UserService interface {
	CreateUser(ctx context.Context, telegramID *int64) (domain.User, error)
	GetUserByTelegramID(ctx context.Context, telegramID int64) (domain.User, error)
	SetActiveSession(ctx context.Context, userID int64, sessionID int64) error
}

type SessionService interface {
	CreateSession(ctx context.Context, userID int64, title string) (int64, error)
	ListSessions(ctx context.Context, userID int64) ([]domain.Session, error)
	GetSession(ctx context.Context, sessionID int64) (domain.Session, error)
}

type Bot struct {
	sdk        *bot.Bot
	ask        AskUseCase
	userSvc    UserService
	sessionSvc SessionService
}

func WireBot(token, webhookSecret string, userSvc UserService, askUC AskUseCase, sessionSvc SessionService, logger *slog.Logger) (*bot.Bot, error) {
	b := &Bot{
		userSvc:    userSvc,
		ask:        askUC,
		sessionSvc: sessionSvc,
	}

	sdk, err := bot.New(
		token,
		bot.WithWebhookSecretToken(webhookSecret),
		bot.WithDefaultHandler(b.handleAsk),
		bot.WithMiddlewares(Recover, Logger(logger), ResolveUser(userSvc)),
	)
	if err != nil {
		return nil, err
	}

	b.sdk = sdk

	sdk.RegisterHandler(bot.HandlerTypeMessageText, startCommand, bot.MatchTypeExact, b.handleStart)
	sdk.RegisterHandler(bot.HandlerTypeMessageText, commandSessions, bot.MatchTypeExact, b.handleSessions)
	sdk.RegisterHandler(bot.HandlerTypeCallbackQueryData, sessionsPrefix, bot.MatchTypePrefix, b.handleSessionsCallback)

	return sdk, nil
}
