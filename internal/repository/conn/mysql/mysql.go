package mysql

import (
	"context"

	"server-template/config"
	"server-template/internal/domain/lifecycle"

	"github.com/pkg/errors"
	mysqlLib "github.com/slighter12/go-lib/database/mysql"
	"go.uber.org/fx"
	"gorm.io/gorm"
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

	Clients map[string]*gorm.DB `name:"mysql_maps"`
	Default *gorm.DB            `name:"default_mysql"`
}

// New 創建 MySQL 客戶端映射
func New(params Params) (Result, error) {
	result := Result{
		Clients: make(map[string]*gorm.DB),
	}

	// 如果沒有配置 MySQL，返回空映射
	if len(params.Config.Mysql) == 0 {
		return result, nil
	}

	// 遍歷所有 MySQL 配置創建客戶端
	for name, cfg := range params.Config.Mysql {
		db, err := mysqlLib.New(cfg)
		if err != nil {
			return result, errors.Wrapf(err, "failed to create MySQL client: %s", name)
		}

		sqlDB, err := db.DB()
		if err != nil {
			return result, errors.Wrapf(err, "failed to get MySQL sql.DB: %s", name)
		}

		// 添加生命週期管理
		params.Lifecycle.Append(fx.Hook{
			OnStart: func(startCtx context.Context) error {
				ctx, cancel := context.WithTimeout(startCtx, lifecycle.DefaultTimeout)
				defer cancel()

				return sqlDB.PingContext(ctx)
			},
			OnStop: func(_ context.Context) error {
				return sqlDB.Close()
			},
		})

		result.Clients[name] = db

		// 設置默認客戶端
		if name == "main" {
			result.Default = db
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
