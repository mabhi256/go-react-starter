package server

import (
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/your-org/go-react-starter/backend/internal/auth"
	"github.com/your-org/go-react-starter/backend/internal/rbac"
	"github.com/your-org/go-react-starter/backend/internal/reqctx"
)

// authMiddleware validates the bearer access token when present and stores the resulting
// identity + request metadata on the context. Operations that declare the "bearer" security
// requirement are rejected with 401 unless a valid identity was established.
func authMiddleware(api huma.API, tokens *auth.TokenService) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		ctx = huma.WithValue(ctx, reqctx.Key, reqctx.Meta{
			IP:        ctx.RemoteAddr(),
			UserAgent: ctx.Header("User-Agent"),
		})

		needsAuth := false
		for _, req := range ctx.Operation().Security {
			if _, ok := req["bearer"]; ok {
				needsAuth = true
				break
			}
		}

		if raw := bearerToken(ctx.Header("Authorization")); raw != "" {
			if id, err := tokens.ParseAccess(raw); err == nil {
				ctx = huma.WithValue(ctx, rbac.ContextKey, id)
			} else if needsAuth {
				_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "invalid or expired token")
				return
			}
		}

		if needsAuth {
			if _, ok := rbac.FromContext(ctx.Context()); !ok {
				_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "authentication required")
				return
			}
		}
		next(ctx)
	}
}

func bearerToken(header string) string {
	const prefix = "Bearer "
	if strings.HasPrefix(header, prefix) {
		return strings.TrimSpace(header[len(prefix):])
	}
	return ""
}

