// Command migrate applies all tern migrations to the configured database, then exits.
// Run via `make migrate` or `go run ./cmd/migrate`.
package main

import (
	"context"
	"os"
	"time"

	"github.com/your-org/go-react-starter/backend/internal/config"
	"github.com/your-org/go-react-starter/backend/internal/platform"
)

func main() {
	cfg, err := config.Load(".env")
	logger := platform.NewLogger(cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("load config")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := platform.NewDB(ctx, cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("connect db")
	}
	defer pool.Close()

	if err := platform.RunMigrations(ctx, pool); err != nil {
		logger.Fatal().Err(err).Msg("migrate")
	}
	logger.Info().Msg("migrations applied")
	os.Exit(0)
}

