package postgres

import (
	"context"
	"database/sql"
	"errors"
	"rag-chat/internal/domain"
	"rag-chat/internal/postgres/sqlc"
	"uuid"

	"github.com/jackc/pgx/v5/pgtype"
)

type SessionRepository struct {
	q *sqlc.Queries
}

func NewSessionRepository(q *sqlc.Queries) *SessionRepository {
	return &SessionRepository{q: q}
}

func (repo *SessionRepository) Create(ctx context.Context, sessionID uuid.UUID, userID int64, title string) (uuid.UUID, error) {
	id, err := repo.q.CreateSession(ctx, sqlc.CreateSessionParams{
		ID:     pgtype.UUID{Bytes: sessionID, Valid: true},
		UserID: userID,
		Title:  pgtype.Text{String: title, Valid: title != ""},
	})
	if err != nil {
		return uuid.Nil(), err
	}

	return id.Bytes, nil
}

var ErrInvalidRole = errors.New("invalid role")

func mapRoleToSQLC(role domain.MessageRole) (sqlc.MessageRole, error) {
	switch role {
	case domain.MessageRoleUser:
		return sqlc.MessageRoleUser, nil
	case domain.MessageRoleAssistant:
		return sqlc.MessageRoleAssistant, nil
	default:
		return sqlc.MessageRole("invalid"), ErrInvalidRole
	}
}

func (repo *SessionRepository) CreateTurn(ctx context.Context, sessionID uuid.UUID, m domain.Message) (int64, error) {
	role, err := mapRoleToSQLC(m.Role)
	if err != nil {
		return 0, err
	}

	return repo.q.CreateTurn(ctx, sqlc.CreateTurnParams{
		SessionID: pgtype.UUID{Bytes: sessionID, Valid: true},
		Role:      role,
		Content:   m.Content,
	})
}

func (repo *SessionRepository) CreateMessage(ctx context.Context, turnID int64, m domain.Message) error {
	var role sqlc.MessageRole
	switch m.Role {
	case domain.MessageRoleUser:
		role = sqlc.MessageRoleUser
	case domain.MessageRoleAssistant:
		role = sqlc.MessageRoleAssistant
	default:
		return ErrInvalidRole
	}

	if err := repo.q.CreateMessage(ctx, sqlc.CreateMessageParams{
		TurnID:  turnID,
		Role:    role,
		Content: m.Content,
	}); err != nil {
		return err
	}

	return nil
}

func (repo *SessionRepository) ListRecentMessages(ctx context.Context, sessionID uuid.UUID, limit int) ([]domain.Message, error) {
	rows, err := repo.q.ListRecentMessages(ctx, sqlc.ListRecentMessagesParams{
		SessionID: pgtype.UUID{Bytes: sessionID, Valid: true},
		TurnLimit: int32(limit),
	})
	if err != nil {
		return nil, err
	}

	messages := make([]domain.Message, 0, len(rows))
	for _, row := range rows {
		var role domain.MessageRole

		switch row.Role {
		case sqlc.MessageRoleUser:
			role = domain.MessageRoleUser
		case sqlc.MessageRoleAssistant:
			role = domain.MessageRoleAssistant
		default:
			return nil, ErrInvalidRole
		}

		messages = append(messages, domain.Message{
			ID:      row.ID,
			Role:    role,
			Content: row.Content,
		})
	}

	return messages, nil
}

func (repo *SessionRepository) List(ctx context.Context, userID int64, offset, limit int) ([]domain.Session, error) {
	rows, err := repo.q.ListSessions(ctx, sqlc.ListSessionsParams{
		UserID: userID,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, err
	}

	sessions := make([]domain.Session, 0, len(rows))
	for _, row := range rows {
		sessions = append(sessions, domain.Session{
			ID:        row.ID.Bytes,
			Title:     row.Title.String,
			CreatedAt: row.CreatedAt.Time,
		})
	}

	return sessions, nil
}

func (repo *SessionRepository) Get(ctx context.Context, sessionID uuid.UUID, userID int64) (domain.Session, error) {
	row, err := repo.q.GetSession(ctx, sqlc.GetSessionParams{
		ID:     pgtype.UUID{Bytes: sessionID, Valid: true},
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Session{}, domain.ErrSessionNotFound
		}

		return domain.Session{}, err
	}

	return domain.Session{
		ID:    sessionID,
		Title: row.Title.String,
	}, nil
}

func (repo *SessionRepository) SetTitle(ctx context.Context, sessionID uuid.UUID, userID int64, title string) error {
	return repo.q.SetSessionTitle(ctx, sqlc.SetSessionTitleParams{
		Title:  pgtype.Text{String: title, Valid: title != ""},
		ID:     pgtype.UUID{Bytes: sessionID, Valid: true},
		UserID: userID,
	})
}
