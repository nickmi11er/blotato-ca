package testutil

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	kafkago "github.com/segmentio/kafka-go"
)

func KafkaTopic(t *testing.T, partitions int) (string, string) {
	t.Helper()
	broker := os.Getenv("TEST_KAFKA_BROKER")
	if broker == "" {
		t.Skip("set TEST_KAFKA_BROKER to run Kafka integration tests")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	conn, err := kafkago.DialContext(ctx, "tcp", broker)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	controller, err := conn.Controller()
	if err != nil {
		t.Fatal(err)
	}
	leader, err := kafkago.DialContext(ctx, "tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	if err != nil {
		t.Fatal(err)
	}
	defer leader.Close()
	_ = leader.SetDeadline(time.Now().Add(10 * time.Second))
	topic := fmt.Sprintf("execution-test-%d", time.Now().UnixNano())
	if err = leader.CreateTopics(kafkago.TopicConfig{Topic: topic, NumPartitions: partitions, ReplicationFactor: 1}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		connection, err := kafkago.DialContext(ctx, "tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
		if err != nil {
			t.Error(err)
			return
		}
		defer connection.Close()
		_ = connection.SetDeadline(time.Now().Add(5 * time.Second))
		if err = connection.DeleteTopics(topic); err != nil {
			t.Error(err)
		}
	})
	// CreateTopics acknowledges metadata creation before every partition is ready.
	// Wait for usable logs so clients do not race leader initialization.
	for partition := range partitions {
		for {
			log, err := kafkago.DialLeader(ctx, "tcp", broker, topic, partition)
			if err == nil {
				_ = log.SetDeadline(time.Now().Add(time.Second))
				_, err = log.ReadLastOffset()
				_ = log.Close()
			}
			if err == nil {
				break
			}
			select {
			case <-ctx.Done():
				t.Fatalf("test topic partition %d not ready: %v", partition, err)
			case <-time.After(25 * time.Millisecond):
			}
		}
	}
	return broker, topic
}
