package inbound

import (
	"encoding/json"
	"errors"
	"time"
)

type Comment struct {
	Type       string    `json:"type"`
	Version    int       `json:"schema_version"`
	Platform   string    `json:"platform"`
	AccountID  string    `json:"account_id"`
	OccurredAt time.Time `json:"occurred_at"`
	Data       struct {
		PostID    string  `json:"post_id"`
		CommentID string  `json:"comment_id"`
		AuthorID  string  `json:"author_id"`
		Own       *bool   `json:"is_own_comment"`
		Text      *string `json:"text"`
	} `json:"data"`
}

func decodeComment(value []byte) (Comment, error) {
	var c Comment
	if err := json.Unmarshal(value, &c); err != nil {
		return c, err
	}
	if c.Type != "CommentReceived" || c.Version != 1 {
		return c, errors.New("unsupported event type or schema version")
	}
	if c.Platform == "" || c.AccountID == "" || c.OccurredAt.IsZero() ||
		c.Data.PostID == "" || c.Data.CommentID == "" || c.Data.AuthorID == "" ||
		c.Data.Own == nil || c.Data.Text == nil {
		return c, errors.New("missing required comment fields")
	}
	return c, nil
}
