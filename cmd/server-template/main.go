package main

import (
	"context"
	"log/slog"
	"os"

	"server-template/config"
	"server-template/internal/delivery/grpc"
	"server-template/internal/delivery/http/http2"
	"server-template/internal/delivery/http/http3"
	"server-template/internal/domain/delivery"
	repo "server-template/internal/domain/repository"
	use "server-template/internal/domain/usecase"
	"server-template/internal/infrastructure/logs"
	"server-template/internal/infrastructure/observability/otel"
	"server-template/internal/infrastructure/observability/profiler"
	"server-template/internal/infrastructure/observability/pyroscope"
	"server-template/internal/infrastructure/rpc"
	"server-template/internal/repository"
	"server-template/internal/repository/conn/mongo"
	"server-template/internal/repository/conn/mysql"
	"server-template/internal/repository/conn/postgres"
	"server-template/internal/repository/conn/redis"
	"server-template/internal/usecase"

	"go.uber.org/fx"
)

type startServerParams struct {
	fx.In
	fx.Lifecycle

	Deliveries []delivery.Delivery `group:"deliveries"`
}

func main() {
	fx.New(
		injectInfra(),
		injectConn(),
		injectRepo(),
		injectUse(),
		injectDelivery(),
		fx.Invoke(
			pyroscope.New,
			otel.New,
			profiler.New,
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
	return fx.Options(
		fx.Provide(
			// 創建數據庫連線
			mysql.New,
			postgres.New,
			redis.New,
			mongo.New,
			rpc.New,
		),
	)
}

func injectRepo() fx.Option {
	return fx.Options(
		fx.Provide(
			repository.NewAuthRPC,
			fx.Annotate(
				repository.NewUserRepository,
				fx.ParamTags(`name:"default_postgres"`),
			),
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
			usecase.NewAuthHTTPUseCase,
		),
		fx.Decorate(func(cfg *config.Config, base use.AuthUseCase) use.AuthUseCase {
			return usecase.ProvideAuthUseCaseProxy(cfg.Observability.Otel.Enable, base)
		}),
	)
}

func injectDelivery() fx.Option {
	return fx.Options(
		fx.Provide(
			fx.Annotate(
				http2.NewHTTP2,
				fx.ResultTags(`group:"deliveries"`),
			),
			fx.Annotate(
				http3.NewHTTP3,
				fx.ResultTags(`group:"deliveries"`),
			),
			fx.Annotate(
				grpc.NewGRPC,
				fx.ResultTags(`group:"deliveries"`),
			),
		),
	)
}

func startServer(ctx context.Context, params startServerParams) {
	for _, delivery := range params.Deliveries {
		go func() {
			if err := delivery.Serve(ctx); err != nil {
				slog.Error("Failed to start server", slog.Any("error", err))
				os.Exit(1)
			}
		}()
	}
}
