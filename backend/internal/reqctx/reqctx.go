// Package reqctx carries per-request metadata (client IP, user agent) on the context so
// handlers can attach it to audit entries. Set by the server middleware.
package reqctx

import "context"

type Meta struct {
	IP        string
	UserAgent string
}

type ctxKey struct{}

// Key is exported so the Huma middleware can set Meta via huma.WithValue.
var Key = ctxKey{}

func WithMeta(ctx context.Context, m Meta) context.Context {
	return context.WithValue(ctx, Key, m)
}

func FromContext(ctx context.Context) Meta {
	m, _ := ctx.Value(Key).(Meta)
	return m
}
