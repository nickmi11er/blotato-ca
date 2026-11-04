package kafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	kafkago "github.com/segmentio/kafka-go"
)

type Config struct {
	Brokers    []string `yaml:"broker"`
	GroupID    string   `yaml:"group_id"`
	Topic      string   `yaml:"topic"`
	RulesTopic string   `yaml:"rules_topic"`
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
		if err = retryHandle(ctx, handle, msg.Value); err != nil {
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

func retryHandle(ctx context.Context, handle MessageHandler, value []byte) error {
	backoff := 100 * time.Millisecond
	for attempt := 1; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := handle(ctx, value)
		if err == nil {
			return nil
		}
		var transient interface{ Retryable() bool }
		if !errors.As(err, &transient) || !transient.Retryable() {
			return err
		}
		if attempt == 1 || attempt%10 == 0 {
			slog.WarnContext(ctx, "retrying comment processing", "attempt", attempt, "error", err)
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		backoff = min(backoff*2, 5*time.Second)
	}
}
