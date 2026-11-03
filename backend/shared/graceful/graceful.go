package graceful

import (
	"context"
	"errors"
	"log/slog"
	"sync"
)

type Graceful interface {
	Add(func(context.Context) error)
	Shutdown(context.Context) error
}

type graceful struct {
	mu   sync.Mutex
	fs   []func(context.Context) error
	once sync.Once
	done chan struct{}
	err  error
}

func New() Graceful { return &graceful{done: make(chan struct{})} }

// Add registers a resource cleanup during startup. Resources close in reverse order.
func (g *graceful) Add(f func(context.Context) error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.fs = append(g.fs, f)
}

func (g *graceful) Shutdown(ctx context.Context) error {
	g.once.Do(func() {
		g.mu.Lock()
		fs := g.fs
		g.fs = nil
		g.mu.Unlock()

		go func() {
			defer close(g.done)
			slog.InfoContext(ctx, "graceful shutdown starting")
			for i := len(fs) - 1; i >= 0; i-- {
				g.err = errors.Join(g.err, fs[i](ctx))
			}
			if g.err == nil {
				slog.InfoContext(ctx, "graceful shutdown successfully completed")
			}
		}()
	})

	select {
	case <-g.done:
		return g.err
	case <-ctx.Done():
		return ctx.Err()
	}
}
