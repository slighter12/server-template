package mysql

import (
	"context"
	"fmt"

	"server-template/config"
	"server-template/internal/domain/lifecycle"
	"server-template/internal/repository/conn"

	mysqlLib "github.com/slighter12/go-lib/database/mysql"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

// Params defines the required parameters.
type Params struct {
	fx.In
	fx.Lifecycle

	Config *config.Config
}

// Result is the connection map plus the default client.
type Result struct {
	fx.Out

	Clients map[string]*gorm.DB `name:"mysql_maps"`
	Default *gorm.DB            `name:"default_mysql"`
}

// New creates MySQL clients. MySQL is optional: with an empty config section
// it returns empty results and registers no lifecycle hooks.
func New(params Params) (Result, error) {
	result := Result{Clients: make(map[string]*gorm.DB)}
	if len(params.Config.Mysql) == 0 {
		return result, nil
	}

	for _, name := range conn.SortedNames(params.Config.Mysql) {
		db, err := mysqlLib.New(params.Config.Mysql[name])
		if err != nil {
			return Result{Clients: make(map[string]*gorm.DB)}, fmt.Errorf("failed to create MySQL client %s: %w", name, err)
		}

		sqlDB, err := db.DB()
		if err != nil {
			return Result{Clients: make(map[string]*gorm.DB)}, fmt.Errorf("failed to get MySQL sql.DB %s: %w", name, err)
		}

		params.Append(fx.Hook{
			OnStart: func(startCtx context.Context) error {
				ctx, cancel := context.WithTimeout(startCtx, lifecycle.DefaultTimeout)
				defer cancel()

				if err := sqlDB.PingContext(ctx); err != nil {
					return fmt.Errorf("failed to ping MySQL %s: %w", name, err)
				}

				return nil
			},
			OnStop: func(context.Context) error {
				return sqlDB.Close()
			},
		})

		result.Clients[name] = db
	}
	result.Default = conn.Default(result.Clients)

	return result, nil
}
