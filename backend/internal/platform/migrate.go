package platform

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/tern/v2/migrate"

	"github.com/your-org/go-react-starter/backend/db"
)

// RunMigrations applies all pending tern migrations. No-op when already current.
// Safe to call concurrently; tern holds a Postgres advisory lock.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire conn: %w", err)
	}
	defer conn.Release()

	m, err := migrate.NewMigrator(ctx, conn.Conn(), "schema_version")
	if err != nil {
		return fmt.Errorf("new migrator: %w", err)
	}
	if err := m.LoadMigrations(db.Migrations()); err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}
	return m.Migrate(ctx)
}

