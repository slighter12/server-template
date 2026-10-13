package main

import (
	"log/slog"
	"runtime"
	"runtime/debug"
)

// Version is injected at build time:
//
//	go build -ldflags "-X main.Version=v1.2.3" ./cmd/server-template
var Version = "dev"

// buildInfo returns the VCS details the Go toolchain embeds in the binary.
func buildInfo() (revision, buildTime string, modified bool) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", "", false
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.time":
			buildTime = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}

	return revision, buildTime, modified
}

// logStartup writes the single startup line with version information.
func logStartup(logger *slog.Logger) {
	revision, buildTime, modified := buildInfo()
	logger.Info("starting server-template",
		slog.String("version", Version),
		slog.String("revision", revision),
		slog.String("build_time", buildTime),
		slog.Bool("modified", modified),
		slog.String("go_version", runtime.Version()),
	)
}
