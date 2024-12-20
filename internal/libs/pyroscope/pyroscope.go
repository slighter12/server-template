package pyroscope

import (
	"context"
	"server-template/config"

	"github.com/grafana/pyroscope-go"
	"go.uber.org/fx"
)

func NewPyroscope(
	lc fx.Lifecycle,
	cnf *config.Config,
	logger *slogAdapter,
) error {
	if !cnf.Observability.Pyroscope.Enable {
		return nil
	}

	profiler, err := pyroscope.Start(pyroscope.Config{
		ApplicationName: cnf.Env.ServiceName,
		ServerAddress:   cnf.Observability.Pyroscope.URL,
		Logger:          logger,
		ProfileTypes: []pyroscope.ProfileType{
			pyroscope.ProfileCPU,
			pyroscope.ProfileInuseObjects,
			pyroscope.ProfileAllocObjects,
			pyroscope.ProfileInuseSpace,
			pyroscope.ProfileAllocSpace,
			pyroscope.ProfileGoroutines,
			pyroscope.ProfileMutexCount,
			pyroscope.ProfileMutexDuration,
			pyroscope.ProfileBlockCount,
			pyroscope.ProfileBlockDuration,
		},
	})
	if err != nil {
		return err
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return profiler.Stop()
		},
	})

	return nil
}
