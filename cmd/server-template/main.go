package main

import (
	"context"
	"log/slog"

	"server-template/config"
	"server-template/internal/delivery/grpc"
	"server-template/internal/delivery/http"
	"server-template/internal/domain/delivery"
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
		injectDelivery(),
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

func injectDelivery() fx.Option {
	return fx.Options(
		fx.Provide(
			http.NewHTTP,
			grpc.NewGRPC,
			fx.Annotate(
				func(http, grpc delivery.Delivery) []delivery.Delivery {
					return []delivery.Delivery{http, grpc}
				},
				fx.ParamTags(`group:"delivery"`, `group:"delivery"`),
				fx.ResultTags(`group:"deliveries"`),
			),
		),
	)
}

func startServer(lc fx.Lifecycle, ctx context.Context, deliveries []delivery.Delivery) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			for _, d := range deliveries {
				d.Serve(lc, ctx)
			}

			return nil
		},
		OnStop: func(ctx context.Context) error {
			slog.Info("Stopping server...")

			return nil
		},
	})
}
