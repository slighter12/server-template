package mongo

import (
	"context"
	"fmt"

	"server-template/config"
	"server-template/internal/domain/lifecycle"
	"server-template/internal/repository/conn"

	mongoLib "github.com/slighter12/go-lib/database/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.uber.org/fx"
)

// Params defines the required parameters.
type Params struct {
	fx.In
	fx.Lifecycle

	Config *config.Config
}

// Result is the client map plus the default client.
type Result struct {
	fx.Out

	Clients map[string]*mongo.Client `name:"mongo_maps"`
	Default *mongo.Client            `name:"default_mongo"`
}

// New creates MongoDB clients. MongoDB is optional: with an empty config
// section it returns empty results and registers no lifecycle hooks.
func New(params Params) (Result, error) {
	result := Result{Clients: make(map[string]*mongo.Client)}
	if len(params.Config.Mongo) == 0 {
		return result, nil
	}

	for _, name := range conn.SortedNames(params.Config.Mongo) {
		client, err := mongoLib.New(params.Config.Mongo[name])
		if err != nil {
			return Result{Clients: make(map[string]*mongo.Client)}, fmt.Errorf("failed to create MongoDB client %s: %w", name, err)
		}

		params.Append(fx.Hook{
			OnStart: func(startCtx context.Context) error {
				ctx, cancel := context.WithTimeout(startCtx, lifecycle.DefaultTimeout)
				defer cancel()

				if err := client.Ping(ctx, nil); err != nil {
					return fmt.Errorf("failed to ping MongoDB %s: %w", name, err)
				}

				return nil
			},
			OnStop: func(ctx context.Context) error {
				return client.Disconnect(ctx)
			},
		})

		result.Clients[name] = client
	}
	result.Default = conn.Default(result.Clients)

	return result, nil
}
