package conversation

import (
	"blotato-comment-automation/internal/feature/automation"
	"context"
	"encoding/json"
	"errors"
	"time"
	"unicode/utf8"
)

var ErrStale = errors.New("conversation route changed")

type Route struct {
	SessionID string
	Epoch     int64
	Platform  string
}
type Message struct {
	Type       string    `json:"type"`
	Version    int       `json:"schema_version"`
	Platform   string    `json:"platform"`
	AccountID  string    `json:"account_id"`
	OccurredAt time.Time `json:"occurred_at"`
	Data       struct {
		ParticipantID  string  `json:"participant_id"`
		ConversationID string  `json:"conversation_id"`
		MessageID      string  `json:"message_id"`
		ReplyTo        string  `json:"reply_to_message_id,omitempty"`
		Own            *bool   `json:"is_own_message"`
		Text           *string `json:"text"`
	} `json:"data"`
}

func DecodeMessage(raw []byte) (Message, error) {
	var m Message
	if len(raw) > 1<<20 || !utf8.Valid(raw) {
		return m, errors.New("invalid DM payload")
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return m, err
	}
	if m.Type != "MessageReceived" || m.Version != 1 || m.OccurredAt.IsZero() || m.Data.Own == nil || m.Data.Text == nil || len(*m.Data.Text) > 64<<10 {
		return m, errors.New("invalid DM fields")
	}
	for _, id := range []string{m.AccountID, m.Platform, m.Data.ParticipantID, m.Data.ConversationID, m.Data.MessageID} {
		if !automation.ValidID(id) {
			return m, errors.New("invalid DM identity")
		}
	}
	return m, nil
}

type Activation struct {
	Type          string `json:"type"`
	Version       int    `json:"schema_version"`
	Platform      string `json:"platform"`
	AccountID     string `json:"account_id"`
	ParticipantID string `json:"participant_id"`
	SessionID     string `json:"session_id"`
	Epoch         int64  `json:"epoch"`
}

func (a Activation) Validate() error {
	if a.Type != "ActivateConversation" || a.Version != 1 || a.Epoch < 1 {
		return errors.New("invalid activation")
	}
	for _, id := range []string{a.Platform, a.AccountID, a.ParticipantID, a.SessionID} {
		if !automation.ValidID(id) {
			return errors.New("invalid activation identity")
		}
	}
	return nil
}

type Source interface {
	Route(context.Context, string, string) (Route, error)
	Accept(context.Context, Message, Route) error
	Activate(context.Context, Activation) error
}
