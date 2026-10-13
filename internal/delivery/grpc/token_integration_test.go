//go:build integration

package grpc

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"server-template/config"
	"server-template/internal/domain/entity"
	"server-template/internal/pkg/apperr"
	"server-template/internal/repository/conn/redis"
	"server-template/proto/pb/authpb"

	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

// TestTokenStoreAgainstRedisCluster runs the gRPC token path (Login issues a
// token, Logout revokes it, ValidateToken rejects it) against the real Redis
// Cluster provider. Run with: make test-integration (services from docker compose).
func TestTokenStoreAgainstRedisCluster(t *testing.T) {
	t.Chdir("../../..") // config/config_demo.yaml lives in the repo root

	cfg, err := config.LoadWithEnv[config.Config]("config_demo", "config")
	require.NoError(t, err)
	cfg.ApplyDefaults()
	require.NoError(t, cfg.Validate())

	var rdb *goredis.ClusterClient
	app := fxtest.New(t,
		fx.NopLogger,
		fx.Supply(cfg),
		fx.Provide(redis.New),
		fx.Populate(&rdb),
	)
	app.RequireStart() // OnStart pings the cluster
	t.Cleanup(app.RequireStop)

	cfg.Auth.JWTSecret = "integration-secret-with-at-least-32-bytes"
	cfg.Auth.AccessTokenTTL = time.Minute
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	// A unique user id per run keeps tokens distinct from other runs.
	userID := strings.ReplaceAll(t.Name(), "/", "_") + "-" + time.Now().Format("150405.000000000")
	ctx := t.Context()
	srv := newServer(&fakeAuth{user: &entity.User{ID: userID, Email: "token@example.com", CreatedAt: time.Now()}}, cfg, logger, rdb)
	login, err := srv.Login(ctx, authpb.LoginRequest_builder{Email: new("token@example.com"), Password: new("ignored")}.Build())
	require.NoError(t, err)
	token := login.GetToken()
	require.NotEmpty(t, token)
	t.Cleanup(func() { _ = rdb.Del(context.WithoutCancel(ctx), tokenKey(token), blacklistKey(token)).Err() })

	// issue stored the token with the configured TTL.
	stored, err := rdb.Get(ctx, tokenKey(token)).Result()
	require.NoError(t, err)
	assert.Equal(t, userID, stored)
	ttl, err := rdb.TTL(ctx, tokenKey(token)).Result()
	require.NoError(t, err)
	assert.Greater(t, ttl, 30*time.Second)
	assert.LessOrEqual(t, ttl, time.Minute)

	revoked, err := srv.tokens.revoked(ctx, token)
	require.NoError(t, err)
	assert.False(t, revoked)

	valid, err := srv.ValidateToken(ctx, authpb.ValidateTokenRequest_builder{Token: &token}.Build())
	require.NoError(t, err)
	assert.Equal(t, userID, valid.GetUser().GetId())

	_, err = srv.Logout(ctx, authpb.LogoutRequest_builder{Token: &token}.Build())
	require.NoError(t, err)

	revoked, err = srv.tokens.revoked(ctx, token)
	require.NoError(t, err)
	assert.True(t, revoked)

	exists, err := rdb.Exists(ctx, tokenKey(token)).Result()
	require.NoError(t, err)
	assert.Zero(t, exists, "revoke deletes the stored token")
	blTTL, err := rdb.TTL(ctx, blacklistKey(token)).Result()
	require.NoError(t, err)
	assert.Positive(t, blTTL, "blacklist entry lives for the token's remaining lifetime")

	_, err = srv.ValidateToken(ctx, authpb.ValidateTokenRequest_builder{Token: &token}.Build())
	require.ErrorIs(t, err, apperr.ErrInvalidToken)
}
