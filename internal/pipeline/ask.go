package pipeline

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

type Chunk struct {
	Title    string
	Content  string
	Distance float64
}

type EmbeddingService interface {
	EmbedOne(ctx context.Context, input string) ([]float32, error)
}

type Agent interface {
	Ask(ctx context.Context, query string) (string, error)
}

type ChunkSearcher interface {
	ListNearestChunks(ctx context.Context, query []float32, limit int) ([]Chunk, error)
}

type Ask struct {
	embSvc EmbeddingService
	chunks ChunkSearcher
	agent  Agent
}

func NewAsk(embSvc EmbeddingService, ragRepo ChunkSearcher, agent Agent) *Ask {
	return &Ask{
		embSvc: embSvc,
		chunks: ragRepo,
		agent:  agent,
	}
}

func (pl *Ask) Execute(ctx context.Context, query string, limit int) (string, error) {
	embedding, err := pl.embSvc.EmbedOne(ctx, query)
	if err != nil {
		return "", fmt.Errorf("embed query: %w", err)
	}

	chunks, err := pl.chunks.ListNearestChunks(ctx, embedding, limit)
	if err != nil {
		return "", fmt.Errorf("list nearest chunks: %w", err)
	}

	reply, err := pl.agent.Ask(ctx, buildQuery(query, chunks))
	if err != nil {
		return "", err
	}

	return reply, nil
}

func buildQuery(query string, chunks []Chunk) string {
	var sb strings.Builder

	sb.WriteString("Retrieved context:\n\n")

	for i, chunk := range chunks {
		sb.WriteString("[" + strconv.Itoa(i+1) + "] " + chunk.Title + "\n")
		sb.WriteString(chunk.Content)
		sb.WriteString("\n\n")
	}

	sb.WriteString("Question:\n")
	sb.WriteString(query)

	return sb.String()
}
