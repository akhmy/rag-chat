package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"rag-chat/internal/config"
	"rag-chat/internal/http/handler"
	"rag-chat/internal/postgres"
	"rag-chat/internal/postgres/sqlc"
	"rag-chat/internal/usecase"
	"syscall"
	"time"

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

	oaiAPIKeyBytes, err := os.ReadFile(cfg.OpenAIAPIKeyFile)
	if err != nil {
		logger.Error("failed to read OpenAI API key", "error", err)
		return 1
	}

	client := openai.NewClient(option.WithAPIKey(string(oaiAPIKeyBytes)))
	embSvc := inopenai.NewEmbeddingService(client)
	agent := inopenai.NewAgent(client)

	dbPasswordBytes, err := os.ReadFile(cfg.DB.PasswordFile)
	if err != nil {
		logger.Error("failed to read DB password", "error", err)
		return 1
	}

	pool, err := postgres.Connect(context.Background(), postgres.Config{
		User:     cfg.DB.User,
		Password: string(dbPasswordBytes),
		Host:     cfg.DB.Host,
		Port:     cfg.DB.Port,
		Name:     cfg.DB.Name,
		Options:  cfg.DB.Options,
	})
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		return 1
	}

	q := sqlc.New(pool)

	sessionRepo := postgres.NewSessionRepository(q)

	documents := postgres.NewDocumentRepository(q)
	ask := handler.Ask(usecase.NewAsk(embSvc, documents, agent, sessionRepo))
	signUp := handler.SignUp(usecase.NewCreateUser(postgres.NewUserRepository(q)))
	sessions := handler.NewSessionHandler(usecase.NewSessionService(sessionRepo))

	srv := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           handler.NewRouter(ask, signUp, sessions, logger),
		ReadTimeout:       cfg.Server.ReadTimeout,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
	}

	notifyCtx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

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
