package usecase

import (
	"context"
	"fmt"
	"iter"
	"strconv"
	"strings"
	"sync"
	"time"
	"uuid"

	"rag-chat/internal/appctx"
	"rag-chat/internal/domain"
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
	ListRecentMessages(ctx context.Context, sessionID uuid.UUID, limit int) ([]domain.Message, error)
}

type answerStreamer interface {
	Stream(ctx context.Context, messages []domain.Message) iter.Seq2[string, error]
}

type chunkRetriever interface {
	ListNearestChunks(ctx context.Context, query []float32, userID int64, limit int) ([]Chunk, error)
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

type Ask struct {
	embedder       queryEmbedder
	chunks         chunkRetriever
	answers        answerStreamer
	turns          turnStore
	titles         sessionTitleGenerator
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
		titles:         titles,
		sessions:       sessions,
		activeSessions: activeSessions,
	}
}

type AskInput struct {
	UserID     int64
	SessionID  uuid.UUID
	Query      string
	TurnsLimit int
	ChunkLimit int
}

func (uc *Ask) Execute(ctx context.Context, in *AskInput) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		logger := appctx.Logger(ctx)

		session, err := uc.resolveSession(ctx, in.UserID, in.SessionID)
		if err != nil {
			yield("", err)
			return
		}

		titleCtx, cancel := context.WithTimeout(ctx, time.Second*5)

		var wg sync.WaitGroup

		defer func() {
			cancel()
			wg.Wait()
		}()

		if session.Title == "" {
			wg.Go(func() {
				title, err := uc.titles.GenerateTitle(titleCtx, in.Query)
				if err != nil {
					logger.Error("failed to generate title", "error", err)
					return
				}
				if err := uc.sessions.SetSessionTitle(ctx, session.ID, in.UserID, title); err != nil {
					logger.Error("failed to set title", "error", err)
				}
			})
		}

		turnID, err := uc.turns.CreateTurn(ctx, session.ID, domain.NewMessage(domain.MessageRoleUser, in.Query))
		if err != nil {
			yield("", fmt.Errorf("create turn: %w", err))
			return
		}

		messages, err := uc.turns.ListRecentMessages(ctx, session.ID, in.TurnsLimit)
		if err != nil {
			yield("", fmt.Errorf("list recent messages: %w", err))
			return
		}

		embedding, err := uc.embedder.Embed(ctx, in.Query)
		if err != nil {
			yield("", fmt.Errorf("embed query: %w", err))
			return
		}

		chunks, err := uc.chunks.ListNearestChunks(ctx, embedding, in.UserID, 10)
		if err != nil {
			yield("", fmt.Errorf("list nearest chunks: %w", err))
			return
		}

		messages[len(messages)-1].Content = buildQuery(messages[len(messages)-1].Content, chunks)

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

		wg.Wait()
	}
}

func (uc *Ask) resolveSession(ctx context.Context, userID int64, sessionID uuid.UUID) (domain.Session, error) {
	if sessionID != uuid.Nil() {
		session, err := uc.sessions.GetSession(ctx, sessionID, userID)
		if err != nil {
			return domain.Session{}, fmt.Errorf("get session: %w", err)
		}
		return session, nil
	}

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
