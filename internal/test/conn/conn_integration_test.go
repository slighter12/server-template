//go:build integration

// Package conn_test starts the four real connection providers against live
// services (docker compose up -d --wait postgres mysql mongo redis) and does
// one real operation on each. Run with: make test-integration.
//
// Connection settings come from config/config_demo.yaml; any key present in
// that file can be overridden by environment variables (POSTGRES_MAIN_MASTER_PORT,
// MYSQL_MAIN_MASTER_HOST, MONGO_MAIN_HOSTS, REDIS_ADDRESS, ...).
package conn_test

import (
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
	"go.uber.org/fx/fxtest"
	"gorm.io/gorm"
)

type deps struct {
	fx.In

	Postgres *gorm.DB            `name:"default_postgres"`
	MySQL    *gorm.DB            `name:"default_mysql"`
	Mongo    *mongodriver.Client `name:"default_mongo"`
	Redis    *goredis.ClusterClient
}

func loadDemoConfig(t *testing.T) *config.Config {
	t.Helper()

	// The demo config lives in <repo>/config; this package is two levels deep.
	t.Chdir("../../..")

	cfg, err := config.LoadWithEnv[config.Config]("config_demo", "config")
	require.NoError(t, err)
	cfg.ApplyDefaults()
	require.NoError(t, cfg.Validate())

	return cfg
}

func TestConnectionsStartAndServe(t *testing.T) {
	cfg := loadDemoConfig(t)

	var d deps
	app := fxtest.New(t,
		fx.NopLogger,
		fx.Supply(cfg),
		fx.Provide(postgres.New, mysql.New, mongo.New, redis.New),
		fx.Invoke(func(in deps) { d = in }),
	)
	// OnStart pings every provider: failure here means a service is unreachable.
	app.RequireStart()
	t.Cleanup(app.RequireStop)

	ctx := t.Context()

	t.Run("postgres", func(t *testing.T) {
		var one int
		require.NoError(t, d.Postgres.WithContext(ctx).Raw("SELECT 1").Scan(&one).Error)
		assert.Equal(t, 1, one)
	})

	t.Run("mysql", func(t *testing.T) {
		var one int
		require.NoError(t, d.MySQL.WithContext(ctx).Raw("SELECT 1").Scan(&one).Error)
		assert.Equal(t, 1, one)
	})

	t.Run("mongo", func(t *testing.T) {
		require.NoError(t, d.Mongo.Ping(ctx, nil))

		coll := d.Mongo.Database("integration_test").Collection("conn_" + time.Now().Format("150405.000000000"))
		t.Cleanup(func() { _ = coll.Drop(ctx) })

		_, err := coll.InsertOne(ctx, bson.M{"name": "template"})
		require.NoError(t, err)

		var got bson.M
		require.NoError(t, coll.FindOne(ctx, bson.M{"name": "template"}).Decode(&got))
		assert.Equal(t, "template", got["name"])

		require.NoError(t, coll.Drop(ctx))
	})

	t.Run("redis", func(t *testing.T) {
		key := "integration:conn:" + time.Now().Format("150405.000000000")
		require.NoError(t, d.Redis.Set(ctx, key, "ok", time.Minute).Err())

		val, err := d.Redis.Get(ctx, key).Result()
		require.NoError(t, err)
		assert.Equal(t, "ok", val)

		n, err := d.Redis.Del(ctx, key).Result()
		require.NoError(t, err)
		assert.EqualValues(t, 1, n)
	})
}
