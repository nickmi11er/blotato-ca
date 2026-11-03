package inbound

import (
	"context"

	inboundDB "blotato-comment-automation/internal/db/sqlcDB/inbound"
)

type Repository interface {
	Record(context.Context, Comment) (bool, error)
}

type repository struct{ q *inboundDB.Queries }

func NewRepository(database inboundDB.DBTX) Repository {
	return &repository{q: inboundDB.New(database)}
}

func (r *repository) Record(ctx context.Context, c Comment) (bool, error) {
	rows, err := r.q.RecordComment(ctx, inboundDB.RecordCommentParams{
		Platform:  c.Platform,
		AccountID: c.AccountID,
		PostID:    c.Data.PostID,
		CommentID: c.Data.CommentID,
		EventType: c.Type,
	})
	return rows == 1, err
}
