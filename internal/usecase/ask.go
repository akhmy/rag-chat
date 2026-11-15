package usecase

import (
	"context"
	"fmt"
	"rag-chat/internal/domain"
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
	Ask(ctx context.Context, messages []domain.Message) (string, error)
}

type ChunkSearcher interface {
	ListNearestChunks(ctx context.Context, query []float32, limit int) ([]Chunk, error)
}

type Ask struct {
	embSvc   EmbeddingService
	chunks   ChunkSearcher
	agent    Agent
	messages MessageRepository
}

type MessageRepository interface {
	CreateTurn(ctx context.Context, sessionID int64, m domain.Message) (int64, error)
	CreateMessage(ctx context.Context, turnID int64, m domain.Message) error
	ListRecentMessages(ctx context.Context, sessionID int64, limit int) ([]domain.Message, error)
}

func NewAsk(embSvc EmbeddingService, ragRepo ChunkSearcher, agent Agent, messages MessageRepository) *Ask {
	return &Ask{
		embSvc:   embSvc,
		chunks:   ragRepo,
		agent:    agent,
		messages: messages,
	}
}

func (uc *Ask) Execute(ctx context.Context, sessionID int64, query string, limit int) (string, error) {
	turnID, err := uc.messages.CreateTurn(ctx, sessionID, domain.NewMessage(domain.MessageRoleUser, query))
	if err != nil {
		return "", fmt.Errorf("create turn: %w", err)
	}

	messages, err := uc.messages.ListRecentMessages(ctx, sessionID, 10)
	if err != nil {
		return "", fmt.Errorf("list recent messages: %w", err)
	}

	embedding, err := uc.embSvc.EmbedOne(ctx, query)
	if err != nil {
		return "", fmt.Errorf("embed query: %w", err)
	}

	chunks, err := uc.chunks.ListNearestChunks(ctx, embedding, limit)
	if err != nil {
		return "", fmt.Errorf("list nearest chunks: %w", err)
	}

	messages[len(messages)-1].Content = buildQuery(messages[len(messages)-1].Content, chunks)

	reply, err := uc.agent.Ask(ctx, messages)
	if err != nil {
		return "", err
	}

	if err := uc.messages.CreateMessage(
		ctx, turnID, domain.NewMessage(domain.MessageRoleAssistant, reply),
	); err != nil {
		return "", fmt.Errorf("save assistant message: %w", err)
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
