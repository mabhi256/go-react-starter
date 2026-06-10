package server

import (
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/trace"
)

// RequestLogger returns an Echo middleware that emits one structured log line per
// request: method, path, status, latency, remote IP, and request ID.
// Because the logger is backed by the OTEL log provider (when configured), these
// lines also appear in Loki.
func RequestLogger(log zerolog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)

			req := c.Request()
			res := c.Response()

			status := res.Status
			if he, ok := err.(*echo.HTTPError); ok {
				status = he.Code
			}

			ev := log.Info()
			if status >= 500 {
				ev = log.Error()
			} else if status >= 400 {
				ev = log.Warn()
			}

			sc := trace.SpanFromContext(req.Context()).SpanContext()
			e := ev.
				Str("method", req.Method).
				Str("path", req.URL.Path).
				Int("status", status).
				Str("latency", time.Since(start).String()).
				Str("ip", c.RealIP()).
				Str("request_id", res.Header().Get(echo.HeaderXRequestID))
			if sc.IsValid() {
				e = e.Str("trace_id", sc.TraceID().String()).Str("span_id", sc.SpanID().String())
			}
			e.Msg("request")

			return err
		}
	}
}
