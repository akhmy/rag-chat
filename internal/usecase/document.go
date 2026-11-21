package usecase

import (
	"context"
	"rag-chat/internal/domain"
)

type DocumentRepository interface {
	Get(ctx context.Context, userID int64, id int64) (domain.Document, error)
	List(ctx context.Context, userID int64, offset, limit int) ([]domain.Document, error)
	Delete(ctx context.Context, id int64, userID int64) error
}

type DocumentService struct {
	repo DocumentRepository
}

func NewDocumentService(repo DocumentRepository) *DocumentService {
	return &DocumentService{
		repo: repo,
	}
}

func (svc *DocumentService) ListDocuments(
	ctx context.Context, userID int64, offset, limit int,
) ([]domain.Document, error) {
	return svc.repo.List(ctx, userID, offset, limit)
}

func (svc *DocumentService) GetDocument(
	ctx context.Context, userID int64, id int64,
) (domain.Document, error) {
	return svc.repo.Get(ctx, userID, id)
}

func (svc *DocumentService) DeleteDocument(ctx context.Context, id int64, userID int64) error {
	return svc.repo.Delete(ctx, id, userID)
}
