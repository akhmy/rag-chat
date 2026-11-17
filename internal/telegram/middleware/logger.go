package middleware

import (
	"context"
	"log/slog"
	"rag-chat/internal/appctx"
	"runtime/debug"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func updateType(upd *models.Update) string {
	switch {
	case upd.Message != nil:
		return "message"
	case upd.CallbackQuery != nil:
		return "callback_query"
	default:
		return "unknown"
	}
}

func Logger(logger *slog.Logger) bot.Middleware {
	return func(next bot.HandlerFunc) bot.HandlerFunc {
		return func(ctx context.Context, b *bot.Bot, upd *models.Update) {
			l := logger.With(
				"update_id", upd.ID,
				"update_type", updateType(upd),
			)

			l.Info("received update")

			start := time.Now()

			defer func() {
				if rec := recover(); rec != nil {
					if err, ok := rec.(error); ok {
						l.Error("panic",
							"duration", time.Since(start),
							"stack", string(debug.Stack()),
							"error", err,
						)
					} else {
						l.Error("panic",
							"stack", string(debug.Stack()),
							"duration", time.Since(start))
					}

					panic(rec)
				}
			}()

			next(appctx.WithLogger(ctx, l), b, upd)

			l.Info("processed update", "duration", time.Since(start))
		}
	}
}
