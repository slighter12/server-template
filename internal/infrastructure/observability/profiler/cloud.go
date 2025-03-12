package profiler

import (
	"log/slog"

	"server-template/config"

	"cloud.google.com/go/profiler"
	"github.com/pkg/errors"
	"go.uber.org/fx"
	"google.golang.org/api/option"
)

// Params 定義 profiler 所需的參數
type Params struct {
	fx.In

	Config *config.Config
	Logger *slog.Logger
}

// slogWriter 用於將 DebugLoggingOutput 重定向到 slog.Logger
type slogWriter struct {
	logger *slog.Logger
}

func newSlogWriter(logger *slog.Logger) *slogWriter {
	return &slogWriter{logger: logger}
}

// Write 實現 io.Writer 接口
func (lw *slogWriter) Write(p []byte) (n int, err error) {
	lw.logger.Debug(string(p))

	return len(p), nil
}

// New 創建並初始化 Cloud Profiler
func New(params Params) error {
	if !params.Config.Observability.CloudProfiler.Enable {
		return nil
	}

	profilerConfig := profiler.Config{
		ServiceVersion:     "1.0.0",
		ProjectID:          params.Config.Observability.CloudProfiler.ProjectID,
		DebugLogging:       params.Config.Env.Debug,
		DebugLoggingOutput: newSlogWriter(params.Logger),
	}

	// 準備客戶端選項
	var clientOptions []option.ClientOption
	if params.Config.Env.Env == "local" && params.Config.Observability.CloudProfiler.ServiceAccount != "" {
		clientOptions = append(clientOptions, option.WithCredentialsFile(params.Config.Observability.CloudProfiler.ServiceAccount))
		params.Logger.Info("Using local service account for Profiler",
			slog.String("service_account", params.Config.Observability.CloudProfiler.ServiceAccount))
	}

	// 初始化 profiler
	if err := profiler.Start(profilerConfig, clientOptions...); err != nil {
		return errors.WithStack(err)
	}

	params.Logger.Info("Cloud Profiler started successfully",
		slog.String("service", profilerConfig.Service),
		slog.String("version", profilerConfig.ServiceVersion),
	)

	return nil
}
