package domain

import "errors"

type Session struct {
	ID    int64
	Title string
}

var (
	ErrSessionNotFound = errors.New("session not found")
)
