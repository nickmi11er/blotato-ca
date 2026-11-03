package kafka

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	kafkago "github.com/segmentio/kafka-go"
)

type Config struct {
	Brokers []string `yaml:"broker"`
	GroupID string   `yaml:"group_id"`
	Topic   string   `yaml:"topic"`
}
type Consumer struct{ reader *kafkago.Reader }
type MessageHandler func(context.Context, []byte) error

func NewConsumer(cfg Config) (*Consumer, error) {
	rc := kafkago.ReaderConfig{
		Brokers: cfg.Brokers, GroupID: cfg.GroupID, Topic: cfg.Topic, StartOffset: kafkago.FirstOffset,
		Dialer:      &kafkago.Dialer{Timeout: 10 * time.Second, DualStack: true},
		ErrorLogger: slog.NewLogLogger(slog.Default().Handler(), slog.LevelError),
	}
	if err := rc.Validate(); err != nil {
		return nil, err
	}
	return &Consumer{reader: kafkago.NewReader(rc)}, nil
}

func (c *Consumer) Close() error { return c.reader.Close() }

func (c *Consumer) Consume(ctx context.Context, handle MessageHandler) error {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			return fmt.Errorf("fetch: %w", err)
		}
		if err = handle(ctx, msg.Value); err != nil {
			return fmt.Errorf("process %s[%d] offset %d: %w", msg.Topic, msg.Partition, msg.Offset, err)
		}
		if err = c.tryCommit(ctx, msg); err != nil {
			return err
		}
	}
}

func (c *Consumer) tryCommit(ctx context.Context, msg kafkago.Message) error {
	const attempts = 3
	for attempt := 1; ; attempt++ {
		err := c.reader.CommitMessages(ctx, msg)
		if err == nil {
			return nil
		}
		if attempt == attempts {
			return fmt.Errorf("commit %s[%d] offset %d: %w", msg.Topic, msg.Partition, msg.Offset, err)
		}
		slog.WarnContext(ctx, "retrying offset commit", "attempt", attempt, "error", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt) * 500 * time.Millisecond):
		}
	}
}
