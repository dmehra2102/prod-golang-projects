package aurora

import (
	"context"
	"fmt"
	"time"

	"github.com/dmehra2102/prod-golang-projects/securevault/internal/config"
	"github.com/dmehra2102/prod-golang-projects/securevault/internal/secrets"
	"github.com/dmehra2102/prod-golang-projects/securevault/pkg/logger"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Client struct {
	Pool *pgxpool.Pool
	log  *logger.Logger
}

func New(
	ctx context.Context,
	cfg config.AuroraConfig,
	sm *secrets.Manager,
	log *logger.Logger,
) (*Client, error) {
	creds, err := sm.GetDBCredentials(ctx, cfg.SecretName)
	if err != nil {
		return nil, fmt.Errorf("aurora: load credentials: %w", err)
	}

	dsn := buildDSN(creds, cfg)
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("aurora: parse dsn: %w", err)
	}

	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnLifetime = cfg.MaxConnIdleTime
	poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	poolCfg.HealthCheckPeriod = cfg.HealthCheckPeriod
	poolCfg.ConnConfig.ConnectTimeout = cfg.ConnectTimeout

	poolCfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		log.Debug().Msg("aurora: new connection established")

		_, err := conn.Exec(ctx, "SET application_name = 'securevault'")
		return err
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("aurora: create pool: %w", err)
	}

	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(connectCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("aurora: ping failed: %w", err)
	}

	log.Info().
		Str("host", creds.Host).
		Int("port", creds.Port).
		Str("db", creds.DBName).
		Int32("max_conns", cfg.MaxConns).
		Msg("aurora: connection pool ready")

	return &Client{Pool: pool, log: log}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	return c.Pool.Ping(ctx)
}

func (c *Client) Close() {
	c.log.Info().Msg("aurora: closing connection pool")
	c.Pool.Close()
}

// Stats returns a snapshot of pool utilisation metrics.
func (c *Client) Stats() *pgxpool.Stat {
	return c.Pool.Stat()
}

// WithTx executes fn inside a serialisable transaction.
// The transaction is automatically committed on success or rolled back on error.
func (c *Client) WithTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := c.Pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.Serializable,
		AccessMode: pgx.ReadWrite,
	})
	if err != nil {
		return fmt.Errorf("aurora: begin tx: %w", err)
	}

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			c.log.Error().Err(rbErr).Msg("aurora: rollback failed")
		}
		return err
	}
	return tx.Commit(ctx)
}

// WithReadTx executes fn inside a read-only transaction.
func (c *Client) WithReadTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := c.Pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.RepeatableRead,
		AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return fmt.Errorf("aurora: begin read tx: %w", err)
	}
	defer func() {
		// A read-only transaction can always be safely rolled back.
		_ = tx.Rollback(ctx)
	}()
	return fn(tx)
}

func buildDSN(creds *secrets.DBCredentials, cfg config.AuroraConfig) string {
	dbName := creds.DBName
	if cfg.DBName != "" {
		dbName = cfg.DBName
	}
	return fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s "+
			"sslmode=require connect_timeout=%d",
		creds.Host,
		creds.Port,
		dbName,
		creds.Username,
		creds.Password,
		int(cfg.ConnectTimeout.Seconds()),
	)
}
