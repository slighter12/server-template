package main

import (
	"context"
	"log/slog"
	"server-template/config"
	"server-template/internal/libs/logs"
	"server-template/internal/libs/pyroscope"

	"go.uber.org/fx"
)

func main() {
	fx.New(
		injectInfra(),
		fx.Provide(),
		fx.Invoke(
			pyroscope.NewPyroscope,
			startServer,
		),
	).Run()
}

func injectInfra() fx.Option {
	return fx.Provide(
		config.New,
		logs.New,
		context.Background,
		pyroscope.NewSlogAdapter,
	)
}

func startServer(ctx context.Context) error {
	// 啟動邏輯
	slog.Info("Starting server...")
	// 這裡整合你的 fx 邏輯，或者加載配置、啟動應用
	return nil
}
