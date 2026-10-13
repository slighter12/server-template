package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"server-template/config"
	"server-template/internal/domain/lifecycle"
	"server-template/internal/repository/conn"

	pgLib "github.com/slighter12/go-lib/database/postgres"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

const (
	dbPoolMonitorInterval       = 5 * time.Second
	dbPoolWarnDurationThreshold = 50 * time.Millisecond
)

// Params defines the required parameters.
type Params struct {
	fx.In
	fx.Lifecycle

	Config *config.Config
	// Logger is used for GORM and pool logging; slog.Default() when absent.
	Logger *slog.Logger `optional:"true"`
}

// Result is the connection map plus the default client.
type Result struct {
	fx.Out

	Clients map[string]*gorm.DB `name:"postgres_maps"`
	Default *gorm.DB            `name:"default_postgres"`
}

// New creates PostgreSQL clients. PostgreSQL is optional: with an empty config
// section it returns empty results and registers no lifecycle hooks.
func New(params Params) (Result, error) {
	result := Result{Clients: make(map[string]*gorm.DB)}
	if len(params.Config.Postgres) == 0 {
		return result, nil
	}

	logger := params.Logger
	if logger == nil {
		logger = slog.Default()
	}
	defaultName, _ := conn.DefaultName(params.Config.Postgres)

	for _, name := range conn.SortedNames(params.Config.Postgres) {
		db, err := pgLib.New(params.Config.Postgres[name])
		if err != nil {
			return Result{Clients: make(map[string]*gorm.DB)}, fmt.Errorf("failed to create PostgreSQL client %s: %w", name, err)
		}
		db = db.Session(&gorm.Session{Logger: newGormSlogLogger(logger, params.Config)})

		sqlDB, err := db.DB()
		if err != nil {
			return Result{Clients: make(map[string]*gorm.DB)}, fmt.Errorf("failed to get PostgreSQL sql.DB %s: %w", name, err)
		}

		var stopMonitor chan struct{}
		monitor := name == defaultName
		params.Append(fx.Hook{
			OnStart: func(startCtx context.Context) error {
				ctx, cancel := context.WithTimeout(startCtx, lifecycle.DefaultTimeout)
				defer cancel()

				if err := sqlDB.PingContext(ctx); err != nil {
					return fmt.Errorf("failed to ping PostgreSQL %s: %w", name, err)
				}
				if monitor {
					stopMonitor = make(chan struct{})
					go monitorDBPool(context.WithoutCancel(startCtx), stopMonitor, logger, sqlDB, dbPoolMonitorInterval)
				}

				return nil
			},
			OnStop: func(context.Context) error {
				if stopMonitor != nil {
					close(stopMonitor)
				}

				return sqlDB.Close()
			},
		})

		result.Clients[name] = db
	}
	result.Default = result.Clients[defaultName]

	return result, nil
}

// monitorDBPool logs when callers had to wait for a free pool connection.
func monitorDBPool(ctx context.Context, stop <-chan struct{}, logger *slog.Logger, sqlDB *sql.DB, interval time.Duration) {
	if logger == nil || sqlDB == nil {
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	prev := sqlDB.Stats()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			cur := sqlDB.Stats()
			waitDelta := cur.WaitCount - prev.WaitCount
			waitDurationDelta := cur.WaitDuration - prev.WaitDuration

			if waitDelta > 0 {
				attrs := []slog.Attr{
					slog.Int64("wait_count_delta", waitDelta),
					slog.Duration("wait_duration_delta", waitDurationDelta),
					slog.Duration("avg_wait", waitDurationDelta/time.Duration(waitDelta)),
					slog.Int("max_open_conns", cur.MaxOpenConnections),
					slog.Int("open_conns", cur.OpenConnections),
					slog.Int("in_use_conns", cur.InUse),
					slog.Int("idle_conns", cur.Idle),
					slog.Int64("wait_count_total", cur.WaitCount),
					slog.Duration("wait_duration_total", cur.WaitDuration),
				}
				if waitDurationDelta >= dbPoolWarnDurationThreshold {
					logger.LogAttrs(ctx, slog.LevelWarn, "Postgres pool wait detected", attrs...)
				} else {
					logger.LogAttrs(ctx, slog.LevelDebug, "Postgres pool wait observed", attrs...)
				}
			}

			prev = cur
		}
	}
}
