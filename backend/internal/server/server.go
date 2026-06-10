// Package server builds the Echo router and the Huma API (OpenAPI 3.1 + Scalar docs) and
// installs cross-cutting middleware. Domain packages register their operations onto the
// returned huma.API; this package imports none of them.
package server

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humaecho"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"

	"github.com/your-org/go-react-starter/backend/internal/auth"
	"github.com/your-org/go-react-starter/backend/internal/config"
)

// New constructs the Echo router and Huma API. Operations are registered by the caller.
func New(cfg config.Config, tokens *auth.TokenService, log zerolog.Logger) (*echo.Echo, huma.API) {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(otelecho.Middleware(cfg.OTEL.ServiceName))
	e.Use(RequestLogger(log))
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: cfg.HTTP.CORSOrigins,
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowHeaders: []string{echo.HeaderAuthorization, echo.HeaderContentType},
	}))

	e.GET("/healthz", func(c echo.Context) error { return c.JSON(http.StatusOK, map[string]string{"status": "ok"}) })
	if !cfg.IsProd() {
		e.GET("/docs", scalarDocs)
	}

	humaCfg := huma.DefaultConfig("go-react-starter API", "1.0.0")
	// humaCfg.Transformers = nil   // remove $schema injection
	humaCfg.Info.Description = "Multi-tenant Go + React starter with auth, RBAC, audit trail, and asynq."
	if cfg.IsProd() {
		humaCfg.OpenAPIPath = ""
	} else {
		humaCfg.OpenAPIPath = "/openapi"
	}
	humaCfg.DocsPath = "" // Scalar is served from /docs above instead of the built-in renderer.
	if humaCfg.Components.SecuritySchemes == nil {
		humaCfg.Components.SecuritySchemes = map[string]*huma.SecurityScheme{}
	}
	humaCfg.Components.SecuritySchemes["bearer"] = &huma.SecurityScheme{
		Type:         "http",
		Scheme:       "bearer",
		BearerFormat: "JWT",
	}

	api := humaecho.New(e, humaCfg)
	api.UseMiddleware(authMiddleware(api, tokens))
	return e, api
}

// scalarDocs renders the Scalar API reference pointed at the generated OpenAPI document.
func scalarDocs(c echo.Context) error {
	const page = `<!doctype html>
<html>
  <head>
    <title>go-react-starter API</title>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
  </head>
  <body>
    <script id="api-reference" data-url="/openapi.yaml"></script>
    <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
  </body>
</html>`
	return c.HTML(http.StatusOK, page)
}

