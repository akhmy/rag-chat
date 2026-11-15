package appctx

import (
	"context"
	"log/slog"
)

var discardLogger = slog.New(slog.DiscardHandler)

func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

func Logger(ctx context.Context) *slog.Logger {
	logger, ok := ctx.Value(loggerKey).(*slog.Logger)
	if !ok {
		return discardLogger
	}

	return logger
}
