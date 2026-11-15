package postgres

import (
	"context"
	"errors"
	"rag-chat/internal/domain"
	"rag-chat/internal/postgres/sqlc"

	"github.com/jackc/pgerrcode"
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
