package di

import (
	"context"

	"blotato-comment-automation/shared/kafka"
)

type KafkaModule struct {
	*Services

	consumer  *kafka.Consumer
	rules     *kafka.Broadcast
	publisher *kafka.Publisher
}

func (s *Services) Kafka() *KafkaModule {
	if s.kafka == nil {
		s.kafka = &KafkaModule{Services: s}
	}
	return s.kafka
}

func (m *KafkaModule) Consumer() (*kafka.Consumer, error) {
	if m.consumer != nil {
		return m.consumer, nil
	}
	consumer, err := kafka.NewConsumer(m.Config().Kafka)
	if err != nil {
		return nil, err
	}
	m.Gc().Add(func(context.Context) error { return consumer.Close() })
	m.consumer = consumer
	return consumer, nil
}

func (m *KafkaModule) Rules() (*kafka.Broadcast, error) {
	if m.rules != nil {
		return m.rules, nil
	}
	cfg := m.Config().Kafka
	rules, err := kafka.NewBroadcast(m.Ctx(), cfg.Brokers, cfg.RulesTopic)
	if err != nil {
		return nil, err
	}
	m.Gc().Add(func(context.Context) error { return rules.Close() })
	m.rules = rules
	return rules, nil
}

func (m *KafkaModule) Publisher() (*kafka.Publisher, error) {
	if m.publisher != nil {
		return m.publisher, nil
	}
	publisher, err := kafka.NewPublisher(m.Config().Kafka)
	if err != nil {
		return nil, err
	}
	m.Gc().Add(func(context.Context) error { return publisher.Close() })
	m.publisher = publisher
	return publisher, nil
}
