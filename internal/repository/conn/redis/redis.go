package redis

import (
	"context"

	"server-template/config"
	"server-template/internal/domain/lifecycle"

	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	redisLib "github.com/slighter12/go-lib/database/redis/cluster"
	"go.uber.org/fx"
)

// Params 定義所需的參數
type Params struct {
	fx.In
	fx.Lifecycle

	Config *config.Config
}

// New 創建一個新的 Redis 集群客戶端
func New(params Params) (*redis.ClusterClient, error) {
	if params.Config == nil {
		return nil, errors.New("redis configuration is required")
	}

	client := redisLib.New(params.Config.Redis)

	// 添加生命週期管理
	params.Lifecycle.Append(fx.Hook{
		OnStart: func(startCtx context.Context) error {
			ctx, cancel := context.WithTimeout(startCtx, lifecycle.DefaultTimeout)
			defer cancel()

			return client.Ping(ctx).Err()
		},
		OnStop: func(ctx context.Context) error {
			return client.Close()
		},
	})

	return client, nil
}
