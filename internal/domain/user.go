package domain

import (
	"errors"
	"time"
)

type User struct {
	ID         int64
	TelegramID *int64
	CreatedAt  time.Time
}

func NewUser(telegramID *int64) User {
	return User{
		TelegramID: telegramID,
	}
}

var (
	ErrUserAlreadyExists = errors.New("user already exists")
)
