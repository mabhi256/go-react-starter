package notify

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/your-org/go-react-starter/backend/internal/platform"
)

const (
	TaskEmail = "notify:email"
	TaskSMS   = "notify:sms"
)

type EmailPayload struct {
	Topic   string `json:"topic"`
	To      string `json:"to"`
	Subject string `json:"subject"`
	HTML    string `json:"html"`
}

type SMSPayload struct {
	Topic   string `json:"topic"`
	Phone   string `json:"phone"`
	Message string `json:"message"`
}

// Sender is the interface auth.Service uses for async delivery of email and SMS.
type Sender interface {
	Email(ctx context.Context, topic, to, subject, html string) error
	SMS(ctx context.Context, topic, phone, message string) error
}

// AsynqSender enqueues notify tasks onto the default queue.
type AsynqSender struct {
	client *asynq.Client
}

func NewAsynqSender(client *asynq.Client) *AsynqSender {
	return &AsynqSender{client: client}
}

func (s *AsynqSender) Email(ctx context.Context, topic, to, subject, html string) error {
	payload, err := json.Marshal(EmailPayload{Topic: topic, To: to, Subject: subject, HTML: html})
	if err != nil {
		return fmt.Errorf("notify: marshal email payload: %w", err)
	}
	wrapped, err := platform.WrapTaskPayload(ctx, payload)
	if err != nil {
		return fmt.Errorf("notify: wrap email payload: %w", err)
	}
	ctx, span := otel.Tracer("notify").Start(ctx, "asynq.enqueue",
		trace.WithAttributes(attribute.String("task.type", TaskEmail)),
	)
	defer span.End()
	_, err = s.client.EnqueueContext(ctx, asynq.NewTask(TaskEmail, wrapped), asynq.Queue("default"), asynq.MaxRetry(3))
	return err
}

func (s *AsynqSender) SMS(ctx context.Context, topic, phone, message string) error {
	payload, err := json.Marshal(SMSPayload{Topic: topic, Phone: phone, Message: message})
	if err != nil {
		return fmt.Errorf("notify: marshal sms payload: %w", err)
	}
	wrapped, err := platform.WrapTaskPayload(ctx, payload)
	if err != nil {
		return fmt.Errorf("notify: wrap sms payload: %w", err)
	}
	ctx, span := otel.Tracer("notify").Start(ctx, "asynq.enqueue",
		trace.WithAttributes(attribute.String("task.type", TaskSMS)),
	)
	defer span.End()
	_, err = s.client.EnqueueContext(ctx, asynq.NewTask(TaskSMS, wrapped), asynq.Queue("default"), asynq.MaxRetry(3))
	return err
}

// HandleEmail returns an asynq handler that delivers email via mailer.
func HandleEmail(mailer platform.Mailer, logger zerolog.Logger) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var p EmailPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			logger.Error().Err(err).Msg("notify: bad email task payload")
			return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
		}
		trace.SpanFromContext(ctx).SetAttributes(
			attribute.String("email.to", p.To),
			attribute.String("email.subject", p.Subject),
		)
		return mailer.Send(ctx, p.To, p.Subject, p.HTML)
	}
}

// HandleSMS returns an asynq handler that delivers SMS via sms sender.
func HandleSMS(sms platform.SMSSender, logger zerolog.Logger) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var p SMSPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			logger.Error().Err(err).Msg("notify: bad sms task payload")
			return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
		}
		trace.SpanFromContext(ctx).SetAttributes(
			attribute.String("sms.phone", p.Phone),
		)
		return sms.Send(ctx, p.Phone, p.Message)
	}
}

