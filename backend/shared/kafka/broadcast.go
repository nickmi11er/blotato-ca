package kafka

import (
	"context"
	"errors"
	"fmt"
	"time"

	kafkago "github.com/segmentio/kafka-go"
	"golang.org/x/sync/errgroup"
)

// Broadcast reads every partition without a consumer group. Every replica receives
// changes. Restarting from the captured log end is safe only for disposable caches.
type Broadcast struct {
	readers []*kafkago.Reader
	writer  *kafkago.Writer
}

func NewBroadcast(ctx context.Context, brokers []string, topic string) (_ *Broadcast, err error) {
	if len(brokers) == 0 || topic == "" {
		return nil, errors.New("rules brokers and topic are required")
	}
	b := &Broadcast{}
	defer func() {
		if err != nil {
			_ = b.Close()
		}
	}()
	dialer := &kafkago.Dialer{Timeout: 5 * time.Second}
	var conn *kafkago.Conn
	var seed string
	for _, broker := range brokers {
		conn, err = dialer.DialContext(ctx, "tcp", broker)
		if err == nil {
			seed = broker
			break
		}
	}
	if err != nil {
		return nil, err
	}
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	partitions, err := conn.ReadPartitions(topic)
	_ = conn.Close()
	if err != nil {
		return nil, err
	}
	for _, partition := range partitions {
		leader, e := dialer.DialLeader(ctx, "tcp", seed, topic, partition.ID)
		if e != nil {
			return nil, e
		}
		_ = leader.SetDeadline(time.Now().Add(5 * time.Second))
		offset, e := leader.ReadLastOffset()
		_ = leader.Close()
		if e != nil {
			return nil, e
		}
		reader := kafkago.NewReader(kafkago.ReaderConfig{Brokers: brokers, Topic: topic, Partition: partition.ID, MinBytes: 1, MaxBytes: 1 << 20, MaxWait: time.Second, Dialer: dialer})
		if e = reader.SetOffset(offset); e != nil {
			_ = reader.Close()
			return nil, e
		}
		b.readers = append(b.readers, reader)
	}
	if len(b.readers) == 0 {
		return nil, fmt.Errorf("rules topic %s has no partitions", topic)
	}
	b.writer = &kafkago.Writer{Addr: kafkago.TCP(brokers...), Topic: topic, Balancer: &kafkago.Hash{}, RequiredAcks: kafkago.RequireAll, WriteTimeout: 5 * time.Second, BatchTimeout: 10 * time.Millisecond}
	return b, nil
}
func (b *Broadcast) Publish(ctx context.Context, messages []kafkago.Message) error {
	return b.writer.WriteMessages(ctx, messages...)
}
func (b *Broadcast) Consume(ctx context.Context, handle MessageHandler) error {
	g, ctx := errgroup.WithContext(ctx)
	for _, reader := range b.readers {
		g.Go(func() error {
			for {
				message, err := reader.ReadMessage(ctx)
				if err != nil {
					return err
				}
				if err = handle(ctx, message.Value); err != nil {
					return err
				}
			}
		})
	}
	return g.Wait()
}
func (b *Broadcast) Close() error {
	var result error
	for _, reader := range b.readers {
		result = errors.Join(result, reader.Close())
	}
	if b.writer != nil {
		result = errors.Join(result, b.writer.Close())
	}
	return result
}
