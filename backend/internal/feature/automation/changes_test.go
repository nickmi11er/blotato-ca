package automation_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"blotato-comment-automation/internal/feature/automation"
	"blotato-comment-automation/internal/feature/matching"
	"blotato-comment-automation/internal/testutil"
	"blotato-comment-automation/shared/kafka"
)

func TestRuleChangesReachBothReplicas(t *testing.T) {
	broker := os.Getenv("TEST_KAFKA_BROKER")
	if broker == "" {
		t.Skip("set TEST_KAFKA_BROKER and create automation.rules topic")
	}
	pool := testutil.Database(t)
	service := automation.NewService(pool)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	account := fmt.Sprintf("fanout-%d", time.Now().UnixNano())
	if err := service.RegisterAccount(ctx, "tenant", account, "instagram"); err != nil {
		t.Fatal(err)
	}
	type replica struct {
		cache   *matching.Cache
		changes *automation.Changes
		bus     *kafka.Broadcast
	}
	replicas := make([]replica, 2)
	var wg sync.WaitGroup
	defer func() {
		cancel()
		wg.Wait()
		for _, r := range replicas {
			if r.bus != nil {
				_ = r.bus.Close()
			}
			if r.cache != nil {
				r.cache.Close()
			}
		}
	}()
	for i := range replicas {
		cache, err := matching.NewCache(ctx, matching.NewRepository(pool), matching.DefaultConfig())
		if err != nil {
			t.Fatal(err)
		}
		replicas[i].cache = cache
		bus, err := kafka.NewBroadcast(ctx, []string{broker}, "automation.rules")
		if err != nil {
			t.Fatal(err)
		}
		replicas[i].bus = bus
		changes := automation.NewChanges(pool, bus, cache)
		replicas[i].changes = changes
		wg.Go(func() {
			if err := changes.Consume(ctx); err != nil && ctx.Err() == nil {
				t.Error(err)
			}
		})
		results, err := cache.Match(ctx, "instagram", account, "post", "pricing")
		if err != nil || len(results) != 0 {
			t.Fatal("initial negative cache", err)
		}
	}
	// Consume the registration notification before testing publication; otherwise
	// that earlier invalidation could mask a broken publication event.
	if err := replicas[0].changes.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	if err := service.Save(ctx, "tenant", "pricing", document(account)); err != nil {
		t.Fatal(err)
	}
	if err := service.Publish(ctx, "tenant", "pricing"); err != nil {
		t.Fatal(err)
	}
	if err := replicas[0].changes.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	await := func(want int) {
		t.Helper()
		deadline := time.Now().Add(4 * time.Second)
		for {
			ready := true
			for _, r := range replicas {
				results, err := r.cache.Match(ctx, "instagram", account, "post", "pricing")
				if err != nil {
					t.Fatal(err)
				}
				ready = ready && len(results) == want
			}
			if ready {
				return
			}
			if time.Now().After(deadline) {
				t.Fatal("both replicas did not observe change before cache freshness timeout")
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	await(1)
	if err := service.Delete(ctx, "tenant", "pricing"); err != nil {
		t.Fatal(err)
	}
	if err := replicas[1].changes.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	await(0)
	var pending int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM rule_changes").Scan(&pending); err != nil || pending != 0 {
		t.Fatal("outbox not drained", err, pending)
	}
}
