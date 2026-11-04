package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	kafkago "github.com/segmentio/kafka-go"
)

func RoutingKey(platform, account, participant string) []byte {
	key, _ := json.Marshal([3]string{platform, account, participant})
	return key
}
func RoutingPartition(key []byte, count int) int {
	partitions := make([]int, count)
	for i := range partitions {
		partitions[i] = i
	}
	return (&kafkago.Murmur2Balancer{}).Balance(kafkago.Message{Key: key}, partitions...)
}
func validatePartitions(ctx context.Context, cfg Config) error {
	var err error
	for _, broker := range cfg.Brokers {
		conn, e := (&kafkago.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", broker)
		if e != nil {
			err = e
			continue
		}
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		partitions, e := conn.ReadPartitions(cfg.Topic)
		_ = conn.Close()
		if e != nil {
			err = e
			continue
		}
		if len(partitions) != cfg.Partitions {
			return fmt.Errorf("topic has %d partitions, routing expects %d; coordinated migration required", len(partitions), cfg.Partitions)
		}
		return nil
	}
	return err
}

type fixedPartition struct{}

func (fixedPartition) Balance(m kafkago.Message, _ ...int) int { return m.Partition }

type Publisher struct {
	writer     *kafkago.Writer
	partitions int
}

func NewPublisher(cfg Config) (*Publisher, error) {
	if cfg.Partitions < 1 || len(cfg.Brokers) == 0 || cfg.Topic == "" {
		return nil, errors.New("invalid publisher configuration")
	}
	return &Publisher{partitions: cfg.Partitions, writer: &kafkago.Writer{Addr: kafkago.TCP(cfg.Brokers...), Topic: cfg.Topic, Balancer: fixedPartition{}, RequiredAcks: kafkago.RequireAll, WriteTimeout: 5 * time.Second, BatchTimeout: 10 * time.Millisecond}}, nil
}
func (p *Publisher) Publish(ctx context.Context, key, value []byte) error {
	return p.writer.WriteMessages(ctx, kafkago.Message{Key: key, Value: value, Partition: RoutingPartition(key, p.partitions)})
}
func (p *Publisher) Close() error { return p.writer.Close() }
