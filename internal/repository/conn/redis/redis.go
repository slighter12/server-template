package redis

import (
	"context"
	"fmt"

	"server-template/config"
	"server-template/internal/domain/lifecycle"

	"github.com/redis/go-redis/v9"
	redisLib "github.com/slighter12/go-lib/database/redis/cluster"
	"go.uber.org/fx"
)

// Params defines the required parameters.
type Params struct {
	fx.In
	fx.Lifecycle

	Config *config.Config
}

// New creates a Redis cluster client. Redis is optional: when the redis
// config section is missing or has no addresses it returns (nil, nil) and
// registers no lifecycle hooks; consumers must handle a nil client.
func New(params Params) (*redis.ClusterClient, error) {
	cfg := params.Config.Redis
	if cfg == nil || len(cfg.Address) == 0 {
		return nil, nil //nolint:nilnil // redis is optional; nil means disabled
	}

	client, err := redisLib.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create Redis client: %w", err)
	}

	params.Append(fx.Hook{
		OnStart: func(startCtx context.Context) error {
			ctx, cancel := context.WithTimeout(startCtx, lifecycle.DefaultTimeout)
			defer cancel()

			if err := client.Ping(ctx).Err(); err != nil {
				return fmt.Errorf("failed to ping Redis: %w", err)
			}

			return nil
		},
		OnStop: func(context.Context) error {
			return client.Close()
		},
	})

	return client, nil
}
