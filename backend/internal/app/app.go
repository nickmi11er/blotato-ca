package app

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"blotato-comment-automation/internal/config"
	"blotato-comment-automation/internal/di"
)

const shutdownTimeout = 10 * time.Second

type App struct {
	services *di.Services
}

func New(ctx context.Context, configPath string) (_ *App, err error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}
	services := di.NewServices(ctx, cfg)
	application := &App{services: services}
	defer func() {
		if err != nil {
			err = errors.Join(err, application.shutdown())
		}
	}()

	_, err = services.Inbound().Handler()
	if err != nil {
		return nil, err
	}
	_, err = services.Kafka().Consumer()
	if err != nil {
		return nil, err
	}
	return application, nil
}

func (a *App) Run(ctx context.Context) (err error) {
	defer func() { err = errors.Join(err, a.shutdown()) }()

	handler, err := a.services.Inbound().Handler()
	if err != nil {
		return err
	}
	consumer, err := a.services.Kafka().Consumer()
	if err != nil {
		return err
	}

	cfg := a.services.Config()
	slog.InfoContext(ctx, "starting consumer", "topic", cfg.Kafka.Topic, "group", cfg.Kafka.GroupID)
	err = consumer.Consume(ctx, handler.Handle)
	if errors.Is(err, context.Canceled) && ctx.Err() != nil {
		return nil
	}
	return err
}

func (a *App) shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	return a.services.Gc().Shutdown(ctx)
}
