//go:build integration

// Package conn_test starts the four real connection providers against live
// services (docker compose up -d --wait postgres mysql mongo redis), applies the
// Postgres goose migrations and exercises ORM / query syntax on each database.
// Run with: make test-integration.
//
// One fx app (the real providers) is shared by every test in the package; tests
// are parallel-safe because each one uses unique table, collection, key or
// email names and cleans up after itself.
//
// Connection settings come from config/config_demo.yaml; any key present in
// that file can be overridden by environment variables (POSTGRES_MAIN_MASTER_PORT,
// MYSQL_MAIN_MASTER_HOST, MONGO_MAIN_HOSTS, REDIS_ADDRESS, ...).
package conn_test

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"server-template/config"
	"server-template/internal/repository/conn/mongo"
	"server-template/internal/repository/conn/mysql"
	"server-template/internal/repository/conn/postgres"
	"server-template/internal/repository/conn/redis"

	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	mongodriver "go.mongodb.org/mongo-driver/v2/mongo"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

type deps struct {
	fx.In

	Postgres *gorm.DB            `name:"default_postgres"`
	MySQL    *gorm.DB            `name:"default_mysql"`
	Mongo    *mongodriver.Client `name:"default_mongo"`
	Redis    *goredis.ClusterClient
}

// d is populated by TestMain once the providers have started.
var d deps //nolint:gochecknoglobals // shared fx app started once in TestMain

// seq makes names unique within one test binary run.
var seq atomic.Int64 //nolint:gochecknoglobals // process-wide counter for unique names

// uniq returns a name that is unique per run and per call (safe in identifiers:
// lower-case letters, digits and underscores only).
func uniq(prefix string) string {
	return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano(), seq.Add(1))
}

// cleanupCtx outlives t.Context(), which is already canceled when Cleanup runs.
func cleanupCtx(t *testing.T) context.Context {
	t.Helper()

	return context.WithoutCancel(t.Context())
}

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	// The demo config lives in <repo>/config; this package is three levels deep.
	// os.Chdir (not t.Chdir) so the tests themselves can run in parallel.
	if err := os.Chdir("../../.."); err != nil {
		fmt.Fprintln(os.Stderr, "chdir:", err)

		return 1
	}

	cfg, err := config.LoadWithEnv[config.Config]("config_demo", "config")
	if err != nil {
		fmt.Fprintln(os.Stderr, "load config:", err)

		return 1
	}
	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		fmt.Fprintln(os.Stderr, "validate config:", err)

		return 1
	}

	app := fx.New(
		fx.NopLogger,
		fx.Supply(cfg),
		fx.Provide(postgres.New, mysql.New, mongo.New, redis.New),
		fx.Invoke(func(in deps) { d = in }),
	)

	// OnStart pings every provider: failure here means a service is unreachable.
	startCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err = app.Start(startCtx)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, "start providers:", err)

		return 1
	}

	if err := migratePostgres(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "migrate postgres:", err)
		_ = app.Stop(context.Background())

		return 1
	}

	code := m.Run()

	stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := app.Stop(stopCtx); err != nil {
		fmt.Fprintln(os.Stderr, "stop providers:", err)
		if code == 0 {
			code = 1
		}
	}

	return code
}

func TestConnectionsStartAndServe(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	t.Run("postgres", func(t *testing.T) {
		t.Parallel()

		var one int
		require.NoError(t, d.Postgres.WithContext(ctx).Raw("SELECT 1").Scan(&one).Error)
		assert.Equal(t, 1, one)
	})

	t.Run("mysql", func(t *testing.T) {
		t.Parallel()

		var one int
		require.NoError(t, d.MySQL.WithContext(ctx).Raw("SELECT 1").Scan(&one).Error)
		assert.Equal(t, 1, one)
	})

	t.Run("mongo", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, d.Mongo.Ping(ctx, nil))

		coll := d.Mongo.Database("integration_test").Collection(uniq("conn"))
		t.Cleanup(func() { _ = coll.Drop(cleanupCtx(t)) })

		_, err := coll.InsertOne(ctx, bson.M{"name": "template"})
		require.NoError(t, err)

		var got bson.M
		require.NoError(t, coll.FindOne(ctx, bson.M{"name": "template"}).Decode(&got))
		assert.Equal(t, "template", got["name"])
	})

	t.Run("redis", func(t *testing.T) {
		t.Parallel()

		key := uniq("integration:conn")
		t.Cleanup(func() { _ = d.Redis.Del(cleanupCtx(t), key).Err() })

		require.NoError(t, d.Redis.Set(ctx, key, "ok", time.Minute).Err())

		val, err := d.Redis.Get(ctx, key).Result()
		require.NoError(t, err)
		assert.Equal(t, "ok", val)

		n, err := d.Redis.Del(ctx, key).Result()
		require.NoError(t, err)
		assert.EqualValues(t, 1, n)
	})
}
