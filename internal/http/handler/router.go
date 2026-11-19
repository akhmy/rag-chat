package handler

import (
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	"rag-chat/internal/http/middleware"
)

func NewRouter(webhookPath string, webhook http.Handler) (http.Handler, error) {
	if !strings.HasPrefix(webhookPath, "/") || webhookPath == "/" ||
		path.Clean(webhookPath) != webhookPath || strings.ContainsAny(webhookPath, "{}") {
		return nil, fmt.Errorf("TELEGRAM_WEBHOOK_PATH must be a literal absolute path without a trailing slash")
	}
	uri, err := url.ParseRequestURI(webhookPath)
	if err != nil || uri.ForceQuery || uri.RawQuery != "" || uri.EscapedPath() != webhookPath {
		return nil, fmt.Errorf("TELEGRAM_WEBHOOK_PATH must be a URL path")
	}

	mux := http.NewServeMux()
	mux.Handle("POST "+webhookPath, webhook)
	return middleware.Recover(mux), nil
}
