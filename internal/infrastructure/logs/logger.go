package logs

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"server-template/config"

	"go.uber.org/fx"
)

// Params defines the parameters required for the logger.
type Params struct {
	fx.In

	Config *config.Config
	// WrapHandler optionally decorates the base handler (for example to add
	// trace ids). It is applied after the text/JSON handler is built.
	WrapHandler func(slog.Handler) slog.Handler `optional:"true"`
}

// New creates the slog.Logger and installs it as the process default.
// Output is text when log.pretty is set, JSON otherwise.
func New(params Params) (*slog.Logger, error) {
	level, err := parseLogLevel(params.Config.Env.Log.Level)
	if err != nil {
		return nil, err
	}

	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if params.Config.Env.Log.Pretty {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}
	if params.WrapHandler != nil {
		handler = params.WrapHandler(handler)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger, nil
}

// parseLogLevel converts a string log level to slog.Level.
func parseLogLevel(level string) (slog.Level, error) {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unknown log level: %s", level)
	}
}
