package usecase

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"uuid"

	"rag-chat/internal/appctx"
	"rag-chat/internal/domain"

	"golang.org/x/sync/errgroup"
)

type Chunk struct {
	Title   string
	Content string

	VectorSearchRank int
	TextSearchRank   int
	RFFScore         float64
}

type queryEmbedder interface {
	Embed(ctx context.Context, input string) ([]float32, error)
}

type turnStore interface {
	CreateTurn(ctx context.Context, sessionID uuid.UUID, message domain.Message) (int64, error)
	CreateMessage(ctx context.Context, turnID int64, message domain.Message) error
	ListRecentMessages(ctx context.Context, sessionID uuid.UUID, upToID int64, limit int) ([]domain.Message, error)
}

type AnswerEvent struct {
	Delta       string
	TotalTokens *int64
}

type answerStreamer interface {
	Stream(ctx context.Context, messages []domain.Message) iter.Seq2[AnswerEvent, error]
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

type tokenUsageStore interface {
	AddTokenUsage(ctx context.Context, userID int64, used int64) error
	GetTokenUsage(ctx context.Context, userID int64, since time.Time) (int64, error)
}

type keywordRetriever interface {
	RetrieveKeywords(ctx context.Context, query string) (string, error)
}

type chunkSearcher interface {
	SearchChunks(
		ctx context.Context,
		queryText string,
		queryEmbedding []float32,
		limit int,
		userID int64,
	) ([]Chunk, error)
}

type Ask struct {
	embedder         queryEmbedder
	chunkSearcher    chunkSearcher
	answers          answerStreamer
	turns            turnStore
	titleGenerator   sessionTitleGenerator
	sessions         askSessions
	tokenUsage       tokenUsageStore
	activeSessions   activeSessionReader
	keywordRetriever keywordRetriever
}

func NewAsk(
	embedder queryEmbedder,
	chunkSearcher chunkSearcher,
	answers answerStreamer,
	turns turnStore,
	titles sessionTitleGenerator,
	sessions askSessions,
	activeSessions activeSessionReader,
	tokenUsage tokenUsageStore,
	keywordRetriever keywordRetriever,
) *Ask {
	return &Ask{
		embedder:         embedder,
		chunkSearcher:    chunkSearcher,
		answers:          answers,
		turns:            turns,
		titleGenerator:   titles,
		sessions:         sessions,
		activeSessions:   activeSessions,
		tokenUsage:       tokenUsage,
		keywordRetriever: keywordRetriever,
	}
}

type AskInput struct {
	UserID           int64
	Query            string
	TurnLimit        int
	ChunkLimit       int
	TokenUsageLimit  int64
	TokenUsageWindow time.Duration
}

var ErrTokenUsageLimitExceeded = errors.New("token usage limit exceeded")

func (uc *Ask) SearchChunks(ctx context.Context, query string, limit int, userID int64) ([]Chunk, error) {
	logger := appctx.Logger(ctx)

	g, gCtx := errgroup.WithContext(ctx)

	var keywords string
	g.Go(func() error {
		v, err := uc.keywordRetriever.RetrieveKeywords(gCtx, query)
		if err != nil {
			logger.Error("failed to retrieve keywords", "error", err)
			keywords = query
		} else {
			keywords = v
		}

		return nil
	})

	var embedding []float32
	g.Go(func() error {
		v, err := uc.embedder.Embed(gCtx, query)
		if err != nil {
			return fmt.Errorf("embed query: %w", err)
		}

		embedding = v
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	if logger.Enabled(ctx, slog.LevelDebug) {
		logger.Debug("keywords retrieved", "keywords", keywords)
	}

	chunks, err := uc.chunkSearcher.SearchChunks(ctx, keywords, embedding, limit, userID)
	if err != nil {
		return nil, err
	}

	return chunks, nil
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
		logger := appctx.Logger(ctx)

		usage, err := uc.tokenUsage.GetTokenUsage(ctx, in.UserID, time.Now().Add(-in.TokenUsageWindow))
		if err != nil {
			yield("", fmt.Errorf("get token usage: %w", err))
			return
		}
		if usage > in.TokenUsageLimit {
			yield("", ErrTokenUsageLimitExceeded)
			return
		}

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
			v, err := uc.SearchChunks(ctx, in.Query, in.ChunkLimit, in.UserID)
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

		if logger.Enabled(ctx, slog.LevelDebug) {
			logChunks(ctx, logger.With("turn_id", turnID), chunks)
		}

		queryMessage.Content = buildQuery(queryMessage.Content, chunks)
		messages = append(messages, queryMessage)

		var answer strings.Builder
		for event, err := range uc.answers.Stream(ctx, messages) {
			if event.TotalTokens != nil {
				if err := uc.tokenUsage.AddTokenUsage(ctx, in.UserID, *event.TotalTokens); err != nil {
					yield("", fmt.Errorf("add token usage: %w", err))
				}
			}

			if err != nil {
				yield("", fmt.Errorf("stream: %w", err))
				return
			}

			if event.TotalTokens != nil {
				continue
			}

			answer.WriteString(event.Delta)
			if !yield(event.Delta, nil) {
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

func logChunks(ctx context.Context, logger *slog.Logger, chunks []Chunk) {
	logger.DebugContext(ctx, "retrieved chunks", "count", len(chunks))

	for i, chunk := range chunks {
		preview := []rune(strings.Join(strings.Fields(chunk.Content), " "))
		text := string(preview)

		if len(preview) > 120 {
			text = string(preview[:120]) + "..."
		}

		logger.DebugContext(
			ctx, "retrieved chunk",
			"pos", i+1,
			"vector_rank", chunk.VectorSearchRank,
			"text_rank", chunk.TextSearchRank,
			"rff", chunk.RFFScore,
			"title", chunk.Title,
			"preview", text,
		)
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
