package usecase

import (
	"context"
	"fmt"
	"iter"
	"strconv"
	"strings"
	"time"
	"uuid"

	"rag-chat/internal/appctx"
	"rag-chat/internal/domain"

	"golang.org/x/sync/errgroup"
)

type Chunk struct {
	Title    string
	Content  string
	Distance float64
}

type queryEmbedder interface {
	Embed(ctx context.Context, input string) ([]float32, error)
}

type turnStore interface {
	CreateTurn(ctx context.Context, sessionID uuid.UUID, message domain.Message) (int64, error)
	CreateMessage(ctx context.Context, turnID int64, message domain.Message) error
	ListRecentMessages(ctx context.Context, sessionID uuid.UUID, upToID int64, limit int) ([]domain.Message, error)
}

type answerStreamer interface {
	Stream(ctx context.Context, messages []domain.Message) iter.Seq2[string, error]
}

type sessionTitleGenerator interface {
	GenerateTitle(ctx context.Context, query string) (string, error)
}

type askSessions interface {
	CreateSession(ctx context.Context, userID int64, title string) (uuid.UUID, error)
	GetSession(ctx context.Context, sessionID uuid.UUID, userID int64) (domain.Session, error)
	SetSessionTitle(ctx context.Context, sessionID uuid.UUID, userID int64, title string) error
}

type activeSessionReader interface {
	GetActiveSession(ctx context.Context, userID int64) (*domain.Session, error)
}

type chunkRetriever interface {
	ListNearestChunks(ctx context.Context, query []float32, userID int64, limit int) ([]Chunk, error)
}

type Ask struct {
	embedder       queryEmbedder
	chunks         chunkRetriever
	answers        answerStreamer
	turns          turnStore
	titleGenerator sessionTitleGenerator
	sessions       askSessions
	activeSessions activeSessionReader
}

func NewAsk(
	embedder queryEmbedder,
	chunks chunkRetriever,
	answers answerStreamer,
	turns turnStore,
	titles sessionTitleGenerator,
	sessions askSessions,
	activeSessions activeSessionReader,
) *Ask {
	return &Ask{
		embedder:       embedder,
		chunks:         chunks,
		answers:        answers,
		turns:          turns,
		titleGenerator: titles,
		sessions:       sessions,
		activeSessions: activeSessions,
	}
}

type AskInput struct {
	UserID     int64
	Query      string
	TurnLimit  int
	ChunkLimit int
}

func (uc *Ask) generateTitle(
	ctx context.Context,
	timeout time.Duration,
	query string,
	sessionID uuid.UUID,
	userID int64,
) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()

	logger := appctx.Logger(ctx)

	title, err := uc.titleGenerator.GenerateTitle(ctx, query)
	if err != nil {
		logger.Error("failed to generate title", "error", err)
		return
	}
	if err := uc.sessions.SetSessionTitle(ctx, sessionID, userID, title); err != nil {
		logger.Error("failed to set title", "error", err)
	}
}

func (uc *Ask) Execute(ctx context.Context, in *AskInput) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		session, err := uc.resolveActiveSession(ctx, in.UserID)
		if err != nil {
			yield("", err)
			return
		}

		queryMessage := domain.NewMessage(domain.MessageRoleUser, in.Query)

		turnID, err := uc.turns.CreateTurn(ctx, session.ID, queryMessage)
		if err != nil {
			yield("", fmt.Errorf("create turn: %w", err))
			return
		}

		if session.Title == "" {
			go uc.generateTitle(ctx, time.Second*5, in.Query, session.ID, in.UserID)
		}

		g, gCtx := errgroup.WithContext(ctx)

		var chunks []Chunk
		g.Go(func() error {
			embedding, err := uc.embedder.Embed(gCtx, in.Query)
			if err != nil {
				return fmt.Errorf("embed query: %w", err)
			}

			v, err := uc.chunks.ListNearestChunks(gCtx, embedding, in.UserID, 10)
			if err != nil {
				return fmt.Errorf("search chunks: %w", err)
			}

			chunks = v
			return nil
		})

		var messages []domain.Message
		g.Go(func() error {
			v, err := uc.turns.ListRecentMessages(gCtx, session.ID, turnID, in.TurnLimit-1)
			if err != nil {
				return fmt.Errorf("list recent messages: %w", err)
			}

			messages = v
			return nil
		})

		if err := g.Wait(); err != nil {
			yield("", err)
			return
		}

		queryMessage.Content = buildQuery(queryMessage.Content, chunks)
		messages = append(messages, queryMessage)

		var answer strings.Builder
		for delta, err := range uc.answers.Stream(ctx, messages) {
			if err != nil {
				yield("", fmt.Errorf("stream: %w", err))
				return
			}

			answer.WriteString(delta)
			if !yield(delta, nil) {
				return
			}
		}

		if err := uc.turns.CreateMessage(
			ctx, turnID, domain.NewMessage(domain.MessageRoleAssistant, answer.String()),
		); err != nil {
			yield("", fmt.Errorf("save assistant message: %w", err))
			return
		}

	}
}

func (uc *Ask) resolveActiveSession(ctx context.Context, userID int64) (domain.Session, error) {
	session, err := uc.activeSessions.GetActiveSession(ctx, userID)
	if err != nil {
		return domain.Session{}, fmt.Errorf("get active session: %w", err)
	}
	if session != nil {
		return *session, nil
	}

	id, err := uc.sessions.CreateSession(ctx, userID, "")
	if err != nil {
		return domain.Session{}, fmt.Errorf("create session: %w", err)
	}
	return domain.Session{ID: id}, nil
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
