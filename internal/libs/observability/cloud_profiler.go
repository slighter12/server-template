package observability

import (
	"context"
	"log/slog"

	"server-template/config"

	"cloud.google.com/go/profiler"
	"github.com/pkg/errors"
	"google.golang.org/api/option"
)

// loggerWriter is a bridge to redirect DebugLoggingOutput to the provided slog.Logger
type slogWriter struct {
	logger *slog.Logger
}

func newSlogWriter(logger *slog.Logger) *slogWriter {
	return &slogWriter{logger: logger}
}

// Write implements the io.Writer interface
func (lw *slogWriter) Write(p []byte) (n int, err error) {
	lw.logger.Debug(string(p))

	return len(p), nil
}

// NewCloudProfiler creates a new ProfilerService
func NewCloudProfiler(
	ctx context.Context,
	cnf *config.Config,
	logger *slog.Logger,
) error {
	if !cnf.Observability.CloudProfiler.Enable {
		return nil
	}

	profilerConfig := profiler.Config{
		ServiceVersion:     "1.0.0",
		ProjectID:          cnf.Observability.CloudProfiler.ProjectID,
		DebugLogging:       cnf.Env.Debug,
		DebugLoggingOutput: newSlogWriter(logger), // Redirect profiler debug logs to your logger

	}

	// Prepare client options
	var clientOptions []option.ClientOption
	if cnf.Env.Env == "local" && cnf.Observability.CloudProfiler.ServiceAccount != "" {
		clientOptions = append(clientOptions, option.WithCredentialsFile(cnf.Observability.CloudProfiler.ServiceAccount))
		logger.Info("Using local service account for Profiler",
			slog.String("service_account", cnf.Observability.CloudProfiler.ServiceAccount))
	}

	// Initialize the profiler
	if err := profiler.Start(profilerConfig, clientOptions...); err != nil {
		return errors.WithStack(err)
	}

	// Your application logic here
	logger.Info("Cloud Profiler started successfully",
		slog.String("service", profilerConfig.Service),
		slog.String("version", profilerConfig.ServiceVersion),
	)

	return nil
}
