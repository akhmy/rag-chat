package postgres

import (
	"context"
	"errors"
	"rag-chat/internal/domain"
	"rag-chat/internal/postgres/sqlc"

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

	var activeSessionID *int64
	if user.ActiveSessionID.Valid {
		value := user.ActiveSessionID.Int64
		activeSessionID = &value
	} else {
		activeSessionID = nil
	}

	return domain.User{
		ID:              user.ID,
		TelegramID:      &telegramID,
		ActiveSessionID: activeSessionID,
		CreatedAt:       user.CreatedAt.Time,
	}, nil
}

func (repo *UserRepository) SetActiveSession(ctx context.Context, userID int64, sessionID int64) error {
	return repo.q.SetActiveSession(ctx, sqlc.SetActiveSessionParams{
		ID:              userID,
		ActiveSessionID: pgtype.Int8{Int64: sessionID, Valid: true},
	})
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
		ID:    row.ID,
		Title: row.Title.String,
	}, nil
}
