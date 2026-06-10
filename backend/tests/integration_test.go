// Integration tests spin up real Postgres and Redis via testcontainers.
// Critical invariants tested: multi-tenant org isolation (items), and auth token revocation.
// Run with: go test ./tests/...  (requires Docker)
package tests

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"

	"github.com/your-org/go-react-starter/backend/db"
	"github.com/your-org/go-react-starter/backend/internal/config"
	"github.com/your-org/go-react-starter/backend/internal/items"
	"github.com/your-org/go-react-starter/backend/internal/org"
	"github.com/your-org/go-react-starter/backend/internal/platform"
	"github.com/your-org/go-react-starter/backend/internal/user"

	"github.com/jackc/tern/v2/migrate"
	"github.com/stretchr/testify/require"
)

func newTestConfig(pgDSN, redisAddr string) config.Config {
	cfg := config.Config{}
	cfg.AppEnv = config.EnvDev
	cfg.DB.DSN = pgDSN
	cfg.Redis.Addr = redisAddr
	cfg.JWT.Secret = "test-secret-32-bytes-long-enough"
	cfg.JWT.Issuer = "test"
	cfg.JWT.AccessTTL = 15 * time.Minute
	cfg.JWT.RefreshTTL = 24 * time.Hour
	cfg.SMS.Provider = "log"
	cfg.SMS.OTPTTL = 5 * time.Minute
	cfg.SMS.OTPMaxAttempts = 5
	cfg.OTEL.ServiceName = "test"
	return cfg
}

func applyMigrations(ctx context.Context, t *testing.T, dsn string) {
	t.Helper()
	conn, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	defer conn.Close(ctx)

	m, err := migrate.NewMigrator(ctx, conn, "schema_version")
	require.NoError(t, err)
	require.NoError(t, m.LoadMigrations(db.Migrations()))
	require.NoError(t, m.Migrate(ctx))
}

// TestItemOrgIsolation verifies that a user in org A cannot read items created in org B.
func TestItemOrgIsolation(t *testing.T) {
	ctx := context.Background()

	pgC, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("starter"),
		tcpostgres.WithUsername("starter"),
		tcpostgres.WithPassword("starter"),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pgC.Terminate(ctx) })

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	applyMigrations(ctx, t, dsn)

	cfg := newTestConfig(dsn, "")
	pool, err := platform.NewDB(ctx, cfg)
	require.NoError(t, err)
	defer pool.Close()

	orgRepo := org.NewRepo(pool)
	orgA, err := orgRepo.Create(ctx, org.NewOrg{Name: "Org A", Type: "clinic", Address: org.Address{}})
	require.NoError(t, err)
	orgB, err := orgRepo.Create(ctx, org.NewOrg{Name: "Org B", Type: "clinic", Address: org.Address{}})
	require.NoError(t, err)

	userRepo := user.NewRepo(pool)
	actor, err := userRepo.CreateWithIdentity(ctx, user.NewUser{OrgID: &orgA.ID, Name: "User A"}, nil)
	require.NoError(t, err)

	itemRepo := items.NewRepo(pool)

	// Create an item scoped to org A.
	it, err := itemRepo.Create(ctx, items.NewItem{
		OrgID: orgA.ID, Name: "Widget", CreatedBy: &actor.ID,
	})
	require.NoError(t, err)

	// Fetch with org A filter: should succeed.
	_, err = itemRepo.GetByID(ctx, it.ID, &orgA.ID)
	require.NoError(t, err)

	// Fetch with org B filter: must return ErrNotFound.
	_, err = itemRepo.GetByID(ctx, it.ID, &orgB.ID)
	require.ErrorIs(t, err, items.ErrNotFound, "cross-org lookup must return not found")
}

// TestAuthRefreshRevocation issues a refresh token, uses it once (should succeed),
// then tries to use the same token again (should fail: one-time rotation).
func TestAuthRefreshRevocation(t *testing.T) {
	ctx := context.Background()

	rC, err := tcredis.Run(ctx, "valkey/valkey:8-alpine")
	require.NoError(t, err)
	t.Cleanup(func() { _ = rC.Terminate(ctx) })

	addr, err := rC.ConnectionString(ctx)
	require.NoError(t, err)
	addr = addr[len("redis://"):]

	cfg := newTestConfig("postgres://unused", addr)
	rdb, err := platform.NewRedis(ctx, cfg)
	require.NoError(t, err)
	defer rdb.Close()

	tokens := newTokenServiceForTest(cfg, rdb)

	userID := newTestUUID()
	refresh, err := tokens.IssueRefresh(ctx, userID)
	require.NoError(t, err)

	got, err := tokens.ConsumeRefresh(ctx, refresh)
	require.NoError(t, err)
	require.Equal(t, userID, got)

	_, err = tokens.ConsumeRefresh(ctx, refresh)
	require.Error(t, err, "consumed token must not be reusable")
}
