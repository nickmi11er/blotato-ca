package inbound

import (
	"encoding/json"
	"errors"
	"time"
	"unicode/utf8"

	"blotato-comment-automation/internal/feature/automation"
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
	if len(value) > 1<<20 || !utf8.Valid(value) {
		return c, errors.New("comment payload exceeds size limit or is not UTF-8")
	}
	if err := json.Unmarshal(value, &c); err != nil {
		return c, err
	}
	if c.Type != "CommentReceived" || c.Version != 1 {
		return c, errors.New("unsupported event type or schema version")
	}
	if !automation.ValidID(c.Platform) || !automation.ValidID(c.AccountID) || c.OccurredAt.IsZero() ||
		!automation.ValidID(c.Data.PostID) || !automation.ValidID(c.Data.CommentID) || !automation.ValidID(c.Data.AuthorID) ||
		c.Data.Own == nil || c.Data.Text == nil {
		return c, errors.New("missing required comment fields")
	}
	if len(*c.Data.Text) > 64<<10 {
		return c, errors.New("comment text exceeds 64 KiB")
	}
	return c, nil
}
