package telegram

import (
	"context"
	"errors"
	"log/slog"
	"rag-chat/internal/appctx"
	"rag-chat/internal/domain"
	"runtime/debug"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func replyError(
	ctx context.Context,
	sdk *bot.Bot,
	chatID int64,
	text string,
) {
	if _, err := sdk.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	}); err != nil {
		appctx.Logger(ctx).Error("failed to send error reply", "error", err)
	}
}

func Recover(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, upd *models.Update) {
		defer func() {
			if rec := recover(); rec != nil {
				if upd.Message != nil {
					replyError(ctx, b, upd.Message.Chat.ID, "unknown error")
				}
			}
		}()

		next(ctx, b, upd)
	}
}

const (
	updateTypeMessage       = "message"
	updateTypeCallbackQuery = "callback_query"
)

func updateType(upd *models.Update) string {
	switch {
	case upd.Message != nil:
		return updateTypeMessage
	case upd.CallbackQuery != nil:
		return updateTypeCallbackQuery
	default:
		return "other"
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

func ResolveUser(userSvc UserService) bot.Middleware {
	return func(next bot.HandlerFunc) bot.HandlerFunc {
		return func(ctx context.Context, b *bot.Bot, upd *models.Update) {
			logger := appctx.Logger(ctx)

			var telegramID int64
			switch updateType(upd) {
			case updateTypeMessage:
				telegramID = upd.Message.Chat.ID
			case updateTypeCallbackQuery:
				telegramID = upd.CallbackQuery.From.ID
			default:
				panic("unknown update type")
			}

			user, err := userSvc.GetUserByTelegramID(ctx, telegramID)
			if err != nil {
				if errors.Is(err, domain.ErrUserNotFound) {
					user, err = userSvc.CreateUser(ctx, &telegramID)
					if err != nil {
						logger.Error("failed to create user", "error", err)

						if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
							ChatID: telegramID,
							Text:   "error",
						}); err != nil {
							logger.Error("failed to send message", "error", err)
						}

						return
					}
				}
			}

			next(appctx.WithUser(ctx, &user), b, upd)
		}
	}
}
