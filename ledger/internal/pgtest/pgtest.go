package pgtest

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func New(t *testing.T, migrationsDir string) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("integration test: skipped with -short")
	}
	ctx := context.Background()

	ctr, err := postgres.Run(ctx, "postgres:17",
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.WithDatabase("test"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start posgres: %v", err)
	}
	t.Cleanup(func() { _ = ctr.Terminate(context.Background()) })

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("find migrations in %s: %v (found %d)", migrationsDir, err, len(files))
	}

	for _, f := range files {
		sql, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("apply %s: %v", f, err)
		}
	}
	return pool
}
