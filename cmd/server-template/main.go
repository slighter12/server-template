package main

import (
	"context"
	"log/slog"
	"sync"

	"server-template/config"
	"server-template/internal/delivery/debug"
	"server-template/internal/delivery/grpc"
	"server-template/internal/delivery/http/http2"
	"server-template/internal/delivery/http/http3"
	"server-template/internal/delivery/http/router"
	"server-template/internal/delivery/http/server"
	"server-template/internal/domain/delivery"
	repo "server-template/internal/domain/repository"
	use "server-template/internal/domain/usecase"
	"server-template/internal/infrastructure/auth"
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

	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

// The template demonstrates "HTTP router -> gRPC service (loopback)".
//
//   - Pure router (no local business logic): delete injectRepo, injectUseCase,
//     injectGRPC (and the connections in injectConn you no longer need). The
//     HTTP use case then talks to a remote service through rpc.
//   - Pure service (gRPC only): delete injectHTTP.
func main() {
	fx.New(
		fx.WithLogger(newFxLogger),
		injectInfra(),
		injectConn(),
		injectRepo(),
		injectUseCase(),
		injectHTTP(),
		injectGRPC(),
		injectDebug(),
		fx.Invoke(
			logStartup,
			pyroscope.New,
			func(p otel.Params) error { return otel.New(context.Background(), p) },
			profiler.New,
			startServer,
		),
	).Run()
}

// newFxLogger routes fx's own lifecycle events to slog at debug level.
func newFxLogger(logger *slog.Logger) fxevent.Logger {
	l := &fxevent.SlogLogger{Logger: logger}
	l.UseLogLevel(slog.LevelDebug)

	return l
}

// injectInfra provides config and logging. Keep it.
func injectInfra() fx.Option {
	return fx.Provide(
		config.New,
		logs.New,
		// Adds trace_id/span_id to log records, only when OTel is enabled.
		func(cfg *config.Config) func(slog.Handler) slog.Handler {
			if !cfg.Observability.Otel.Enable {
				return nil
			}

			return otel.NewTraceHandler
		},
		auth.NewArgon2idHasher,
	)
}

// injectConn provides the connections. All of them are mandatory: a missing
// config section, a creation failure or a failed startup ping aborts the app.
// Delete the registration and the config section of the ones you do not use
// (and the repositories that depend on them).
func injectConn() fx.Option {
	return fx.Provide(
		mysql.New,
		postgres.New,
		// Consumers depend on the redis.Cmdable interface (the provider
		// returns a cluster client).
		fx.Annotate(redis.New, fx.As(new(goredis.Cmdable))),
		mongo.New,
	)
}

// injectRepo provides the persistence layer used by the gRPC side.
func injectRepo() fx.Option {
	return fx.Options(
		fx.Provide(
			fx.Annotate(
				repository.NewUserRepository,
				fx.ParamTags(`name:"default_postgres"`),
			),
			fx.Annotate(
				repository.NewTransactionManager,
				fx.ParamTags(`name:"default_postgres"`, ``),
				fx.As(new(repo.TransactionManager)),
			),
		),
		fx.Decorate(func(cfg *config.Config, base repo.UserRepository) repo.UserRepository {
			return repository.ProvideUserRepositoryProxy(cfg.Observability.Otel.Enable, base)
		}),
	)
}

// injectUseCase provides the business logic served by gRPC.
func injectUseCase() fx.Option {
	return fx.Options(
		fx.Provide(usecase.NewAuthUseCase),
		fx.Decorate(func(cfg *config.Config, base use.AuthUseCase) use.AuthUseCase {
			return usecase.ProvideAuthUseCaseProxy(cfg.Observability.Otel.Enable, base)
		}),
	)
}

// injectHTTP is the HTTP layer: router, echo server, h2c and HTTP/3 deliveries,
// and the HTTP use case that calls the gRPC service. Delete this group to get a
// pure gRPC service.
func injectHTTP() fx.Option {
	return fx.Options(
		fx.Provide(
			rpc.New, // loopback / remote gRPC clients, only needed by the HTTP side
			repository.NewAuthRPC,
			usecase.NewAuthHTTPUseCase,
			server.New,
			fx.Annotate(
				router.New,
				fx.As(new(server.RouteRegistrar)),
				fx.ResultTags(`group:"http_routes"`),
			),
			fx.Annotate(http2.New, fx.ResultTags(`group:"deliveries"`)),
			fx.Annotate(http3.New, fx.ResultTags(`group:"deliveries"`)),
		),
		fx.Decorate(func(cfg *config.Config, base use.AuthHTTPUseCase) use.AuthHTTPUseCase {
			return usecase.ProvideAuthHTTPUseCaseProxy(cfg.Observability.Otel.Enable, base)
		}),
	)
}

// injectGRPC is the gRPC layer. Delete this group to get a pure HTTP router.
func injectGRPC() fx.Option {
	return fx.Provide(
		fx.Annotate(grpc.NewGRPC, fx.ResultTags(`group:"deliveries"`)),
	)
}

// injectDebug is the optional debug server (pprof, flight recorder).
func injectDebug() fx.Option {
	return fx.Provide(
		fx.Annotate(debug.NewDebugServer, fx.ResultTags(`group:"deliveries"`)),
	)
}

type startServerParams struct {
	fx.In

	Lifecycle  fx.Lifecycle
	Shutdowner fx.Shutdowner
	Logger     *slog.Logger
	Deliveries []delivery.Delivery `group:"deliveries"`
}

// startServer runs every delivery in its own goroutine once the app has
// started. A Serve failure shuts the whole app down (exit code 1) so OnStop
// hooks still run. The context passed to Serve is canceled on OnStop.
func startServer(p startServerParams) {
	var wg sync.WaitGroup

	// Serve outlives OnStart's own (startup-timeout) context, so it gets a
	// dedicated one that OnStop cancels.
	ctx, cancel := context.WithCancel(context.Background())

	p.Lifecycle.Append(fx.Hook{
		OnStart: func(context.Context) error {
			for _, d := range p.Deliveries {
				wg.Go(func() {
					if err := d.Serve(ctx); err != nil {
						p.Logger.Error("delivery stopped with error", slog.Any("error", err))
						if sErr := p.Shutdowner.Shutdown(fx.ExitCode(1)); sErr != nil {
							p.Logger.Error("shutdown failed", slog.Any("error", sErr))
						}
					}
				})
			}

			return nil
		},
		OnStop: func(context.Context) error {
			// Deliveries stop through their own OnStop hooks (graceful
			// shutdown); the cancel only releases anything watching ctx.
			// Not waiting on wg here: those hooks run after this one.
			cancel()

			return nil
		},
	})
}
