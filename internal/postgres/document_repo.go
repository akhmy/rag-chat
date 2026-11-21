package postgres

import (
	"context"
	"errors"
	"rag-chat/internal/domain"
	"rag-chat/internal/postgres/sqlc"
	"rag-chat/internal/usecase"

	"github.com/jackc/pgx/v5"
	"github.com/pgvector/pgvector-go"
)

type DocumentRepository struct {
	q *sqlc.Queries
}

func NewDocumentRepository(q *sqlc.Queries) *DocumentRepository {
	return &DocumentRepository{q: q}
}

func (repo *DocumentRepository) List(ctx context.Context, userID int64, offset, limit int) ([]domain.Document, error) {
	rows, err := repo.q.ListDocuments(ctx, sqlc.ListDocumentsParams{
		UserID: userID,
		Offset: int32(offset),
		Limit:  int32(limit),
	})
	if err != nil {
		return nil, err
	}

	documents := make([]domain.Document, 0, len(rows))
	for _, row := range rows {
		documents = append(documents, domain.Document{
			ID:    row.ID,
			Title: row.Title,
		})
	}

	return documents, nil
}

func (repo *DocumentRepository) Get(ctx context.Context, userID int64, id int64) (domain.Document, error) {
	row, err := repo.q.GetDocument(ctx, sqlc.GetDocumentParams{
		UserID: userID,
		ID:     id,
	})
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return domain.Document{}, domain.ErrDocumentNotFound
		default:
			return domain.Document{}, err
		}
	}

	return domain.Document{
		ID:    row.ID,
		Title: row.Title,
	}, nil
}

func (repo *DocumentRepository) Create(
	ctx context.Context, title string, userID int64,
) (int64, error) {
	return repo.q.InsertDocument(ctx, sqlc.InsertDocumentParams{
		Title:  title,
		UserID: userID,
	})
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
	ctx context.Context, query []float32, userID int64, limit int,
) ([]usecase.Chunk, error) {
	rows, err := repo.q.ListNearestChunks(ctx, sqlc.ListNearestChunksParams{
		Query:      pgvector.NewVector(query).String(),
		UserID:     userID,
		ChunkLimit: int32(limit),
	})
	if err != nil {
		return nil, err
	}

	chunks := make([]usecase.Chunk, 0, len(rows))
	for _, row := range rows {
		chunks = append(chunks, usecase.Chunk{
			Title:    row.Title,
			Content:  row.Content,
			Distance: row.Distance,
		})
	}

	return chunks, nil
}

func (repo *DocumentRepository) Delete(ctx context.Context, id int64, userID int64) error {
	n, err := repo.q.DeleteDocument(ctx, sqlc.DeleteDocumentParams{
		ID:     id,
		UserID: userID,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrDocumentNotFound
	}
	return nil
}
