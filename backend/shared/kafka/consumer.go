package kafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	kafkago "github.com/segmentio/kafka-go"
)

type Config struct {
	Brokers    []string `yaml:"broker"`
	GroupID    string   `yaml:"group_id"`
	Topic      string   `yaml:"topic"`
	RulesTopic string   `yaml:"rules_topic"`
	Partitions int      `yaml:"partitions"`
}
type Consumer struct {
	group *kafkago.ConsumerGroup
	cfg   Config
}
type MessageHandler func(context.Context, []byte) error
type PartitionHandler struct {
	Handle func(context.Context, kafkago.Message) error
	Idle   func()
}
type PartitionFactory func(assignedPartitions int) PartitionHandler

func NewConsumer(cfg Config) (*Consumer, error) {
	if cfg.Partitions < 1 {
		return nil, errors.New("messaging.partitions is required")
	}
	group, err := kafkago.NewConsumerGroup(kafkago.ConsumerGroupConfig{
		Brokers: cfg.Brokers, ID: cfg.GroupID, Topics: []string{cfg.Topic}, StartOffset: kafkago.FirstOffset,
		Dialer: &kafkago.Dialer{Timeout: 5 * time.Second}, Timeout: 5 * time.Second,
		WatchPartitionChanges: true, PartitionWatchInterval: 5 * time.Second,
		ErrorLogger: slog.NewLogLogger(slog.Default().Handler(), slog.LevelError),
	})
	if err != nil {
		return nil, err
	}
	return &Consumer{group: group, cfg: cfg}, nil
}
func (c *Consumer) Close() error { return c.group.Close() }
func (c *Consumer) Consume(ctx context.Context, factory PartitionFactory) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(ctx, func() { _ = c.group.Close() })
	defer stop()
	failures := make(chan error, 1)
	report := func(err error) {
		select {
		case failures <- err:
		default:
		}
		cancel()
	}
	var workers sync.WaitGroup
	defer func() { cancel(); _ = c.group.Close(); workers.Wait() }()
	for {
		generation, err := c.group.Next(ctx)
		if err != nil {
			select {
			case failure := <-failures:
				return failure
			default:
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		if err = validatePartitions(ctx, c.cfg); err != nil {
			cancel()
			return err
		}
		assignments := generation.Assignments[c.cfg.Topic]
		if len(assignments) == 0 {
			generation.Start(func(gctx context.Context) { <-gctx.Done() })
			continue
		}
		for _, assignment := range assignments {
			workers.Add(1)
			generation.Start(func(gctx context.Context) {
				defer workers.Done()
				bounded, cancelAssignment := context.WithCancel(gctx)
				defer cancelAssignment()
				stop := context.AfterFunc(ctx, cancelAssignment)
				defer stop()
				// A distinct handler/router is allocated for every partition in every generation.
				handler := factory(len(assignments))
				reader := kafkago.NewReader(kafkago.ReaderConfig{Brokers: c.cfg.Brokers, Topic: c.cfg.Topic, Partition: assignment.ID, MinBytes: 1, MaxBytes: 1 << 20, MaxWait: time.Second, Dialer: &kafkago.Dialer{Timeout: 5 * time.Second}})
				defer reader.Close()
				if err := reader.SetOffset(assignment.Offset); err != nil {
					report(err)
					return
				}
				for {
					fetch, done := context.WithTimeout(bounded, time.Second)
					msg, err := reader.ReadMessage(fetch)
					done()
					if bounded.Err() != nil {
						return
					}
					if errors.Is(err, context.DeadlineExceeded) {
						if handler.Idle != nil {
							handler.Idle()
						}
						continue
					}
					if err != nil {
						report(fmt.Errorf("read partition %d: %w", assignment.ID, err))
						return
					}
					if RoutingPartition(msg.Key, c.cfg.Partitions) != assignment.ID {
						report(errors.New("message used an incompatible partitioning strategy"))
						return
					}
					err = retryHandle(bounded, func(call context.Context, _ []byte) error {
						request, cancelRequest := context.WithTimeout(call, 10*time.Second)
						defer cancelRequest()
						return handler.Handle(request, msg)
					}, msg.Value)
					if bounded.Err() != nil {
						return
					}
					if err != nil {
						report(fmt.Errorf("process %s[%d] offset %d: %w", msg.Topic, msg.Partition, msg.Offset, err))
						return
					}
					// Generation-specific commits cannot advance offsets after ownership changes.
					err = generation.CommitOffsets(map[string]map[int]int64{msg.Topic: {msg.Partition: msg.Offset + 1}})
					if err != nil {
						if bounded.Err() == nil {
							report(err)
						}
						return
					}
				}
			})
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
