package main

import (
	"context"
	"log/slog"
	"os"
	"rag-chat/internal/appcontext"
	"rag-chat/internal/config"
	"rag-chat/internal/extract"
	"rag-chat/internal/pipeline"
	"rag-chat/internal/postgres"
	"rag-chat/internal/postgres/sqlc"
	"strings"

	inopenai "rag-chat/internal/openai"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

func main() {
	os.Exit(func() int {
		logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

		cfg, err := config.ParseEnv[config.IngestConfig]()
		if err != nil {
			logger.Error("failed to parse config", "error", err)
			return 1
		}

		dbPasswordBytes, err := os.ReadFile(cfg.DB.PasswordFile)
		if err != nil {
			logger.Error("failed to read db password", "error", err)
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
			logger.Error("failed to connect to db", "error", err)
			return 1
		}

		q := sqlc.New(pool)
		documents := postgres.NewDocumentRepository(q)

		oaiAPIKeyBytes, err := os.ReadFile(cfg.OpenAIAPIKeyFile)
		if err != nil {
			logger.Error("failed to read OpenAI API key", "error", err)
			return 1
		}

		emb := inopenai.NewEmbeddingService(
			openai.NewClient(
				option.WithAPIKey(string(oaiAPIKeyBytes)),
			),
		)

		for _, arg := range os.Args[1:] {
			path, title, found := strings.Cut(arg, "=")
			if !found {
				logger.Error("failed to parse arg", "arg", arg)
				return 1
			}

			var source pipeline.Source
			if strings.HasSuffix(path, ".pdf") {
				source = extract.NewPDFSource(path, title)
			} else if strings.HasSuffix(path, ".txt") {
				source = extract.NewTXTSource(path, title)
			} else {
				logger.Error("unknown document type", "arg", arg)
				continue
			}

			ctx := appcontext.WithLogger(context.Background(), logger)
			if err := pipeline.NewIngest(documents, source, emb).Execute(ctx); err != nil {
				logger.Error("failed to ingest", "error", err)
				return 1
			}
		}

		return 0
	}())
}
