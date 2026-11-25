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
	"strings"
	"syscall"
	"time"

	"github.com/go-telegram/bot"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	inopenai "rag-chat/internal/openai"
)

func main() {
	os.Exit(run())
}

type application struct {
	ask         *usecase.Ask
	ingest      *usecase.Ingest
	userSvc     *usecase.UserService
	sessionSvc  *usecase.SessionService
	documentSvc *usecase.DocumentService
}

func run() int {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg, err := config.ParseEnv[config.APIConfig]()
	if err != nil {
		logger.Error("failed to parse config", "error", err)
		return 1
	}

	pool, err := buildPool(context.Background(), cfg.DB)
	if err != nil {
		logger.Error("failed to build database pool", "error", err)
		return 1
	}
	defer pool.Close()

	app, err := buildApplication(cfg, sqlc.New(pool))
	if err != nil {
		logger.Error("failed to build application", "error", err)
		return 1
	}

	srv, b, err := buildServer(cfg, app, logger)
	if err != nil {
		logger.Error("failed to initialize server", "error", err)
		return 1
	}

	defer func() {
		if err := srv.Close(); err != nil {
			logger.Error("failed to close server", "error", err)
		}
	}()

	return serve(context.Background(), srv, b, cfg.Server.ShutdownTimeout, logger)
}

func buildPool(ctx context.Context, cfg config.DBConfig) (*pgxpool.Pool, error) {
	dbPasswordBytes, err := os.ReadFile(cfg.PasswordFile)
	if err != nil {
		return nil, fmt.Errorf("read db password: %w", err)
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

func buildApplication(cfg config.APIConfig, q *sqlc.Queries) (application, error) {
	// LLM
	oaiAPIKeyBytes, err := os.ReadFile(cfg.OpenAIAPIKeyFile)
	if err != nil {
		return application{}, fmt.Errorf("read OpenAI API key: %w", err)
	}
	client := openai.NewClient(option.WithAPIKey(string(oaiAPIKeyBytes)))

	embedder := inopenai.NewEmbedder(client)
	agent := inopenai.NewAgent(client)
	titleGenerator := inopenai.NewTitleGenerator(client)

	// REPOS
	userRepo := postgres.NewUserRepository(q)
	sessionRepo := postgres.NewSessionRepository(q)
	documentRepo := postgres.NewDocumentRepository(q)

	// SERVICES & USE CASES
	sessionSvc := usecase.NewSessionService(sessionRepo)
	ask := usecase.NewAsk(
		embedder,
		documentRepo,
		agent,
		sessionRepo,
		titleGenerator,
		sessionSvc,
		userRepo,
	)
	ingest := usecase.NewIngest(documentRepo, embedder)

	userSvc := usecase.NewUserService(userRepo)
	documentSvc := usecase.NewDocumentService(documentRepo)

	return application{
		ask:         ask,
		ingest:      ingest,
		userSvc:     userSvc,
		sessionSvc:  sessionSvc,
		documentSvc: documentSvc,
	}, nil
}

func buildServer(cfg config.APIConfig, app application, logger *slog.Logger) (*http.Server, *bot.Bot, error) {
	webhookSecretBytes, err := os.ReadFile(cfg.TelegramWebhookSecretFile)
	if err != nil {
		return nil, nil, fmt.Errorf("read Telegram webhook secret: %w", err)
	}

	webhookSecret := strings.TrimSpace(string(webhookSecretBytes))
	if len(webhookSecret) == 0 {
		return nil, nil, fmt.Errorf("missing Telegram webhook secret")
	}

	botTokenBytes, err := os.ReadFile(cfg.TelegramBotTokenFile)
	if err != nil {
		return nil, nil, fmt.Errorf("read Telegram bot token: %w", err)
	}

	downloader := telegram.NewDocumentDownloader(&http.Client{Timeout: time.Minute}, 10<<20)

	b, err := telegram.NewBot(
		string(botTokenBytes),
		webhookSecret,
		downloader,
		app.ask,
		app.ingest,
		app.userSvc,
		app.sessionSvc,
		app.documentSvc,
		logger,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create Telegram bot: %w", err)
	}

	router, err := handler.NewRouter(cfg.TelegramWebhookPath, b.WebhookHandler())
	if err != nil {
		return nil, nil, err
	}

	srv := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           router,
		ReadTimeout:       cfg.Server.ReadTimeout,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
	}

	return srv, b, nil
}

func serve(
	ctx context.Context,
	srv *http.Server,
	b *bot.Bot,
	shutdownTimeout time.Duration,
	logger *slog.Logger,
) int {
	notifyCtx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go b.StartWebhook(appctx.WithLogger(notifyCtx, logger))

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

		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
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
