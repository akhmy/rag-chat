package postgres

import (
	"context"
	"errors"
	"rag-chat/internal/domain"
	"rag-chat/internal/postgres/sqlc"
)

type SessionRepository struct {
	q *sqlc.Queries
}

func NewSessionRepository(q *sqlc.Queries) *SessionRepository {
	return &SessionRepository{q: q}
}

func (repo *SessionRepository) Create(ctx context.Context, userID int64, title string) (int64, error) {
	id, err := repo.q.CreateSession(ctx, sqlc.CreateSessionParams{
		UserID: userID,
		Title:  title,
	})
	if err != nil {
		return 0, err
	}

	return id, nil
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

func (repo *SessionRepository) CreateTurn(ctx context.Context, sessionID int64, m domain.Message) (int64, error) {
	role, err := mapRoleToSQLC(m.Role)
	if err != nil {
		return 0, err
	}

	return repo.q.CreateTurn(ctx, sqlc.CreateTurnParams{
		SessionID: sessionID,
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

func (repo *SessionRepository) ListRecentMessages(ctx context.Context, sessionID int64, limit int) ([]domain.Message, error) {
	rows, err := repo.q.ListRecentMessages(ctx, sqlc.ListRecentMessagesParams{
		SessionID: sessionID,
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
