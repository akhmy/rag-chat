package pipeline

import (
	"context"
	"fmt"
	"iter"
	"rag-chat/internal/appcontext"
	"strconv"
	"strings"
	"time"
)

type Source interface {
	Title() string
	Iter(ctx context.Context) iter.Seq2[string, error]
}

type BatchEmbeddingService interface {
	EmbedMany(ctx context.Context, input []string) ([][]float32, error)
}

type Ingest struct {
	repo   IngestRepo
	source Source
	embSvc BatchEmbeddingService
}

func NewIngest(repo IngestRepo, extractor Source, embSvc BatchEmbeddingService) *Ingest {
	return &Ingest{
		repo:   repo,
		source: extractor,
		embSvc: embSvc,
	}
}

type IngestRepo interface {
	InsertDocument(ctx context.Context, title string) (int64, error)
	DeleteDocument(ctx context.Context, id int64) error
	InsertChunks(
		ctx context.Context,
		documentID int64,
		contents []string,
		embeddings [][]float32,
	) error
	MarkDocumentAsIngested(ctx context.Context, documentID int64) error
}

var (
	ChunkSize    = 100
	ChunkOverlap = 25
)

func (pl *Ingest) Execute(ctx context.Context) error {
	logger := appcontext.Logger(ctx)

	documentID, err := pl.repo.InsertDocument(ctx, pl.source.Title())
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

		if err := pl.repo.DeleteDocument(cleanupCtx, documentID); err != nil {
			logger.Error("failed to delete document "+strconv.FormatInt(documentID, 10), "error", err)
		}
	}()

	for content, err := range pl.source.Iter(ctx) {
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

		embeddings, err := pl.embSvc.EmbedMany(ctx, batch)
		if err != nil {
			return fmt.Errorf("embed batch: %w", err)
		}

		if err := pl.repo.InsertChunks(ctx, documentID, batch, embeddings); err != nil {
			return fmt.Errorf("save chunks: %w", err)
		}
	}

	if err := pl.repo.MarkDocumentAsIngested(ctx, documentID); err != nil {
		return fmt.Errorf("mark document as ingested: %w", err)
	}

	fullyIngested = true

	return nil
}
