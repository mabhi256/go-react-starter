// Command api starts the HTTP server. Wire all platform dependencies and register
// every domain handler onto the Huma API before serving.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"

	"github.com/your-org/go-react-starter/backend/internal/audit"
	authpkg "github.com/your-org/go-react-starter/backend/internal/auth"
	"github.com/your-org/go-react-starter/backend/internal/config"
	"github.com/your-org/go-react-starter/backend/internal/items"
	"github.com/your-org/go-react-starter/backend/internal/notify"
	"github.com/your-org/go-react-starter/backend/internal/org"
	"github.com/your-org/go-react-starter/backend/internal/platform"
	"github.com/your-org/go-react-starter/backend/internal/queueadmin"
	"github.com/your-org/go-react-starter/backend/internal/rbac"
	"github.com/your-org/go-react-starter/backend/internal/server"
	"github.com/your-org/go-react-starter/backend/internal/user"
)

func main() {
	cfg, err := config.Load(".env")
	logger := platform.NewLogger(cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("load config")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := platform.NewDB(ctx, cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("connect db")
	}
	defer db.Close()

	if err := platform.RunMigrations(ctx, db); err != nil {
		logger.Fatal().Err(err).Msg("run migrations")
	}

	rdb, err := platform.NewRedis(ctx, cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("connect redis")
	}
	defer rdb.Close()

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

	asynqClient := asynq.NewClient(platform.AsynqRedisOpt(cfg))
	defer asynqClient.Close()

	auditRec := audit.NewAsynqRecorder(asynqClient)
	userRepo := user.NewRepo(db)
	tokens := authpkg.NewTokenService(cfg, rdb)
	authSvc := authpkg.NewService(userRepo, tokens, notify.NewAsynqSender(asynqClient), rdb, cfg)
	orgRepo := org.NewRepo(db)
	itemsRepo := items.NewRepo(db)

	e, api := server.New(cfg, tokens, logger)

	authpkg.NewHandler(authSvc).Register(api)
	org.NewHandler(orgRepo, userRepo, auditRec).Register(api)
	user.NewHandler(userRepo, auditRec).Register(api)
	items.NewHandler(itemsRepo, auditRec).Register(api)
	queueadmin.NewHandler(asynq.NewInspector(platform.AsynqRedisOpt(cfg)), auditRec).Register(api)

	if err := bootstrapSuperAdmin(ctx, cfg, userRepo, logger); err != nil {
		logger.Fatal().Err(err).Msg("bootstrap super admin")
	}

	addr := fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port)
	logger.Info().Str("addr", addr).Msg("starting API server")

	errCh := make(chan error, 1)
	go func() { errCh <- e.Start(addr) }()

	select {
	case <-ctx.Done():
		logger.Info().Msg("shutting down")
		shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = e.Shutdown(shutCtx)
	case err := <-errCh:
		if err != nil {
			logger.Fatal().Err(err).Msg("server error")
		}
	}
}

func bootstrapSuperAdmin(ctx context.Context, cfg config.Config, users *user.Repo, log zerolog.Logger) error {
	if cfg.SuperAdmin.Email == "" {
		return nil
	}
	if cfg.SuperAdmin.Password == "" {
		return fmt.Errorf("SUPER_ADMIN_PASSWORD must be set when SUPER_ADMIN_EMAIL is set")
	}
	exists, err := users.SuperAdminExists(ctx)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.SuperAdmin.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	hashStr := string(hash)
	email := cfg.SuperAdmin.Email
	_, err = users.CreateWithIdentity(ctx, user.NewUser{
		Name:         "Super Admin",
		Email:        &email,
		PasswordHash: &hashStr,
		Roles:        []rbac.Role{rbac.RoleSuperAdmin},
	}, &user.Identity{Provider: "password", Subject: email})
	if err != nil {
		return err
	}
	log.Info().Str("email", email).Msg("super admin created")
	return nil
}
