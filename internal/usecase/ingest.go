package usecase

import (
	"context"
	"fmt"
	"iter"
	"strconv"
	"strings"
	"time"

	"rag-chat/internal/appctx"
)

type DocumentSource interface {
	Title() string
	Iter(ctx context.Context) iter.Seq2[string, error]
}

type batchEmbedder interface {
	EmbedBatch(ctx context.Context, input []string) ([][]float32, error)
}

type documentStore interface {
	Create(ctx context.Context, title string, userID int64) (int64, error)
	Delete(ctx context.Context, id int64, userID int64) error
	InsertChunks(
		ctx context.Context,
		documentID int64,
		contents []string,
		embeddings [][]float32,
	) error
	MarkDocumentAsIngested(ctx context.Context, documentID int64) error
}

type Ingest struct {
	repo     documentStore
	embedder batchEmbedder
}

func NewIngest(repo documentStore, embedder batchEmbedder) *Ingest {
	return &Ingest{
		repo:     repo,
		embedder: embedder,
	}
}

var (
	ChunkSize    = 100
	ChunkOverlap = 25
)

func (uc *Ingest) Execute(ctx context.Context, source DocumentSource, userID int64) error {
	logger := appctx.Logger(ctx)

	documentID, err := uc.repo.Create(ctx, source.Title(), userID)
	if err != nil {
		return fmt.Errorf("save document: %w", err)
	}

	var fullyIngested bool

	defer func() {
		if fullyIngested {
			return
		}

		logger.Error("failed to fully ingest document")

		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second*3)
		defer cancel()

		if err := uc.repo.Delete(cleanupCtx, documentID, userID); err != nil {
			logger.Error("failed to delete document "+strconv.FormatInt(documentID, 10), "error", err)
		}
	}()

	for content, err := range source.Iter(ctx) {
		if err != nil {
			return fmt.Errorf("extract: %w", err)
		}

		words := strings.Fields(content)

		batch := make([]string, 0)

		step := ChunkSize - ChunkOverlap
		for start := 0; start < len(words); start += step {
			end := min(start+ChunkSize, len(words))

			batch = append(batch, strings.Join(words[start:end], " "))

			if end == len(words) {
				break
			}
		}

		embeddings, err := uc.embedder.EmbedBatch(ctx, batch)
		if err != nil {
			return fmt.Errorf("embed batch: %w", err)
		}

		if err := uc.repo.InsertChunks(ctx, documentID, batch, embeddings); err != nil {
			return fmt.Errorf("save chunks: %w", err)
		}
	}

	if err := uc.repo.MarkDocumentAsIngested(ctx, documentID); err != nil {
		return fmt.Errorf("mark document as ingested: %w", err)
	}

	fullyIngested = true

	return nil
}
