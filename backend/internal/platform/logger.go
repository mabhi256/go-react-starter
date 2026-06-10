// Package platform wires external infrastructure: logging, db, redis, jobs, otel,
// and the pluggable mailer/SMS providers. It depends on config and nothing in the
// domain layer.
package platform

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"time"

	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
	"github.com/rs/zerolog"

	"github.com/your-org/go-react-starter/backend/internal/config"
)

// NewLogger returns a zerolog logger: pretty console in dev, JSON to stdout in prod.
// When an OTEL endpoint is configured, logs are also forwarded via OTLP to Loki.
// The OTEL writer is lazy; it queries the global provider on each write, so it
// works safely even when called before NewOTEL (pre-OTEL writes go to the no-op provider).
func NewLogger(cfg config.Config) zerolog.Logger {
	var primary io.Writer
	level := zerolog.DebugLevel
	if cfg.IsProd() {
		primary = os.Stdout
		level = zerolog.InfoLevel
	} else {
		primary = zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05"}
	}

	var w io.Writer = primary
	if cfg.OTEL.Endpoint != "" {
		w = zerolog.MultiLevelWriter(primary, &otelLogWriter{})
	}

	base := zerolog.New(w).With().Timestamp()
	if cfg.IsProd() {
		base = base.Str("service", cfg.OTEL.ServiceName)
	}
	return base.Logger().Level(level)
}

// otelLogWriter bridges zerolog JSON output to the OTEL log API so all application
// logs are shipped to Loki via OTLP. It implements zerolog.LevelWriter.
type otelLogWriter struct{}

func (w *otelLogWriter) Write(p []byte) (n int, err error) {
	return w.WriteLevel(zerolog.NoLevel, p)
}

// WriteLevel parses the zerolog JSON payload and emits a structured OTEL log record.
// Each JSON field becomes an OTEL attribute (accessible as Loki structured metadata),
// and the "message" field becomes the log body, so Loki stores a readable string
// rather than a raw JSON blob. Falls back to raw body on parse failure.
func (w *otelLogWriter) WriteLevel(level zerolog.Level, p []byte) (n int, err error) {
	var rec otellog.Record
	rec.SetTimestamp(time.Now())
	rec.SetSeverity(zerologToOTELSeverity(level))

	var fields map[string]any
	if jsonErr := json.Unmarshal(p, &fields); jsonErr != nil {
		rec.SetBody(otellog.StringValue(string(p)))
		global.Logger("app").Emit(context.Background(), rec)
		return len(p), nil
	}

	if msg, ok := fields["message"].(string); ok {
		rec.SetBody(otellog.StringValue(msg))
	}

	attrs := make([]otellog.KeyValue, 0, len(fields))
	for k, v := range fields {
		if k == "time" {
			continue
		}
		switch val := v.(type) {
		case string:
			attrs = append(attrs, otellog.String(k, val))
		case float64:
			attrs = append(attrs, otellog.Float64(k, val))
		case bool:
			attrs = append(attrs, otellog.Bool(k, val))
		}
	}
	rec.AddAttributes(attrs...)

	global.Logger("app").Emit(context.Background(), rec)
	return len(p), nil
}

func zerologToOTELSeverity(l zerolog.Level) otellog.Severity {
	switch l {
	case zerolog.TraceLevel:
		return otellog.SeverityTrace
	case zerolog.DebugLevel:
		return otellog.SeverityDebug
	case zerolog.InfoLevel:
		return otellog.SeverityInfo
	case zerolog.WarnLevel:
		return otellog.SeverityWarn
	case zerolog.ErrorLevel:
		return otellog.SeverityError
	case zerolog.FatalLevel, zerolog.PanicLevel:
		return otellog.SeverityFatal
	default:
		return otellog.SeverityInfo
	}
}

