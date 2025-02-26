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
	"server-template/internal/libs/logs"
	"server-template/internal/libs/mongo"
	"server-template/internal/libs/mysql"
	"server-template/internal/libs/observability"
	"server-template/internal/libs/postgres"
	"server-template/internal/libs/redis"
	"server-template/internal/libs/rpc"
	"server-template/internal/repository"
	"server-template/internal/usecase"

	"github.com/pkg/errors"
	"go.uber.org/fx"
	"gorm.io/gorm"
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
	return fx.Options(
		fx.Provide(
			// MySQL connections
			func(cfg *config.Config, lc fx.Lifecycle) (map[string]*gorm.DB, error) {
				dbMap := make(map[string]*gorm.DB)
				for name, dbCfg := range cfg.Mysql {
					dbConn, err := mysql.New(lc, dbCfg)
					if err != nil {
						slog.Error("Failed to create MySQL connection", slog.String("name", name), slog.Any("error", err))

						return nil, errors.Wrap(err, "mysql.New")
					}
					dbMap[name] = dbConn
				}

				return dbMap, nil
			},

			// PostgreSQL connections
			func(cfg *config.Config, lc fx.Lifecycle) (map[string]*gorm.DB, error) {
				pgMap := make(map[string]*gorm.DB)
				for name, dbCfg := range cfg.Postgres {
					dbConn, err := postgres.NewWithPgx(lc, dbCfg)
					if err != nil {
						slog.Error("Failed to create PostgreSQL connection", slog.String("name", name), slog.Any("error", err))

						return nil, errors.Wrap(err, "postgres.New")
					}
					pgMap[name] = dbConn
				}

				return pgMap, nil
			},

			// Default MySQL connection
			fx.Annotate(
				func(dbMap map[string]*gorm.DB) (*gorm.DB, error) {
					db, ok := dbMap["main"]
					if !ok {
						return nil, errors.New("main MySQL database not found")
					}

					return db, nil
				},
				fx.ResultTags(`name:"default_mysql"`),
			),

			// Default PostgreSQL connection
			fx.Annotate(
				func(pgMap map[string]*gorm.DB) (*gorm.DB, error) {
					db, ok := pgMap["main"]
					if !ok {
						return nil, errors.New("main PostgreSQL database not found")
					}

					return db, nil
				},
				fx.ResultTags(`name:"default_postgres"`),
			),
		),
		fx.Provide(
			redis.NewClusterClient,
			mongo.New,
			rpc.NewRPCClients,
		),
	)
}

func injectRepo() fx.Option {
	return fx.Options(
		fx.Provide(
			repository.NewAuthRPC,
			fx.Annotate(
				repository.NewUserRepository,
				fx.ParamTags(`name:"default"`),
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
