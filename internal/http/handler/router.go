package handler

import (
	"log/slog"
	"net/http"
	"rag-chat/internal/http/middleware"
)

func NewRouter(ask http.Handler, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("POST /ask", ask)

	handler := middleware.Logger(logger)(mux)
	handler = middleware.Recover(handler)

	return handler
}
