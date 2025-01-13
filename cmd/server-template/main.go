package main

import (
	"context"
	"log/slog"

	"server-template/config"
	"server-template/internal/libs/logs"
	"server-template/internal/libs/mongo"
	"server-template/internal/libs/mysql"
	"server-template/internal/libs/observability"
	"server-template/internal/libs/redis"
	"server-template/internal/libs/rpc"
	"server-template/internal/repository"
	"server-template/internal/usecase"

	"go.uber.org/fx"
)

func main() {
	fx.New(
		injectInfra(),
		injectConn(),
		injectRepo(),
		injectUse(),
		fx.Invoke(
			observability.NewPyroscope,
			observability.NewTracer,
			observability.NewCloudProfiler,
			startServer,
		),
	).Run()
}

func injectInfra() fx.Option {
	return fx.Provide(
		config.New,
		logs.New,
		context.Background,
	)
}

func injectConn() fx.Option {
	return fx.Provide(
		mysql.New,
		redis.NewClusterClient,
		mongo.New,
		rpc.NewRPCClients,
	)
}

func injectRepo() fx.Option {
	return fx.Provide(
		repository.NewAuthRPC,
		repository.NewUserRepository,
	)
}

func injectUse() fx.Option {
	return fx.Provide(
		usecase.NewAuthUseCase,
	)
}

func startServer(ctx context.Context) error {
	// 啟動邏輯
	slog.Info("Starting server...")
	// 這裡整合你的 fx 邏輯，或者加載配置、啟動應用
	return nil
}
