package mongo

import (
	"context"

	"server-template/config"
	"server-template/internal/domain/lifecycle"

	"github.com/pkg/errors"
	mongoLib "github.com/slighter12/go-lib/database/mongo"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/fx"
)

// Params 定義所需的參數
type Params struct {
	fx.In
	fx.Lifecycle

	Config *config.Config
}

// Result 定義返回的連線映射
type Result struct {
	fx.Out

	Clients map[string]*mongo.Client `name:"mongo_maps"`
	Default *mongo.Client            `name:"default_mongo"`
}

// New 創建 MongoDB 客戶端映射
func New(params Params) (Result, error) {
	result := Result{
		Clients: make(map[string]*mongo.Client),
	}

	// 如果沒有配置 MongoDB，返回空映射
	if len(params.Config.Mongo) == 0 {
		return result, nil
	}

	// 遍歷所有 MongoDB 配置創建客戶端
	for name, cfg := range params.Config.Mongo {
		client, err := mongoLib.New(context.Background(), cfg)
		if err != nil {
			return result, errors.Wrapf(err, "failed to create MongoDB client: %s", name)
		}

		// 添加生命週期管理
		params.Lifecycle.Append(fx.Hook{
			OnStart: func(startCtx context.Context) error {
				ctx, cancel := context.WithTimeout(startCtx, lifecycle.DefaultTimeout)
				defer cancel()

				return client.Ping(ctx, nil)
			},
			OnStop: func(ctx context.Context) error {
				return client.Disconnect(ctx)
			},
		})

		result.Clients[name] = client

		// 設置默認客戶端
		if name == "main" {
			result.Default = client
		}
	}

	// 確保有默認客戶端
	if result.Default == nil && len(result.Clients) > 0 {
		// 如果沒有 "main" 客戶端，使用第一個作為默認
		for _, client := range result.Clients {
			result.Default = client

			break
		}
	}

	return result, nil
}
