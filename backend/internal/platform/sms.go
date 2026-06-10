package platform

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/your-org/go-react-starter/backend/internal/config"
)

// SMSSender delivers OTP / notification SMS. Dev logs the message; prod plugs a provider.
type SMSSender interface {
	Send(ctx context.Context, phone, message string) error
}

// NewSMSSender selects an implementation by SMS_PROVIDER.
func NewSMSSender(cfg config.Config, logger zerolog.Logger) (SMSSender, error) {
	var inner SMSSender
	var backend string
	switch cfg.SMS.Provider {
	case "log", "":
		inner, backend = &logSMS{logger: logger}, "log"
	case "msg91":
		if cfg.SMS.MSG91AuthKey == "" {
			return nil, fmt.Errorf("MSG91_AUTH_KEY is required when SMS_PROVIDER=msg91")
		}
		inner, backend = &msg91SMS{authKey: cfg.SMS.MSG91AuthKey, senderID: cfg.SMS.MSG91SenderID}, "msg91"
	default:
		return nil, fmt.Errorf("SMS_PROVIDER %q not implemented (extension point in platform/sms.go)", cfg.SMS.Provider)
	}
	return &tracingSMSSender{inner: inner, backend: backend}, nil
}

type tracingSMSSender struct {
	inner   SMSSender
	backend string
}

func (s *tracingSMSSender) Send(ctx context.Context, phone, message string) error {
	_, span := otel.Tracer("platform").Start(ctx, "sms.send",
		trace.WithAttributes(
			attribute.String("sms.backend", s.backend),
			attribute.String("sms.phone", phone),
		),
	)
	defer span.End()
	err := s.inner.Send(ctx, phone, message)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}

type logSMS struct {
	logger zerolog.Logger
}

func (s *logSMS) Send(_ context.Context, phone, message string) error {
	s.logger.Info().Str("to", phone).Str("sms", message).Msg("SMS (dev log sender)")
	return nil
}

type msg91SMS struct {
	authKey  string
	senderID string
}

func (s *msg91SMS) Send(ctx context.Context, phone, message string) error {
	mobile := strings.TrimPrefix(phone, "+")
	params := url.Values{
		"authkey": {s.authKey},
		"mobiles": {mobile},
		"message": {message},
		"sender":  {s.senderID},
		"route":   {"4"},
		"country": {"91"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.msg91.com/api/sendhttp.php?"+params.Encode(), nil)
	if err != nil {
		return fmt.Errorf("msg91: build request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("msg91: send: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("msg91: unexpected status %d: %s", resp.StatusCode, body)
	}
	return nil
}

