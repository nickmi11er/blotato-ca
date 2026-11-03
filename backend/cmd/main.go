package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"blotato-comment-automation/internal/app"
)

func main() {
	configPath := flag.String("config", "config.yml", "configuration file path")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app, err := app.New(ctx, *configPath)
	if err != nil {
		slog.Error("failed to create application", "error", err)
		os.Exit(1)
	}
	if err := app.Run(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			slog.InfoContext(ctx, "application stopped")
			return
		}
		slog.Error("application stopped", "error", err)
		os.Exit(1)
	}
}
