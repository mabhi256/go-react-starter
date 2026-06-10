package auth

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/api/idtoken"

	"github.com/your-org/go-react-starter/backend/internal/config"
	"github.com/your-org/go-react-starter/backend/internal/notify"
	"github.com/your-org/go-react-starter/backend/internal/rbac"
	"github.com/your-org/go-react-starter/backend/internal/user"
)

const pwresetTTL = time.Hour

func pwresetKey(token string) string { return "pwreset:" + token }

// ErrUnauthorized is returned for any failed credential / OTP check. The message is kept
// deliberately vague so callers can't probe which accounts exist.
var ErrUnauthorized = errors.New("invalid credentials")

// TokenPair is the result of a successful login.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int // access token lifetime, seconds
}

type Service struct {
	users  *user.Repo
	tokens *TokenService
	notify notify.Sender
	redis  *redis.Client
	cfg    config.Config
}

func NewService(users *user.Repo, tokens *TokenService, notifySender notify.Sender, rdb *redis.Client, cfg config.Config) *Service {
	return &Service{users: users, tokens: tokens, notify: notifySender, redis: rdb, cfg: cfg}
}

func (s *Service) LoginPassword(ctx context.Context, email, password string) (TokenPair, error) {
	userID, hash, err := s.users.GetAuthByEmail(ctx, email)
	if errors.Is(err, user.ErrNotFound) || hash == nil {
		return TokenPair{}, ErrUnauthorized
	}
	if err != nil {
		return TokenPair{}, err
	}
	_, span := otel.Tracer("auth").Start(ctx, "bcrypt.verify")
	err = bcrypt.CompareHashAndPassword([]byte(*hash), []byte(password))
	span.End()
	if err != nil {
		return TokenPair{}, ErrUnauthorized
	}
	return s.issueFor(ctx, userID)
}

// LoginGoogle verifies a Google ID token and logs in an existing account. New users are not
// auto-provisioned (they have no org); admins create accounts; Google is only a login method.
// On first Google sign-in for a known email, the google identity is linked to that account.
func (s *Service) LoginGoogle(ctx context.Context, rawIDToken string) (TokenPair, error) {
	if s.cfg.Google.ClientID == "" {
		return TokenPair{}, fmt.Errorf("google login not configured")
	}
	_, span := otel.Tracer("auth").Start(ctx, "google.idtoken.validate")
	payload, err := idtoken.Validate(ctx, rawIDToken, s.cfg.Google.ClientID)
	span.End()
	if err != nil {
		return TokenPair{}, ErrUnauthorized
	}

	userID, err := s.users.FindUserIDByIdentity(ctx, "google", payload.Subject)
	switch {
	case err == nil:
		return s.issueFor(ctx, userID)
	case !errors.Is(err, user.ErrNotFound):
		return TokenPair{}, err
	}

	// No google identity yet; link by verified email if such an account exists.
	email, _ := payload.Claims["email"].(string)
	if email == "" {
		return TokenPair{}, ErrUnauthorized
	}
	uid, _, err := s.users.GetAuthByEmail(ctx, email)
	if errors.Is(err, user.ErrNotFound) {
		return TokenPair{}, ErrUnauthorized
	}
	if err != nil {
		return TokenPair{}, err
	}
	if err := s.users.AddIdentity(ctx, uid, user.Identity{Provider: "google", Subject: payload.Subject}); err != nil {
		return TokenPair{}, err
	}
	return s.issueFor(ctx, uid)
}

func otpKey(phone string) string      { return "otp:" + phone }
func otpTriesKey(phone string) string { return "otp_tries:" + phone }

// RequestOTP issues a one-time code to a known phone number. To avoid leaking which numbers
// are registered, it returns nil even when the phone has no account (no SMS is sent).
func (s *Service) RequestOTP(ctx context.Context, phone string) error {
	phone = strings.TrimSpace(phone)
	if _, err := s.users.GetUserIDByPhone(ctx, phone); err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return nil
		}
		return err
	}
	code, err := numericCode(6)
	if err != nil {
		return err
	}
	if err := s.redis.Set(ctx, otpKey(phone), code, s.cfg.SMS.OTPTTL).Err(); err != nil {
		return err
	}
	s.redis.Del(ctx, otpTriesKey(phone))
	return s.notify.SMS(ctx, "otp", phone, fmt.Sprintf("Your verification code is %s (valid %s).", code, s.cfg.SMS.OTPTTL))
}

func (s *Service) VerifyOTP(ctx context.Context, phone, code string) (TokenPair, error) {
	phone = strings.TrimSpace(phone)
	tries, err := s.redis.Incr(ctx, otpTriesKey(phone)).Result()
	if err != nil {
		return TokenPair{}, err
	}
	if tries == 1 {
		s.redis.Expire(ctx, otpTriesKey(phone), s.cfg.SMS.OTPTTL)
	}
	if int(tries) > s.cfg.SMS.OTPMaxAttempts {
		s.redis.Del(ctx, otpKey(phone))
		return TokenPair{}, ErrUnauthorized
	}

	want, err := s.redis.Get(ctx, otpKey(phone)).Result()
	if errors.Is(err, redis.Nil) || want != code {
		return TokenPair{}, ErrUnauthorized
	}
	if err != nil {
		return TokenPair{}, err
	}

	userID, err := s.users.GetUserIDByPhone(ctx, phone)
	if err != nil {
		return TokenPair{}, ErrUnauthorized
	}
	s.redis.Del(ctx, otpKey(phone), otpTriesKey(phone))
	return s.issueFor(ctx, userID)
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	userID, err := s.tokens.ConsumeRefresh(ctx, refreshToken)
	if err != nil {
		return TokenPair{}, ErrUnauthorized
	}
	return s.issueFor(ctx, userID)
}

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	return s.tokens.RevokeRefresh(ctx, refreshToken)
}

// ChangePassword verifies the caller's current password then replaces it.
func (s *Service) ChangePassword(ctx context.Context, userID uuid.UUID, currentPwd, newPwd string) error {
	hash, err := s.users.GetPasswordHashByID(ctx, userID)
	if err != nil {
		return ErrUnauthorized
	}
	_, span := otel.Tracer("auth").Start(ctx, "bcrypt.verify")
	bcryptErr := bcrypt.CompareHashAndPassword([]byte(*hash), []byte(currentPwd))
	span.End()
	if hash == nil || bcryptErr != nil {
		return ErrUnauthorized
	}
	_, hashSpan := otel.Tracer("auth").Start(ctx, "bcrypt.hash")
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPwd), bcrypt.DefaultCost)
	hashSpan.End()
	if err != nil {
		return err
	}
	return s.users.UpdatePasswordHash(ctx, userID, string(newHash))
}

// Me returns the current identity for the authenticated caller.
func (s *Service) Me(ctx context.Context, userID uuid.UUID) (rbac.Identity, error) {
	return s.users.LoadIdentity(ctx, userID)
}

func (s *Service) issueFor(ctx context.Context, userID uuid.UUID) (TokenPair, error) {
	id, err := s.users.LoadIdentity(ctx, userID)
	if err != nil {
		return TokenPair{}, err
	}
	access, err := s.tokens.IssueAccess(id)
	if err != nil {
		return TokenPair{}, err
	}
	refresh, err := s.tokens.IssueRefresh(ctx, userID)
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{AccessToken: access, RefreshToken: refresh, ExpiresIn: int(s.cfg.JWT.AccessTTL / time.Second)}, nil
}

// ForgotPassword sends a reset link to the given email. Returns nil even when the
// email is unknown so callers cannot enumerate accounts.
func (s *Service) ForgotPassword(ctx context.Context, email string) error {
	email = strings.TrimSpace(email)
	uid, _, err := s.users.GetAuthByEmail(ctx, email)
	if errors.Is(err, user.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	token := fmt.Sprintf("%x", b)

	if err := s.redis.Set(ctx, pwresetKey(token), uid.String(), pwresetTTL).Err(); err != nil {
		return err
	}

	link := fmt.Sprintf("%s/reset-password?token=%s", s.cfg.Mail.AppURL, token)
	body := fmt.Sprintf(
		`<p>A password reset for your account was requested.</p>`+
			`<p><a href="%s">Reset password</a> (link expires in 1 hour)</p>`+
			`<p>If you did not request this, you can safely ignore this email.</p>`,
		link,
	)
	return s.notify.Email(ctx, "password_reset", email, "Reset your password", body)
}

// ResetPassword validates the token and sets a new password, then invalidates the token.
func (s *Service) ResetPassword(ctx context.Context, token, newPwd string) error {
	uidStr, err := s.redis.Get(ctx, pwresetKey(token)).Result()
	if errors.Is(err, redis.Nil) {
		return ErrUnauthorized
	}
	if err != nil {
		return err
	}
	uid, err := uuid.Parse(uidStr)
	if err != nil {
		return ErrUnauthorized
	}
	_, hashSpan := otel.Tracer("auth").Start(ctx, "bcrypt.hash")
	hash, err := bcrypt.GenerateFromPassword([]byte(newPwd), bcrypt.DefaultCost)
	hashSpan.End()
	if err != nil {
		return err
	}
	if err := s.users.UpdatePasswordHash(ctx, uid, string(hash)); err != nil {
		return err
	}
	s.redis.Del(ctx, pwresetKey(token)) //nolint:errcheck
	return nil
}

func numericCode(n int) (string, error) {
	const digits = "0123456789"
	b := make([]byte, n)
	for i := range b {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", err
		}
		b[i] = digits[idx.Int64()]
	}
	return string(b), nil
}

