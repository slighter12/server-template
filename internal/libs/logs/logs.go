package logs

import (
	"log/slog"
	"os"
	"server-template/config"

	"github.com/pkg/errors"
)

func New(cfg *config.Config) (*slog.Logger, error) {
	// Parse initial log level from config
	level, err := parseLogLevel(cfg.Env.Log.Level)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to parse log level")
	}

	// Initialize slog logger with JSON format and specified log level
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger, nil
}

func parseLogLevel(level string) (slog.Level, error) {
	switch level {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, errors.Errorf("unknown log level: %s", level)
	}
}
