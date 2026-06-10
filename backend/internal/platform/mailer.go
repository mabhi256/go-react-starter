package platform

import (
	"context"
	"fmt"
	"net/smtp"

	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/your-org/go-react-starter/backend/internal/config"
)

// Mailer sends transactional email. Dev uses SMTP (Mailhog); prod uses Amazon SES.
type Mailer interface {
	Send(ctx context.Context, to, subject, htmlBody string) error
}

// NewMailer selects the implementation by APP_ENV.
func NewMailer(ctx context.Context, cfg config.Config) (Mailer, error) {
	if cfg.IsProd() {
		ac, err := awscfg.LoadDefaultConfig(ctx, awscfg.WithRegion(cfg.Mail.AWSRegion))
		if err != nil {
			return nil, fmt.Errorf("load aws config: %w", err)
		}
		return &tracingMailer{inner: &sesMailer{client: sesv2.NewFromConfig(ac), from: cfg.Mail.From}, backend: "ses"}, nil
	}
	return &tracingMailer{inner: &smtpMailer{addr: fmt.Sprintf("%s:%d", cfg.Mail.SMTPHost, cfg.Mail.SMTPPort), from: cfg.Mail.From}, backend: "smtp"}, nil
}

type tracingMailer struct {
	inner   Mailer
	backend string
}

func (m *tracingMailer) Send(ctx context.Context, to, subject, htmlBody string) error {
	_, span := otel.Tracer("platform").Start(ctx, "mailer.send",
		trace.WithAttributes(
			attribute.String("mailer.backend", m.backend),
			attribute.String("email.to", to),
			attribute.String("email.subject", subject),
		),
	)
	defer span.End()
	err := m.inner.Send(ctx, to, subject, htmlBody)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}

type smtpMailer struct {
	addr string
	from string
}

func (m *smtpMailer) Send(_ context.Context, to, subject, htmlBody string) error {
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		m.from, to, subject, htmlBody)
	return smtp.SendMail(m.addr, nil, m.from, []string{to}, []byte(msg))
}

type sesMailer struct {
	client *sesv2.Client
	from   string
}

func (m *sesMailer) Send(ctx context.Context, to, subject, htmlBody string) error {
	_, err := m.client.SendEmail(ctx, &sesv2.SendEmailInput{
		FromEmailAddress: &m.from,
		Destination:      &types.Destination{ToAddresses: []string{to}},
		Content: &types.EmailContent{Simple: &types.Message{
			Subject: &types.Content{Data: &subject},
			Body:    &types.Body{Html: &types.Content{Data: &htmlBody}},
		}},
	})
	return err
}

