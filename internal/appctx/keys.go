package appctx

type contextKey int

const (
	loggerKey contextKey = iota
	userKey
)
