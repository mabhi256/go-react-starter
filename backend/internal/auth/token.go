// Package auth implements login (email/password, Google, phone OTP) and token issuance.
// Access tokens are short-lived JWTs carrying org + roles; refresh tokens are opaque random
// strings stored in Redis so they can be revoked and rotated.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/your-org/go-react-starter/backend/internal/config"
	"github.com/your-org/go-react-starter/backend/internal/rbac"
)

var ErrInvalidToken = errors.New("invalid or expired token")

type accessClaims struct {
	OrgID string   `json:"org_id,omitempty"`
	Roles []string `json:"roles"`
	jwt.RegisteredClaims
}

type TokenService struct {
	secret     []byte
	issuer     string
	accessTTL  time.Duration
	refreshTTL time.Duration
	redis      *redis.Client
}

func NewTokenService(cfg config.Config, rdb *redis.Client) *TokenService {
	return &TokenService{
		secret:     []byte(cfg.JWT.Secret),
		issuer:     cfg.JWT.Issuer,
		accessTTL:  cfg.JWT.AccessTTL,
		refreshTTL: cfg.JWT.RefreshTTL,
		redis:      rdb,
	}
}

func (s *TokenService) IssueAccess(id rbac.Identity) (string, error) {
	now := time.Now()
	roles := make([]string, len(id.Roles))
	for i, r := range id.Roles {
		roles[i] = string(r)
	}
	claims := accessClaims{
		Roles: roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   id.UserID.String(),
			Issuer:    s.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTTL)),
		},
	}
	if id.OrgID != nil {
		claims.OrgID = id.OrgID.String()
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}

func (s *TokenService) ParseAccess(raw string) (rbac.Identity, error) {
	var claims accessClaims
	_, err := jwt.ParseWithClaims(raw, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return s.secret, nil
	}, jwt.WithIssuer(s.issuer))
	if err != nil {
		return rbac.Identity{}, ErrInvalidToken
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return rbac.Identity{}, ErrInvalidToken
	}
	id := rbac.Identity{UserID: userID}
	if claims.OrgID != "" {
		orgID, err := uuid.Parse(claims.OrgID)
		if err != nil {
			return rbac.Identity{}, ErrInvalidToken
		}
		id.OrgID = &orgID
	}
	for _, r := range claims.Roles {
		id.Roles = append(id.Roles, rbac.Role(r))
	}
	return id, nil
}

func refreshKey(token string) string { return "refresh:" + token }

// IssueRefresh creates an opaque refresh token bound to a user in Redis.
func (s *TokenService) IssueRefresh(ctx context.Context, userID uuid.UUID) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := hex.EncodeToString(buf)
	if err := s.redis.Set(ctx, refreshKey(token), userID.String(), s.refreshTTL).Err(); err != nil {
		return "", fmt.Errorf("store refresh token: %w", err)
	}
	return token, nil
}

// ConsumeRefresh validates a refresh token and deletes it (one-time use / rotation).
func (s *TokenService) ConsumeRefresh(ctx context.Context, token string) (uuid.UUID, error) {
	val, err := s.redis.GetDel(ctx, refreshKey(token)).Result()
	if errors.Is(err, redis.Nil) {
		return uuid.Nil, ErrInvalidToken
	}
	if err != nil {
		return uuid.Nil, err
	}
	return uuid.Parse(val)
}

// RevokeRefresh removes a refresh token (logout).
func (s *TokenService) RevokeRefresh(ctx context.Context, token string) error {
	return s.redis.Del(ctx, refreshKey(token)).Err()
}

