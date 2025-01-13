package observability

import (
	"context"
	"fmt"
	"log/slog"

	"server-template/config"

	"github.com/grafana/pyroscope-go"
	"github.com/pkg/errors"
	"go.uber.org/fx"
)

// pyroscopeLogger 用於適配 slog.Logger 以符合 pyroscope.Logger 接口
type pyroscopeLogger struct {
	logger *slog.Logger
}

// newPyroscopeLogger 創建一個新的 pyroscopeLogger
func newPyroscopeLogger(logger *slog.Logger) pyroscopeLogger {
	return pyroscopeLogger{logger: logger}
}

// Debugf 實現 pyroscope.Logger 的 Debugf 方法
func (s pyroscopeLogger) Debugf(format string, args ...interface{}) {
	s.logger.Debug(fmt.Sprintf(format, args...))
}

// Infof 實現 pyroscope.Logger 的 Infof 方法
func (s pyroscopeLogger) Infof(format string, args ...interface{}) {
	s.logger.Info(fmt.Sprintf(format, args...))
}

// Warnf 實現 pyroscope.Logger 的 Warnf 方法
func (s pyroscopeLogger) Warnf(format string, args ...interface{}) {
	s.logger.Warn(fmt.Sprintf(format, args...))
}

// Errorf 實現 pyroscope.Logger 的 Errorf 方法
func (s pyroscopeLogger) Errorf(format string, args ...interface{}) {
	s.logger.Error(fmt.Sprintf(format, args...))
}

func NewPyroscope(
	lc fx.Lifecycle,
	cnf *config.Config,
	logger *slog.Logger,
) error {
	if !cnf.Observability.Pyroscope.Enable {
		return nil
	}

	profiler, err := pyroscope.Start(pyroscope.Config{
		ApplicationName: cnf.Env.ServiceName,
		ServerAddress:   cnf.Observability.Pyroscope.URL,
		Logger:          newPyroscopeLogger(logger),
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
		return errors.WithStack(err)
	}

	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			return profiler.Stop()
		},
	})

	return nil
}
