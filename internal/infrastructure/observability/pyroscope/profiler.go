package pyroscope

import (
	"context"
	"fmt"
	"log/slog"

	"server-template/config"

	"github.com/grafana/pyroscope-go"
	"github.com/pkg/errors"
	"go.uber.org/fx"
)

// Params 定義 profiler 所需的參數
type Params struct {
	fx.In

	Lifecycle fx.Lifecycle
	Config    *config.Config
	Logger    *slog.Logger
}

// logger 用於適配 slog.Logger 以符合 pyroscope.Logger 接口
type logger struct {
	logger *slog.Logger
}

// newLogger 創建一個新的 logger
func newLogger(l *slog.Logger) logger {
	return logger{logger: l}
}

// Debugf 實現 pyroscope.Logger 的 Debugf 方法
func (l logger) Debugf(format string, args ...interface{}) {
	l.logger.Debug(fmt.Sprintf(format, args...))
}

// Infof 實現 pyroscope.Logger 的 Infof 方法
func (l logger) Infof(format string, args ...interface{}) {
	l.logger.Info(fmt.Sprintf(format, args...))
}

// Warnf 實現 pyroscope.Logger 的 Warnf 方法
func (l logger) Warnf(format string, args ...interface{}) {
	l.logger.Warn(fmt.Sprintf(format, args...))
}

// Errorf 實現 pyroscope.Logger 的 Errorf 方法
func (l logger) Errorf(format string, args ...interface{}) {
	l.logger.Error(fmt.Sprintf(format, args...))
}

// New 創建並初始化 Pyroscope profiler
func New(params Params) error {
	if !params.Config.Observability.Pyroscope.Enable {
		return nil
	}

	profiler, err := pyroscope.Start(pyroscope.Config{
		ApplicationName: params.Config.Env.ServiceName,
		ServerAddress:   params.Config.Observability.Pyroscope.URL,
		Logger:          newLogger(params.Logger),
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

	params.Lifecycle.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			return profiler.Stop()
		},
	})

	return nil
}
