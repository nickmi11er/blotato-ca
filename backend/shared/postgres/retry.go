package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

const (
	serializationFailure = "40001"
	deadlockDetected     = "40P01"
)

// Retry a write only when it could not have committed.
func isSafeToReplay(err error) bool {
	if err == nil {
		return false
	}
	if pgconn.SafeToRetry(err) {
		return true
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == serializationFailure || pgErr.Code == deadlockDetected
	}
	return false
}

func withRetry(ctx context.Context, attempts int, retryable func(error) bool, fn func() error) error {
	backoff := 100 * time.Millisecond
	var err error
	for i := 0; i < attempts; i++ {
		if err = fn(); err == nil {
			return nil
		}
		if !retryable(err) {
			return err
		}
		if i == attempts-1 {
			break
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			backoff *= 2
		}
	}
	return err
}
