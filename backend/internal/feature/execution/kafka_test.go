package execution_test

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"blotato-comment-automation/internal/feature/admission"
	"blotato-comment-automation/internal/feature/conversation"
	"blotato-comment-automation/internal/feature/execution"
	"blotato-comment-automation/internal/feature/inbound"
	"blotato-comment-automation/internal/feature/ingress"
	"blotato-comment-automation/internal/feature/matching"
	"blotato-comment-automation/internal/testutil"
	"blotato-comment-automation/shared/kafka"
	kafkago "github.com/segmentio/kafka-go"
)

func TestKafkaActivationRebalanceAndDMResume(t *testing.T) {
	broker, topic := testutil.KafkaTopic(t, 3)
	f := setup(t, definition())
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	var wg sync.WaitGroup
	defer func() { cancel(); wg.Wait() }()
	cfg := kafka.Config{Brokers: []string{broker}, Topic: topic, Partitions: 3, GroupID: topic + "-group"}
	transport, err := kafka.NewPublisher(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()
	cache, err := matching.NewCache(ctx, matching.NewRepository(f.pool), matching.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer cache.Close()
	comments := inbound.NewHandler(admission.NewRepository(f.pool), cache)
	factory := ingress.NewFactory(comments, f.store, conversation.DefaultConfig())
	var assignments atomic.Int32
	received := make(chan string, 32)
	startConsumer := func() {
		consumer, err := kafka.NewConsumer(cfg)
		if err != nil {
			t.Fatal(err)
		}
		wg.Go(func() {
			defer consumer.Close()
			err := consumer.Consume(ctx, func(count int) kafka.PartitionHandler {
				assignments.Add(1)
				handler := factory.Partition(count)
				handle := handler.Handle
				handler.Handle = func(ctx context.Context, msg kafkago.Message) error {
					err := handle(ctx, msg)
					if err == nil {
						var event conversation.Message
						_ = json.Unmarshal(msg.Value, &event)
						select {
						case received <- event.Data.MessageID:
						default:
						}
					}
					return err
				}
				return handler
			})
			if err != nil && ctx.Err() == nil {
				t.Errorf("consumer: %v", err)
			}
		})
	}
	send := func(id, text string) {
		t.Helper()
		raw, _ := json.Marshal(dm(id, text))
		if err := transport.Publish(ctx, kafka.RoutingKey("instagram", "account", "person"), raw); err != nil {
			t.Fatal(err)
		}
	}
	awaitMessage := func(id string) {
		t.Helper()
		for {
			select {
			case got := <-received:
				if got == id {
					return
				}
			case <-ctx.Done():
				t.Fatal("message not consumed", id)
			}
		}
	}
	startConsumer()
	send("warm-before", "hello")
	awaitMessage("warm-before")
	initial := assignments.Load()
	startConsumer()
	for assignments.Load() <= initial {
		select {
		case <-ctx.Done():
			t.Fatal("rebalance did not happen")
		case <-time.After(20 * time.Millisecond):
		}
	}
	send("warm-after", "hello")
	awaitMessage("warm-after")
	worker := execution.NewWorker(f.store, f.engine, f.sender, execution.NewPublisher(transport), execution.DefaultConfig())
	wg.Go(func() {
		if err := worker.Run(ctx); err != nil && ctx.Err() == nil {
			t.Error(err)
		}
	})
	for {
		s := f.session(t)
		if s.CurrentNodeID == "capture" && s.Status == "waiting" {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("activation did not start workflow")
		case <-time.After(20 * time.Millisecond):
		}
	}
	send("answer", "person@example.com")
	awaitMessage("answer")
	for {
		s := f.session(t)
		if s.CurrentNodeID == "delay" {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("DM did not resume workflow")
		case <-time.After(20 * time.Millisecond):
		}
	}
}
