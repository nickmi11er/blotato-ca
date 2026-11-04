package ingress

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"unicode/utf8"

	"blotato-comment-automation/internal/feature/conversation"
	"blotato-comment-automation/internal/feature/inbound"
	"blotato-comment-automation/shared/kafka"
	kafkago "github.com/segmentio/kafka-go"
)

type Factory struct {
	comments *inbound.Handler
	source   conversation.Source
	cfg      conversation.Config
}

func NewFactory(comments *inbound.Handler, source conversation.Source, cfg conversation.Config) *Factory {
	return &Factory{comments: comments, source: source, cfg: cfg}
}
func (f *Factory) Partition(count int) kafka.PartitionHandler {
	cfg := f.cfg
	cfg.MaxBytes = max(1, cfg.MaxBytes/max(1, count))
	router := conversation.NewRouter(f.source, cfg)
	return kafka.PartitionHandler{Idle: router.Prune, Handle: func(ctx context.Context, message kafkago.Message) error {
		if len(message.Value) > 1<<20 || !utf8.Valid(message.Value) {
			return errors.New("invalid event payload")
		}
		var envelope struct {
			Type          string `json:"type"`
			Platform      string `json:"platform"`
			AccountID     string `json:"account_id"`
			ParticipantID string `json:"participant_id"`
			Data          struct {
				AuthorID      string `json:"author_id"`
				ParticipantID string `json:"participant_id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(message.Value, &envelope); err != nil {
			return err
		}
		participant := envelope.ParticipantID
		switch envelope.Type {
		case "CommentReceived":
			participant = envelope.Data.AuthorID
		case "MessageReceived":
			participant = envelope.Data.ParticipantID
		case "ActivateConversation":
		default:
			return errors.New("unsupported inbound event type")
		}
		if !bytes.Equal(message.Key, kafka.RoutingKey(envelope.Platform, envelope.AccountID, participant)) {
			return errors.New("event key must use canonical JSON [platform,account_id,participant_id]")
		}
		switch envelope.Type {
		case "CommentReceived":
			return f.comments.Handle(ctx, message.Value)
		case "MessageReceived":
			dm, err := conversation.DecodeMessage(message.Value)
			if err != nil {
				return err
			}
			if err = router.Message(ctx, dm); err != nil {
				return retryable{err}
			}
		case "ActivateConversation":
			var a conversation.Activation
			if err := json.Unmarshal(message.Value, &a); err != nil {
				return err
			}
			if err := a.Validate(); err != nil {
				return err
			}
			if err := router.Activate(ctx, a); err != nil {
				return retryable{err}
			}
		}
		return nil
	}}
}

type retryable struct{ error }

func (e retryable) Unwrap() error   { return e.error }
func (e retryable) Retryable() bool { return true }
