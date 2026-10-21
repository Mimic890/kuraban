package logger

import (
	"context"

	"github.com/rs/zerolog"
)

// ctxKey is unexported so other packages can't collide.
type ctxKey struct{}

// IntoCtx returns a new context carrying the given logger. Retrieve it later
// with FromCtx. Middleware that builds a per-request logger should call this
// once per request.
func IntoCtx(ctx context.Context, l zerolog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromCtx returns the logger bound to ctx. If no logger is bound (e.g. the
// call is outside any middleware, or ctx is nil), it returns the global
// logger. Never returns nil.
func FromCtx(ctx context.Context) *zerolog.Logger {
	if ctx != nil {
		if l, ok := ctx.Value(ctxKey{}).(zerolog.Logger); ok {
			return &l
		}
	}
	l := Get().Logger
	return &l
}
