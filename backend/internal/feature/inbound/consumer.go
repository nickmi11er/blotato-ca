package inbound

import (
	"context"
	"errors"
	"log/slog"

	"blotato-comment-automation/internal/feature/matching"
)

type Handler struct {
	repository Repository
	matcher    Matcher
}

func NewHandler(repository Repository, matcher Matcher) *Handler {
	return &Handler{repository: repository, matcher: matcher}
}
func (h *Handler) Handle(ctx context.Context, value []byte) error {
	comment, err := decodeComment(value)
	if err != nil {
		return err
	}
	if *comment.Data.Own {
		return nil
	}
	matches, err := h.matcher.Match(ctx, comment.Platform, comment.AccountID, comment.Data.PostID, *comment.Data.Text)
	if err != nil {
		if errors.Is(err, matching.ErrPlatform) {
			return err
		}
		return retryableError{err}
	}
	if len(matches) == 0 {
		return nil
	}
	inserted, err := h.repository.Record(ctx, comment, matches)
	if err != nil {
		return retryableError{err}
	}
	slog.InfoContext(ctx, "comment matched", "comment_id", comment.Data.CommentID, "automations", len(matches), "duplicate", !inserted)
	return nil
}

// Infrastructure failures leave the same Kafka record pending for a bounded-backoff retry.
type retryableError struct{ error }

func (e retryableError) Unwrap() error   { return e.error }
func (e retryableError) Retryable() bool { return true }
