package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"blotato-comment-automation/internal/config"
	"blotato-comment-automation/internal/di"

	"golang.org/x/sync/errgroup"
)

const shutdownTimeout = 10 * time.Second

type App struct{ services *di.Services }

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
	if _, err = services.Inbound().Handler(); err != nil {
		return nil, err
	}
	if _, err = services.Kafka().Consumer(); err != nil {
		return nil, err
	}
	// Capture rules-topic offsets before any comment processing or API writes.
	if _, err = services.Automation().Changes(); err != nil {
		return nil, err
	}
	if _, err = services.Automation().HTTP(); err != nil {
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
	cache, err := a.services.Automation().Cache()
	if err != nil {
		return err
	}
	changes, err := a.services.Automation().Changes()
	if err != nil {
		return err
	}
	server, err := a.services.Automation().HTTP()
	if err != nil {
		return err
	}
	group, runCtx := errgroup.WithContext(ctx)
	group.Go(func() error { return cache.Run(runCtx) })
	group.Go(func() error { return changes.Consume(runCtx) })
	group.Go(func() error { return changes.Run(runCtx) })
	group.Go(func() error { return consumer.Consume(runCtx, handler.Handle) })
	group.Go(func() error {
		err := server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	})
	group.Go(func() error {
		<-runCtx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			_ = server.Close()
			return err
		}
		return nil
	})
	slog.InfoContext(ctx, "starting automation backend", "topic", a.services.Config().Kafka.Topic, "http", server.Addr)
	err = group.Wait()
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
