package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"rag-chat/internal/appctx"
	"rag-chat/internal/config"
	"rag-chat/internal/http/handler"
	"rag-chat/internal/postgres"
	"rag-chat/internal/postgres/sqlc"
	"rag-chat/internal/telegram"
	"rag-chat/internal/usecase"
	"syscall"
	"time"

	"github.com/go-telegram/bot"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	inopenai "rag-chat/internal/openai"
)

const (
	serverShutdownTimeout = time.Second * 10
)

func main() {
	os.Exit(run())
}

func run() int {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg, err := config.ParseEnv[config.APIConfig]()
	if err != nil {
		logger.Error("failed to parse config", "error", err)
		return 1
	}

	pool, err := connectDatabase(context.Background(), cfg.DB)
	if err != nil {
		logger.Error("failed to initialize database", "error", err)
		return 1
	}
	defer pool.Close()

	srv, telegramBot, err := buildServer(cfg, sqlc.New(pool), logger)
	if err != nil {
		logger.Error("failed to initialize server", "error", err)
		return 1
	}

	return serve(srv, telegramBot, logger)
}

func connectDatabase(ctx context.Context, cfg config.DBConfig) (*pgxpool.Pool, error) {
	dbPasswordBytes, err := os.ReadFile(cfg.PasswordFile)
	if err != nil {
		return nil, fmt.Errorf("read DB password: %w", err)
	}

	pool, err := postgres.Connect(ctx, postgres.Config{
		User:     cfg.User,
		Password: string(dbPasswordBytes),
		Host:     cfg.Host,
		Port:     cfg.Port,
		Name:     cfg.Name,
		Options:  cfg.Options,
	})
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	return pool, nil
}

func buildServer(cfg config.APIConfig, q *sqlc.Queries, logger *slog.Logger) (*http.Server, *bot.Bot, error) {
	oaiAPIKeyBytes, err := os.ReadFile(cfg.OpenAIAPIKeyFile)
	if err != nil {
		return nil, nil, fmt.Errorf("read OpenAI API key: %w", err)
	}

	client := openai.NewClient(option.WithAPIKey(string(oaiAPIKeyBytes)))
	embSvc := inopenai.NewEmbeddingService(client)
	agent := inopenai.NewAgent(client)
	titleGen := inopenai.NewTitleGenerator(client)

	sessionRepo := postgres.NewSessionRepository(q)
	userRepo := postgres.NewUserRepository(q)

	sessionSvc := usecase.NewSessionService(sessionRepo)
	userSvc := usecase.NewUserService(userRepo)
	documents := postgres.NewDocumentRepository(q)

	askUC := usecase.NewAsk(embSvc, documents, agent, sessionRepo, titleGen, sessionRepo, userRepo)
	askHandler := handler.NewAskHandler(askUC, handler.AskHandlerConfig{
		Timeout:        time.Second * 15,
		MaxQueryLength: 2048,
		RetrieveLimit:  10,
	})
	sessionHandler := handler.NewSessionHandler(sessionSvc)

	webhookSecretBytes, err := os.ReadFile(cfg.TelegramWebhookSecretFile)
	if err != nil {
		return nil, nil, fmt.Errorf("read Telegram webhook secret: %w", err)
	}

	botTokenBytes, err := os.ReadFile(cfg.TelegramBotTokenFile)
	if err != nil {
		return nil, nil, fmt.Errorf("read Telegram bot token: %w", err)
	}

	bot, err := telegram.WireBot(string(botTokenBytes), string(webhookSecretBytes), userSvc, askUC, sessionSvc, logger)
	if err != nil {
		return nil, nil, fmt.Errorf("create Telegram bot: %w", err)
	}

	srv := &http.Server{
		Addr: cfg.Server.Addr,
		Handler: handler.NewRouter(
			askHandler,
			handler.SignUp(userSvc),
			sessionHandler,
			bot.WebhookHandler(),
			logger,
		),
		ReadTimeout:       cfg.Server.ReadTimeout,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
	}

	return srv, bot, nil
}

func serve(srv *http.Server, telegramBot *bot.Bot, logger *slog.Logger) int {
	notifyCtx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go telegramBot.StartWebhook(appctx.WithLogger(notifyCtx, logger))

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("failed to listen and serve", "error", err)
			return 1
		}
		logger.Info("server stopped")
		return 0
	case <-notifyCtx.Done():
		logger.Info("shutting down server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), serverShutdownTimeout)
		err := srv.Shutdown(shutdownCtx)
		cancel()
		if err != nil {
			logger.Warn("failed to shutdown server gracefully", "error", err)
			return 1
		}
		logger.Info("server shut down gracefully")
		return 0
	}
}
