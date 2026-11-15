package handler

import (
	"log/slog"
	"net/http"
	"rag-chat/internal/http/middleware"
)

func NewRouter(ask, signUp, sessions http.Handler, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("POST /users/{userID}/sessions/{sessionID}/messages", ask)
	mux.Handle("POST /users", signUp)
	mux.Handle("/users/{userID}/sessions", sessions)

	handler := middleware.Logger(logger)(mux)
	handler = middleware.Recover(handler)

	return handler
}
