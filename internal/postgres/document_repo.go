package postgres

import (
	"context"
	"rag-service/internal/pipeline"
	"rag-service/internal/postgres/sqlc"

	"github.com/pgvector/pgvector-go"
)

type DocumentRepository struct {
	q *sqlc.Queries
}

func NewDocumentRepository(q *sqlc.Queries) *DocumentRepository {
	return &DocumentRepository{q: q}
}

func (repo *DocumentRepository) InsertDocument(
	ctx context.Context, title string,
) (int64, error) {
	return repo.q.InsertDocument(ctx, title)
}

func (repo *DocumentRepository) DeleteDocument(ctx context.Context, id int64) error {
	return repo.q.DeleteDocument(ctx, id)
}

func (repo *DocumentRepository) InsertChunks(
	ctx context.Context, documentID int64, contents []string,
	embeddings [][]float32,
) error {
	vectors := make([]string, 0, len(embeddings))
	for _, embedding := range embeddings {
		vectors = append(vectors, pgvector.NewVector(embedding).String())
	}

	if err := repo.q.InsertChunks(ctx, sqlc.InsertChunksParams{
		DocumentID: documentID,
		Contents:   contents,
		Embeddings: vectors,
	}); err != nil {
		return err
	}

	return nil
}

func (repo *DocumentRepository) MarkDocumentAsIngested(
	ctx context.Context, documentID int64,
) error {
	return repo.q.MarkDocumentAsIngested(ctx, documentID)
}

func (repo *DocumentRepository) ListNearestChunks(
	ctx context.Context, query []float32, limit int,
) ([]pipeline.Chunk, error) {
	rows, err := repo.q.ListNearestChunks(ctx, sqlc.ListNearestChunksParams{
		Query:       pgvector.NewVector(query).String(),
		ResultLimit: int32(limit),
	})
	if err != nil {
		return nil, err
	}

	chunks := make([]pipeline.Chunk, 0, len(rows))
	for _, row := range rows {
		chunks = append(chunks, pipeline.Chunk{
			Title:    row.Title,
			Content:  row.Content,
			Distance: row.Distance,
		})
	}

	return chunks, nil
}
