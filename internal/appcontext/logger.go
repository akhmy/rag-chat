package appcontext

import "context"

type AppLogger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

type NopLogger struct{}

func (NopLogger) Debug(msg string, args ...any) {}
func (NopLogger) Info(msg string, args ...any)  {}
func (NopLogger) Warn(msg string, args ...any)  {}
func (NopLogger) Error(msg string, args ...any) {}

func WithLogger(ctx context.Context, logger AppLogger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

func Logger(ctx context.Context) AppLogger {
	logger, ok := ctx.Value(loggerKey).(AppLogger)
	if !ok {
		return NopLogger{}
	}

	return logger
}
