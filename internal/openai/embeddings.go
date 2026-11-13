package openai

import (
	"context"

	"github.com/openai/openai-go/v3"
)

type EmbeddingService struct {
	client openai.Client
}

func NewEmbeddingService(client openai.Client) *EmbeddingService {
	return &EmbeddingService{
		client: client,
	}
}

func (svc *EmbeddingService) EmbedMany(ctx context.Context, input []string) ([][]float32, error) {
	res, err := svc.client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Model: openai.EmbeddingModelTextEmbedding3Small,
		Input: openai.EmbeddingNewParamsInputUnion{
			OfArrayOfStrings: input,
		},
	})
	if err != nil {
		return nil, err
	}

	embeddings := make([][]float32, 0, len(input))

	for _, item := range res.Data {
		embedding := make([]float32, 0, len(item.Embedding))
		for _, value := range item.Embedding {
			embedding = append(embedding, float32(value))
		}
		embeddings = append(embeddings, embedding)
	}

	return embeddings, nil
}

func (svc *EmbeddingService) EmbedOne(ctx context.Context, input string) ([]float32, error) {
	embeddings, err := svc.EmbedMany(ctx, []string{input})
	if err != nil {
		return nil, err
	}

	return embeddings[0], nil
}
