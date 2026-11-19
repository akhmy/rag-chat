package domain

import (
	"errors"
	"time"
	"uuid"
)

type Session struct {
	ID        uuid.UUID
	Title     string
	CreatedAt time.Time
}

var (
	ErrSessionNotFound = errors.New("session not found")
)
