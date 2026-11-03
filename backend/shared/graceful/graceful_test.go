package graceful_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"blotato-comment-automation/shared/graceful"
)

func TestShutdownClosesDependenciesOnceAndPreservesErrors(t *testing.T) {
	g := graceful.New()
	databaseErr := errors.New("database close failed")
	consumerErr := errors.New("consumer close failed")
	var closed []string
	g.Add(func(context.Context) error {
		closed = append(closed, "database")
		return databaseErr
	})
	g.Add(func(context.Context) error {
		closed = append(closed, "consumer")
		return consumerErr
	})

	for range 2 {
		err := g.Shutdown(t.Context())
		if !errors.Is(err, databaseErr) || !errors.Is(err, consumerErr) {
			t.Fatalf("shutdown lost a cleanup error: %v", err)
		}
	}
	if !slices.Equal(closed, []string{"consumer", "database"}) {
		t.Fatalf("resources closed in the wrong order or more than once: %v", closed)
	}
}

func TestShutdownDeadlineBoundsBlockingCleanup(t *testing.T) {
	g := graceful.New()
	release := make(chan struct{})
	defer close(release)
	started := make(chan context.Context, 1)
	g.Add(func(ctx context.Context) error {
		started <- ctx
		<-release // A driver Close method may ignore its context.
		return nil
	})
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- g.Shutdown(ctx) }()

	select {
	case cleanupCtx := <-started:
		if deadline, ok := cleanupCtx.Deadline(); !ok || deadline.IsZero() {
			t.Fatal("cleanup did not receive a deadline")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cleanup did not start")
	}
	select {
	case err := <-result:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("got %v, want deadline exceeded", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown did not return when its deadline elapsed")
	}
}
