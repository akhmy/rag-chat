package postgres

import (
	"context"
	"errors"
	"rag-chat/internal/domain"
	"rag-chat/internal/postgres/sqlc"
	"time"
	"uuid"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type UserRepository struct {
	q *sqlc.Queries
}

func NewUserRepository(q *sqlc.Queries) *UserRepository {
	return &UserRepository{q: q}
}

func (repo *UserRepository) Create(ctx context.Context, u domain.User) (domain.User, error) {
	var arg pgtype.Int8

	if u.TelegramID == nil {
		arg = pgtype.Int8{Valid: false}
	} else {
		arg = pgtype.Int8{Int64: *u.TelegramID, Valid: true}
	}

	sqlcUser, err := repo.q.CreateUser(ctx, arg)
	if err != nil {
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
			switch pgErr.Code {
			case pgerrcode.UniqueViolation:
				return domain.User{}, domain.ErrUserAlreadyExists
			}
		}

		return domain.User{}, err
	}

	user := domain.User{
		ID:        sqlcUser.ID,
		CreatedAt: sqlcUser.CreatedAt.Time,
	}

	if sqlcUser.TelegramID.Valid {
		telegramID := sqlcUser.TelegramID.Int64
		user.TelegramID = &telegramID
	}

	return user, nil
}

func (repo *UserRepository) GetByTelegramID(ctx context.Context, telegramID int64) (domain.User, error) {
	user, err := repo.q.GetUserByTelegramID(ctx, pgtype.Int8{Int64: telegramID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, domain.ErrUserNotFound
		}

		return domain.User{}, err
	}

	var activeSessionID uuid.UUID
	if user.ActiveSessionID.Valid {
		value := user.ActiveSessionID.Bytes
		activeSessionID = value
	} else {
		activeSessionID = uuid.Nil()
	}

	return domain.User{
		ID:              user.ID,
		TelegramID:      &telegramID,
		ActiveSessionID: activeSessionID,
		CreatedAt:       user.CreatedAt.Time,
	}, nil
}

func (repo *UserRepository) SetActiveSession(ctx context.Context, userID int64, sessionID uuid.UUID) error {
	n, err := repo.q.SetActiveSession(ctx, sqlc.SetActiveSessionParams{
		UserID:    userID,
		SessionID: pgtype.UUID{Bytes: sessionID, Valid: true},
	})
	if err != nil {
		return err
	}

	if n == 0 {
		return domain.ErrSessionNotFound
	}

	return nil
}

func (repo *UserRepository) GetActiveSession(ctx context.Context, userID int64) (*domain.Session, error) {
	row, err := repo.q.GetActiveSession(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return &domain.Session{
		ID:    row.ID.Bytes,
		Title: row.Title.String,
	}, nil
}

func (repo *UserRepository) AddTokenUsage(ctx context.Context, userID int64, used int64) error {
	return repo.q.AddTokenUsage(ctx, sqlc.AddTokenUsageParams{
		UserID: userID,
		Tokens: used,
	})
}

func (repo *UserRepository) GetTokenUsage(ctx context.Context, userID int64, since time.Time) (int64, error) {
	return repo.q.GetTokenUsage(ctx, sqlc.GetTokenUsageParams{
		UserID: userID,
		UsedAt: pgtype.Timestamptz{Time: since, Valid: true},
	})
}
