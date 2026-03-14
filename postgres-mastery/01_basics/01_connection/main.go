// ===========================================
// MODULE 1: CONNECTING TO POSTGRESQL
// ===========================================

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/dmehra2102/prod-golang-projects/postgres-mastery/config"
	"github.com/dmehra2102/prod-golang-projects/postgres-mastery/internal/logger"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	logger.Header("MODULE 1 - LESSON 1", "Connecting to PostgreSQL")

	ctx := context.Background()
	cfg := config.Default()

	// ─── PART 1: UNDERSTANDING THE DSN ────────────────────────────
	logger.Lesson("Part 1 — The Connection String (DSN)")
	logger.Explain(`A DSN (Data Source Name) encodes all connection parameters.
		pgx accepts the PostgreSQL URI format:
		postgres://[user[:password]@][host][:port][/dbname][?param=value]
		
		Common parameters:
		sslmode=disable|require|verify-full
		connect_timeout=10            (seconds)
		pool_max_conns=25             (pgxpool only)
		statement_cache_mode=describe (server-side prepared statements)`,
	)

	logger.Result("DSN", cfg.DSN())
	logger.Result("LibPQ DSN", cfg.LibPQDSN())

	// ─── PART 2: SINGLE CONNECTION ────────────────────────────────
	logger.Lesson("Part 2 — Single Connection (pgx.Conn)")
	logger.Explain(`pgx.Connect creates ONE connection to PostgreSQL.
		Use this for:
  		• Command-line scripts and one-shot jobs
  		• LISTEN/NOTIFY (requires a dedicated connection)
  		• Session-scoped settings (SET LOCAL, advisory locks)
  		• Testing and migrations`,
	)

	conn, err := pgx.Connect(ctx, cfg.DSN())
	if err != nil {
		logger.Fatal(err)
		return
	}
	defer func() {
		conn.Close(ctx)
		logger.Success("Single connnection closed cleanly.")
	}()

	logger.Success("pgx.Connect -> connected!")

	// ─── PART 3: SERVER INTROSPECTION ─────────────────────────────
	logger.Lesson("Part 3 — Server Version & Runtime Config")

	var version string
	logger.SQL("SELECT version()")
	err = conn.QueryRow(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		logger.Error(err)
		return
	}
	logger.Result("PostgreSQL version", version)

	// Read runtime parameters (pg_settings)
	type pgSetting struct {
		name    string
		setting string
		unit    string
	}

	interestingSettings := []string{
		"max_connections",
		"shared_buffers",
		"work_mem",
		"maintenance_work_mem",
		"effective_cache_size",
		"wal_level",
		"timezone",
	}

	logger.SQL(`SELECT name, setting, unit
		FROM pg_settings
		WHERE name = ANY($1)
		ORDER BY name`,
	)

	rows, err := conn.Query(ctx, `
		SELECT name, setting, COALESCE(unit, '') AS unit
		FROM pg_settings
		WHERE name = ANY($1)
		ORDER BY name`,
		interestingSettings,
	)

	if err != nil {
		logger.Error(err)
		return
	}
	defer rows.Close()

	logger.Divider()
	logger.TableHeader("Setting", "Value", "Unit")
	for rows.Next() {
		var s pgSetting
		if err := rows.Scan(&s.name, &s.setting, &s.unit); err != nil {
			logger.Error(err)
			return
		}
		logger.TableRow(s.name, s.setting, s.unit)
	}
	if err := rows.Err(); err != nil {
		logger.Error(err)
		return
	}

	// ─── PART 4: CONTEXT AND TIMEOUTS ─────────────────────────────
	logger.Lesson("Part 4 — Context & Timeouts")
	logger.Explain(`Every pgx operation accepts a context.Context.
		This is crucial for production:
		• Cancelled HTTP requests cancel in-flight queries
		• Timeouts prevent slow queries from blocking forever
		• Shutdown signals propagate cleanly through the query stack`,
	)

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var now time.Time
	logger.SQL("SELECT NOW()")
	err = conn.QueryRow(timeoutCtx, "SELECT NOW()").Scan(&now)
	if err != nil {
		logger.Error(err)
		return
	}
	logger.Result("Server time (with 5s deadline)", now.Format(time.RFC3339))

	// ─── PART 5: CONNECTION POOL ───────────────────────────────────
	logger.Lesson("Part 5 — Connection Pool (pgxpool)")
	logger.Explain(`For web servers and APIs, use pgxpool.Pool.
		It manages a pool of connections that are reused across requests.
		
		Key pool parameters:
		MaxConns          — hard cap on open connections (default: 4 x CPU)
		MinConns          — always keep N connections warm
		MaxConnLifetime   — rotate connections (avoids stale proxies)
		MaxConnIdleTime   — close idle connections to save DB resources
		HealthCheckPeriod — periodically ping idle connections`,
	)

	poolConfig, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		logger.Error(err)
		return
	}

	poolConfig.MaxConns = 20
	poolConfig.MinConns = 2
	poolConfig.MaxConnLifetime = 30 * time.Minute
	poolConfig.MaxConnIdleTime = 5 * time.Second
	poolConfig.HealthCheckPeriod = 1 * time.Minute

	// AfterConnect hook — run setup on every new connection
	// Common uses: SET search_path, SET application_name, SET timezone
	poolConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET application_name = 'pgmastery'")
		return err
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		logger.Error(err)
		return
	}
	defer func() {
		pool.Close()
		logger.Success("Pool closed cleanly.")
	}()

	stats := pool.Stat()
	logger.Result("Pool — TotalConns", stats.TotalConns())
	logger.Result("Pool — IdleConns", stats.IdleConns())
	logger.Result("Pool — MaxConns", stats.MaxConns())

	poolConn, err := pool.Acquire(ctx)
	if err != nil {
		logger.Error(err)
		return
	}
	defer poolConn.Release() // ← ALWAYS release! Forgetting this leaks connections.

	logger.Success("Acquired connection from pool")

	// Use the acquired connection
	var appName string
	logger.SQL("SHOW application_name  -- set by AfterConnect hook")
	err = poolConn.QueryRow(ctx, "SHOW application_name").Scan(&appName)
	if err != nil {
		logger.Error(err)
		return
	}
	logger.Result("application_name", appName)

	// ─── PART 6: CONNECTION HYGIENE ───────────────────────────────
	logger.Lesson("Part 6 — Connection Hygiene Checklist")
	logger.Explain(`Production connection best practices:
  		✓ Always defer conn.Close(ctx) or pool.Close()
  		✓ Always defer poolConn.Release() after Acquire()
  		✓ Always defer rows.Close() after Query()
  		✓ Always check rows.Err() after the rows loop
  		✓ Pass context everywhere; never use context.Background() in handlers
  		✓ Set application_name so pg_stat_activity is readable
  		✓ Use pgxpool for anything serving concurrent requests
  		✓ Configure MaxConnLifetime to handle load balancer reconnects
  		✓ Never share a pgx.Conn between goroutines (use pool instead)`,
	)

	fmt.Println()
	logger.Success("Lesson 1 complete! Next: go run ./01_basics/02_datatypes/main.go")
}
