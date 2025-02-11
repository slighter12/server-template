package main

import (
	"context"
	"log/slog"

	"server-template/config"
	repo "server-template/internal/domain/repository"
	use "server-template/internal/domain/usecase"
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
	return fx.Options(
		fx.Provide(
			repository.NewAuthRPC,
			repository.NewUserRepository,
		),
		fx.Decorate(func(cfg *config.Config, base repo.UserRepository) repo.UserRepository {
			return repository.ProvideUserRepositoryProxy(cfg.Observability.Otel.Enable, base)
		}),
	)
}

func injectUse() fx.Option {
	return fx.Options(
		fx.Provide(
			usecase.NewAuthUseCase,
		),
		fx.Decorate(func(cfg *config.Config, base use.AuthUseCase) use.AuthUseCase {
			return usecase.ProvideAuthUseCaseProxy(cfg.Observability.Otel.Enable, base)
		}),
	)
}

func startServer(lc fx.Lifecycle, ctx context.Context) error {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			slog.Info("Starting server...")

			return nil
		},
		OnStop: func(ctx context.Context) error {
			slog.Info("Stopping server...")

			return nil
		},
	})

	// 這裡整合你的 fx 邏輯，或者加載配置、啟動應用
	return nil
}
