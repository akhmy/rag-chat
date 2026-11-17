package openai

import (
	"context"
	"errors"

	"github.com/openai/openai-go/v3"
)

type Embedder struct {
	client openai.Client
}

func NewEmbedder(client openai.Client) *Embedder {
	return &Embedder{
		client: client,
	}
}

func (e *Embedder) EmbedBatch(ctx context.Context, input []string) ([][]float32, error) {
	res, err := e.client.Embeddings.New(ctx, openai.EmbeddingNewParams{
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

func (e *Embedder) Embed(ctx context.Context, input string) ([]float32, error) {
	res, err := e.client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Model: openai.EmbeddingModelTextEmbedding3Small,
		Input: openai.EmbeddingNewParamsInputUnion{
			OfString: openai.String(input),
		},
	})
	if err != nil {
		return nil, err
	}

	if len(res.Data) < 1 {
		return nil, errors.New("failed to read embedding data")
	}

	embedding := make([]float32, 0, len(res.Data[0].Embedding))
	for _, value := range res.Data[0].Embedding {
		embedding = append(embedding, float32(value))
	}

	return embedding, nil
}
