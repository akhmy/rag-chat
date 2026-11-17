package domain

import (
	"errors"
	"time"
)

type Session struct {
	ID        int64
	Title     string
	CreatedAt time.Time
}

var (
	ErrSessionNotFound = errors.New("session not found")
)
