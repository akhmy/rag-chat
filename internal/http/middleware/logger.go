package middleware

import (
	"log/slog"
	"net/http"
	"rag-chat/internal/appctx"
	"time"
)

type wrappedResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *wrappedResponseWriter) WriteHeader(statusCode int) {
	if w.statusCode != 0 {
		return
	}

	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *wrappedResponseWriter) Write(b []byte) (int, error) {
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}

	return w.ResponseWriter.Write(b)
}

func Logger(base *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger := base.With(
				"url", r.URL.Path,
				"method", r.Method,
			)

			logger.Info("received request")

			ww := &wrappedResponseWriter{ResponseWriter: w}
			ctx := appctx.WithLogger(r.Context(), logger)
			start := time.Now()

			defer func() {
				if rec := recover(); rec != nil {
					if err, ok := rec.(error); ok {
						logger.With(
							"duration", time.Since(start),
							"status", ww.statusCode,
						).Error("recovered from panic", "error", err)
					}

					logger.With(
						"duration", time.Since(start),
						"status", ww.statusCode,
					).Error("recovered from panic")

					panic(rec)
				}
			}()

			next.ServeHTTP(ww, r.WithContext(ctx))

			logger.With(
				"duration", time.Since(start),
				"status", ww.statusCode,
			).Info("completed request")
		})
	}
}
