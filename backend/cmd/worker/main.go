// Command worker runs the asynq background task processor.
package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"

	"github.com/your-org/go-react-starter/backend/internal/audit"
	"github.com/your-org/go-react-starter/backend/internal/config"
	"github.com/your-org/go-react-starter/backend/internal/notify"
	"github.com/your-org/go-react-starter/backend/internal/platform"
)

func main() {
	cfg, err := config.Load(".env")
	logger := platform.NewLogger(cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("load config")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	otelShutdown, err := platform.NewOTEL(ctx, cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("init otel")
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = otelShutdown(ctx)
	}()

	stopPyroscope, err := platform.NewPyroscope(cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("init pyroscope")
	}
	defer stopPyroscope()

	db, err := platform.NewDB(ctx, cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("connect db")
	}
	defer db.Close()

	mailer, err := platform.NewMailer(ctx, cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("init mailer")
	}

	smsSender, err := platform.NewSMSSender(cfg, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("init sms sender")
	}

	srv := asynq.NewServer(platform.AsynqRedisOpt(cfg), asynq.Config{
		Concurrency: 10,
		// Default is 1s, giving up to ~1.5s queue latency. 100ms brings it to ~150ms.
		// Doesn't matter much for email/SMS but cheap enough on Valkey/Elasticache.
		TaskCheckInterval: 100 * time.Millisecond,
		Queues: map[string]int{
			"audit":   5,
			"default": 5,
		},
	})

	if err := registerQueueGauge(cfg, logger); err != nil {
		logger.Fatal().Err(err).Msg("register queue gauge")
	}

	auditRepo := audit.NewRepo(db)
	mux := asynq.NewServeMux()
	mux.Use(taskSpanMiddleware(logger))
	mux.Handle(audit.TaskWrite, audit.HandleWrite(auditRepo, logger))
	mux.Handle(notify.TaskEmail, notify.HandleEmail(mailer, logger))
	mux.Handle(notify.TaskSMS, notify.HandleSMS(smsSender, logger))

	logger.Info().Msg("starting worker")
	if err := srv.Run(mux); err != nil {
		logger.Fatal().Err(err).Msg("worker error")
	}
}

func taskSpanMiddleware(logger zerolog.Logger) func(asynq.Handler) asynq.Handler {
	return func(next asynq.Handler) asynq.Handler {
		return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
			start := time.Now()
			ctx, payload := platform.UnwrapTaskPayload(ctx, t.Payload())
			ctx, span := otel.Tracer("worker").Start(ctx, t.Type())
			defer span.End()
			err := next.ProcessTask(ctx, asynq.NewTask(t.Type(), payload))
			elapsed := time.Since(start)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				logger.Error().Err(err).Str("task", t.Type()).Dur("elapsed", elapsed).Msg("task failed")
				return err
			}
			logTaskDone(logger, t.Type(), payload, elapsed)
			return nil
		})
	}
}

func registerQueueGauge(cfg config.Config, logger zerolog.Logger) error {
	inspector := asynq.NewInspector(platform.AsynqRedisOpt(cfg))
	meter := otel.GetMeterProvider().Meter("worker")
	gauge, err := meter.Int64ObservableGauge("asynq_queue_depth",
		metric.WithDescription("Number of tasks in each queue state (active/pending/retry/archived)"),
	)
	if err != nil {
		return err
	}
	_, err = meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		queues, err := inspector.Queues()
		if err != nil {
			logger.Warn().Err(err).Msg("queue gauge: list queues")
			return nil
		}
		for _, q := range queues {
			info, err := inspector.GetQueueInfo(q)
			if err != nil {
				continue
			}
			attrs := func(state string) metric.ObserveOption {
				return metric.WithAttributes(
					attribute.String("queue", q),
					attribute.String("state", state),
				)
			}
			o.ObserveInt64(gauge, int64(info.Active), attrs("active"))
			o.ObserveInt64(gauge, int64(info.Pending), attrs("pending"))
			o.ObserveInt64(gauge, int64(info.Retry), attrs("retry"))
			o.ObserveInt64(gauge, int64(info.Archived), attrs("archived"))
		}
		return nil
	}, gauge)
	return err
}

func logTaskDone(logger zerolog.Logger, taskType string, payload []byte, elapsed time.Duration) {
	switch taskType {
	case notify.TaskEmail:
		var p notify.EmailPayload
		if err := json.Unmarshal(payload, &p); err == nil {
			logger.Info().Str("task", taskType).Str("topic", p.Topic).Str("to", p.To).Str("subject", p.Subject).Dur("elapsed", elapsed).Msg("sent email")
			return
		}
	case notify.TaskSMS:
		var p notify.SMSPayload
		if err := json.Unmarshal(payload, &p); err == nil {
			logger.Info().Str("task", taskType).Str("topic", p.Topic).Str("phone", p.Phone).Dur("elapsed", elapsed).Msg("sent sms")
			return
		}
	case audit.TaskWrite:
		var e audit.Entry
		if err := json.Unmarshal(payload, &e); err == nil {
			logger.Info().Str("task", taskType).Str("actor_id", e.ActorID.String()).Str("action", e.Action).Str("resource_type", e.ResourceType).Str("resource_id", e.ResourceID).Dur("elapsed", elapsed).Msg("audit entry written")
			return
		}
	}
	logger.Info().Str("task", taskType).Dur("elapsed", elapsed).Msg("task done")
}

