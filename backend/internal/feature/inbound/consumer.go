package inbound

import (
	"context"
	"log/slog"
)

type Handler struct{ repository Repository }

func NewHandler(repository Repository) *Handler { return &Handler{repository: repository} }

func (h *Handler) Handle(ctx context.Context, value []byte) error {
	comment, err := decodeComment(value)
	if err != nil {
		return err
	}
	if *comment.Data.Own {
		return nil
	}
	inserted, err := h.repository.Record(ctx, comment)
	if err != nil {
		return err
	}
	slog.InfoContext(ctx, "comment received", "comment_id", comment.Data.CommentID, "duplicate", !inserted)
	return nil
}
