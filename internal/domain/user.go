package domain

import (
	"errors"
	"time"
	"uuid"
)

type User struct {
	ID              int64
	TelegramID      *int64
	ActiveSessionID uuid.UUID
	CreatedAt       time.Time
}

func NewUser(telegramID *int64) User {
	return User{
		TelegramID: telegramID,
	}
}

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("user already exists")
)
