package kafka

import (
	"context"
	"errors"
	"testing"
)

type transientError struct{}

func (transientError) Error() string   { return "temporary failure" }
func (transientError) Retryable() bool { return true }
func TestRetryKeepsSameRecordUntilHandled(t *testing.T) {
	calls := 0
	err := retryHandle(t.Context(), func(_ context.Context, value []byte) error {
		calls++
		if string(value) != "record" {
			t.Fatal("changed record")
		}
		if calls == 1 {
			return transientError{}
		}
		return nil
	}, []byte("record"))
	if err != nil || calls != 2 {
		t.Fatal(calls, err)
	}
}
func TestRetryStopsOnCancellationOrPermanentError(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	err := retryHandle(ctx, func(context.Context, []byte) error { cancel(); return transientError{} }, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	permanent := errors.New("invalid event")
	calls := 0
	err = retryHandle(t.Context(), func(context.Context, []byte) error { calls++; return permanent }, nil)
	if !errors.Is(err, permanent) || calls != 1 {
		t.Fatal(calls, err)
	}
}
