package execution

import (
	"blotato-comment-automation/internal/feature/conversation"
	"blotato-comment-automation/shared/kafka"
	"context"
	"encoding/json"
)

type Publisher struct{ transport *kafka.Publisher }

func NewPublisher(transport *kafka.Publisher) *Publisher { return &Publisher{transport: transport} }
func (p *Publisher) PublishActivation(ctx context.Context, a conversation.Activation) error {
	if err := a.Validate(); err != nil {
		return err
	}
	raw, err := json.Marshal(a)
	if err != nil {
		return err
	}
	return p.transport.Publish(ctx, kafka.RoutingKey(a.Platform, a.AccountID, a.ParticipantID), raw)
}
