package tests

import (
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	authpkg "github.com/your-org/go-react-starter/backend/internal/auth"
	"github.com/your-org/go-react-starter/backend/internal/config"
)

func newTestUUID() uuid.UUID { return uuid.Must(uuid.NewV7()) }

func newTokenServiceForTest(cfg config.Config, rdb *redis.Client) *authpkg.TokenService {
	return authpkg.NewTokenService(cfg, rdb)
}

