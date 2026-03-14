package db

import (
	"context"
	"fmt"

	"github.com/dmehra2102/prod-golang-projects/postgres-mastery/config"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(ctx context.Context) (*pgx.Conn, error) {
	cfg := config.Default()
	conn, err := pgx.Connect(ctx, cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("pgx connect: %w", err)
	}
	return conn, nil
}

// NewPool creates a connection pool using pgxpool.
// Pool is the recommended approach for production web services.
func NewPool(ctx context.Context) (*pgxpool.Pool, error) {
	cfg := config.Default()

	poolCfg, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("pgxpool.ParseConfig: %w", err)
	}

	poolCfg.MaxConns = 25
	poolCfg.MinConns = 2

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.NewWithConfig: %w", err)
	}

	return pool, nil
}

// MustConnect panics if the connection cannot be established.
func MustConnect(ctx context.Context) *pgx.Conn {
	conn, err := Connect(ctx)
	if err != nil {
		panic(fmt.Sprintf("could not connect to PostgreSQL: %v\n\nMake sure Docker is running:\n  docker-compose up -d\n", err))
	}
	return conn
}

// MustPool panics if the pool cannot be created.
func MustPool(ctx context.Context) *pgxpool.Pool {
	pool, err := NewPool(ctx)
	if err != nil {
		panic(fmt.Sprintf("could not create pool: %v\n\nMake sure Docker is running:\n  docker-compose up -d\n", err))
	}
	return pool
}

// DropAndCreate is a test helper that drops and recreates a table.
func DropAndCreate(ctx context.Context, conn *pgx.Conn, ddl string) error {
	_, err := conn.Exec(ctx, ddl)
	return err
}

// ExecFile executes a multistatement SQL string (e.g., schema setup).
func ExecFile(ctx context.Context, conn *pgx.Conn, sql string) error {
	_, err := conn.Exec(ctx, sql)
	return err
}

// Ping checks that the connection is alive and the server is reachable.
func Ping(ctx context.Context, conn *pgx.Conn) error {
	return conn.Ping(ctx)
}
