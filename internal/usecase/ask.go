package usecase

import (
	"context"
	"fmt"
	"iter"
	"rag-chat/internal/appctx"
	"rag-chat/internal/domain"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Chunk struct {
	Title    string
	Content  string
	Distance float64
}

type Embedder interface {
	EmbedOne(ctx context.Context, input string) ([]float32, error)
}

type MessageRepository interface {
	CreateTurn(ctx context.Context, sessionID int64, m domain.Message) (int64, error)
	CreateMessage(ctx context.Context, turnID int64, m domain.Message) error
	ListRecentMessages(ctx context.Context, sessionID int64, limit int) ([]domain.Message, error)
}

type Agent interface {
	Ask(ctx context.Context, messages []domain.Message) (string, error)
	Stream(ctx context.Context, messages []domain.Message) iter.Seq2[string, error]
}

type ChunkSearcher interface {
	ListNearestChunks(ctx context.Context, query []float32, limit int) ([]Chunk, error)
}

type TitleGenerator interface {
	GenerateTitle(ctx context.Context, query string) (string, error)
}

type Ask struct {
	embedder      Embedder
	chunks        ChunkSearcher
	agent         Agent
	messages      MessageRepository
	titleGen      TitleGenerator
	sessionRepo   AskSessionRepository
	sessionGetter ActiveSessionGetter
}

type TitleSetter interface {
	SetSessionTitle(ctx context.Context, id int64, title string) error
}

type AskSessionRepository interface {
	Get(ctx context.Context, sessionID int64) (domain.Session, error)
	SetTitle(ctx context.Context, sessionID int64, title string) error
	Create(ctx context.Context, userID int64, title string) (int64, error)
}

type ActiveSessionGetter interface {
	GetActiveSession(ctx context.Context, userID int64) (*domain.Session, error)
}

func NewAsk(
	embSvc Embedder,
	ragRepo ChunkSearcher,
	agent Agent,
	messages MessageRepository,
	titleGen TitleGenerator,
	sessionRepo AskSessionRepository,
	sessionGetter ActiveSessionGetter,
) *Ask {
	return &Ask{
		titleGen:      titleGen,
		embedder:      embSvc,
		chunks:        ragRepo,
		agent:         agent,
		messages:      messages,
		sessionRepo:   sessionRepo,
		sessionGetter: sessionGetter,
	}
}

type AskInput struct {
	UserID     int64
	SessionID  *int64
	Query      string
	TurnsLimit int
	ChunkLimit int
}

func (uc *Ask) Execute(ctx context.Context, in *AskInput) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		logger := appctx.Logger(ctx)

		// TITLE
		var session domain.Session
		if in.SessionID == nil {
			sessionPtr, err := uc.sessionGetter.GetActiveSession(ctx, in.UserID)
			if err != nil {
				yield("", fmt.Errorf("get active session: %w", err))
			}
			if sessionPtr == nil {
				id, err := uc.sessionRepo.Create(ctx, in.UserID, "")
				if err != nil {
					yield("", fmt.Errorf("create session: %w", err))
					return
				}
				session = domain.Session{ID: id}
			} else {
				session = *sessionPtr
			}
		} else {
			var err error
			session, err = uc.sessionRepo.Get(ctx, *in.SessionID)
			if err != nil {
				yield("", fmt.Errorf("get session: %w", err))
				return
			}
		}

		titleCtx, cancel := context.WithTimeout(ctx, time.Second*5)

		var wg sync.WaitGroup

		defer func() {
			cancel()
			wg.Wait()
		}()

		if session.Title == "" {
			wg.Go(func() {
				title, err := uc.titleGen.GenerateTitle(titleCtx, in.Query)
				if err != nil {
					logger.Error("failed to generate title", "error", err)
					return
				}
				if err := uc.sessionRepo.SetTitle(ctx, session.ID, title); err != nil {
					logger.Error("failed to set title", "error", err)
				}
			})
		}

		turnID, err := uc.messages.CreateTurn(ctx, session.ID, domain.NewMessage(domain.MessageRoleUser, in.Query))
		if err != nil {
			yield("", fmt.Errorf("create turn: %w", err))
			return
		}

		messages, err := uc.messages.ListRecentMessages(ctx, session.ID, in.TurnsLimit)
		if err != nil {
			yield("", fmt.Errorf("list recent messages: %w", err))
			return
		}

		embedding, err := uc.embedder.EmbedOne(ctx, in.Query)
		if err != nil {
			yield("", fmt.Errorf("embed query: %w", err))
			return
		}

		chunks, err := uc.chunks.ListNearestChunks(ctx, embedding, 10)
		if err != nil {
			yield("", fmt.Errorf("list nearest chunks: %w", err))
			return
		}

		messages[len(messages)-1].Content = buildQuery(messages[len(messages)-1].Content, chunks)

		var answer strings.Builder
		for delta, err := range uc.agent.Stream(ctx, messages) {
			if err != nil {
				yield("", fmt.Errorf("stream: %w", err))
				return
			}

			answer.WriteString(delta)
			if !yield(delta, nil) {
				return
			}
		}

		if err := uc.messages.CreateMessage(
			ctx, turnID, domain.NewMessage(domain.MessageRoleAssistant, answer.String()),
		); err != nil {
			yield("", fmt.Errorf("save assistant message: %w", err))
			return
		}

		wg.Wait()
	}
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
