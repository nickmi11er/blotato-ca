package automation

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	db "blotato-comment-automation/internal/db/sqlcDB/automation"
	"blotato-comment-automation/shared/kafka"

	"github.com/jackc/pgx/v5/pgxpool"
	kafkago "github.com/segmentio/kafka-go"
)

type Invalidator interface{ Invalidate(string, int64) }
type Change struct {
	EventID    int64  `json:"event_id"`
	AccountID  string `json:"account_id"`
	Generation int64  `json:"generation"`
}
type Changes struct {
	pool  *pgxpool.Pool
	bus   *kafka.Broadcast
	cache Invalidator
}

func NewChanges(pool *pgxpool.Pool, bus *kafka.Broadcast, cache Invalidator) *Changes {
	return &Changes{pool: pool, bus: bus, cache: cache}
}
func (c *Changes) Consume(ctx context.Context) error {
	return c.bus.Consume(ctx, func(_ context.Context, value []byte) error {
		var event Change
		if err := json.Unmarshal(value, &event); err != nil {
			return fmt.Errorf("invalid rule change: %w", err)
		}
		if !ValidID(event.AccountID) || event.Generation < 1 || event.EventID < 1 {
			return fmt.Errorf("invalid rule change fields")
		}
		c.cache.Invalidate(event.AccountID, event.Generation)
		return nil
	})
}
func (c *Changes) Flush(ctx context.Context) error {
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	q := db.New(tx)
	rows, err := q.PendingRuleChanges(ctx)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return tx.Commit(ctx)
	}
	messages := make([]kafkago.Message, 0, len(rows))
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		value, err := json.Marshal(Change{EventID: row.ID, AccountID: row.AccountID, Generation: row.Generation})
		if err != nil {
			return err
		}
		messages = append(messages, kafkago.Message{Key: []byte(row.AccountID), Value: value, Headers: []kafkago.Header{{Key: "event_id", Value: []byte(strconv.FormatInt(row.ID, 10))}}})
		ids = append(ids, row.ID)
	}
	if err = c.bus.Publish(ctx, messages); err != nil {
		return err
	}
	// Delete only after broker acknowledgement. A crash can duplicate, never lose, a notification.
	if err = q.DeleteRuleChanges(ctx, ids); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (c *Changes) Run(ctx context.Context) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := c.Flush(bounded)
			cancel()
			if err != nil && ctx.Err() == nil {
				slog.WarnContext(ctx, "rule outbox will retry", "error", err)
			}
		}
	}
}
