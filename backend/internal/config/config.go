// Package config loads runtime configuration via koanf.
//
// Precedence (low -> high): built-in defaults -> .env file -> process env vars.
// Keys are flat UPPER_SNAKE (e.g. HTTP_PORT) matching .env.example. A single APP_ENV
// (dev|prod) selects provider implementations elsewhere; there are no other code branches.
package config

import (
	"fmt"
	"time"

	"github.com/knadh/koanf/parsers/dotenv"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

type Env string

const (
	EnvDev  Env = "dev"
	EnvProd Env = "prod"
)

type Config struct {
	AppEnv Env

	HTTP struct {
		Host        string
		Port        int
		CORSOrigins []string
	}

	DB struct {
		DSN string
	}

	Redis struct {
		Addr     string
		Password string
		DB       int
	}

	JWT struct {
		Secret     string
		AccessTTL  time.Duration
		RefreshTTL time.Duration
		Issuer     string
	}

	Google struct {
		ClientID string
	}

	Mail struct {
		From      string
		SMTPHost  string
		SMTPPort  int
		AWSRegion string
		AppURL    string
	}

	SMS struct {
		Provider       string
		OTPTTL         time.Duration
		OTPMaxAttempts int
		MSG91AuthKey   string
		MSG91SenderID  string
	}

	OTEL struct {
		Endpoint    string
		ServiceName string
	}

	Pyroscope struct {
		URL string
	}

	SuperAdmin struct {
		Email    string
		Password string
	}
}

func (c Config) IsProd() bool { return c.AppEnv == EnvProd }

// Load reads defaults, then .env (if present), then env vars.
func Load(dotenvPath string) (Config, error) {
	k := koanf.New(".")

	_ = k.Load(file.Provider(dotenvPath), dotenv.Parser()) // optional; ignore if missing
	if err := k.Load(env.Provider("", ".", func(s string) string { return s }), nil); err != nil {
		return Config{}, fmt.Errorf("load env: %w", err)
	}

	get := func(key, def string) string {
		if v := k.String(key); v != "" {
			return v
		}
		return def
	}
	dur := func(key, def string) (time.Duration, error) {
		d, err := time.ParseDuration(get(key, def))
		if err != nil {
			return 0, fmt.Errorf("invalid duration %s: %w", key, err)
		}
		return d, nil
	}

	var c Config
	c.AppEnv = Env(get("APP_ENV", "dev"))

	c.HTTP.Host = get("HTTP_HOST", "0.0.0.0")
	c.HTTP.Port = atoi(get("HTTP_PORT", "8080"))
	c.HTTP.CORSOrigins = splitCSV(get("HTTP_CORS_ORIGINS", "http://localhost:3000"))

	c.DB.DSN = get("DB_DSN", "")

	c.Redis.Addr = get("REDIS_ADDR", "localhost:6379")
	c.Redis.Password = get("REDIS_PASSWORD", "")
	c.Redis.DB = atoi(get("REDIS_DB", "0"))

	c.JWT.Secret = get("JWT_SECRET", "")
	c.JWT.Issuer = get("JWT_ISSUER", "go-react-starter")
	var err error
	if c.JWT.AccessTTL, err = dur("JWT_ACCESS_TTL", "15m"); err != nil {
		return Config{}, err
	}
	if c.JWT.RefreshTTL, err = dur("JWT_REFRESH_TTL", "720h"); err != nil {
		return Config{}, err
	}

	c.Google.ClientID = get("GOOGLE_OAUTH_CLIENT_ID", "")

	c.Mail.From = get("MAIL_FROM", "no-reply@example.local")
	c.Mail.SMTPHost = get("MAIL_SMTP_HOST", "localhost")
	c.Mail.SMTPPort = atoi(get("MAIL_SMTP_PORT", "1025"))
	c.Mail.AWSRegion = get("AWS_REGION", "ap-south-1")
	c.Mail.AppURL = get("MAIL_APP_URL", "http://localhost:3000")

	c.SMS.Provider = get("SMS_PROVIDER", "log")
	if c.SMS.OTPTTL, err = dur("OTP_TTL", "5m"); err != nil {
		return Config{}, err
	}
	c.SMS.OTPMaxAttempts = atoi(get("OTP_MAX_ATTEMPTS", "5"))
	c.SMS.MSG91AuthKey = get("MSG91_AUTH_KEY", "")
	c.SMS.MSG91SenderID = get("MSG91_SENDER_ID", "STARTER")

	c.OTEL.Endpoint = get("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	c.OTEL.ServiceName = get("OTEL_SERVICE_NAME", "go-react-starter-api")

	c.Pyroscope.URL = get("PYROSCOPE_URL", "")

	c.SuperAdmin.Email = get("SUPER_ADMIN_EMAIL", "")
	c.SuperAdmin.Password = get("SUPER_ADMIN_PASSWORD", "")

	return c, c.validate()
}

func (c Config) validate() error {
	if c.DB.DSN == "" {
		return fmt.Errorf("DB_DSN is required")
	}
	if c.JWT.Secret == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}
	if c.IsProd() && c.JWT.Secret == "dev-only-change-me-in-prod" {
		return fmt.Errorf("JWT_SECRET must be changed in prod")
	}
	return nil
}
