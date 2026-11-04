package automation_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"blotato-comment-automation/internal/feature/automation"
	"blotato-comment-automation/internal/feature/matching"
	"blotato-comment-automation/internal/testutil"
	"blotato-comment-automation/shared/kafka"
)

type observedInvalidation struct {
	cache      *matching.Cache
	generation atomic.Int64
}

func (o *observedInvalidation) Invalidate(account string, generation int64) {
	o.cache.Invalidate(account, generation)
	o.generation.Store(generation)
}

func TestRuleChangesReachBothReplicas(t *testing.T) {
	broker, topic := testutil.KafkaTopic(t, 1)
	pool := testutil.Database(t)
	service := automation.NewService(pool)
	ctx, cancel := context.WithTimeout(t.Context(), 40*time.Second)
	defer cancel()
	account := fmt.Sprintf("fanout-%d", time.Now().UnixNano())
	type replica struct {
		cache    *matching.Cache
		bus      *kafka.Broadcast
		observer *observedInvalidation
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
		config := matching.DefaultConfig()
		config.FreshTTL = 10 * time.Minute
		config.NegativeTTL = 10 * time.Minute
		cache, err := matching.NewCache(ctx, matching.NewRepository(pool), config)
		if err != nil {
			t.Fatal(err)
		}
		replicas[i].cache = cache
		bus, err := kafka.NewBroadcast(ctx, []string{broker}, topic)
		if err != nil {
			t.Fatal(err)
		}
		replicas[i].bus = bus
		observer := &observedInvalidation{cache: cache}
		replicas[i].observer = observer
		changes := automation.NewChanges(pool, bus, observer)
		wg.Go(func() {
			if err := changes.Consume(ctx); err != nil && ctx.Err() == nil {
				t.Error(err)
			}
		})
		wg.Go(func() {
			if err := changes.Run(ctx); err != nil && ctx.Err() == nil {
				t.Error(err)
			}
		})
	}
	if err := service.RegisterAccount(ctx, "tenant", account, "instagram"); err != nil {
		t.Fatal(err)
	}
	// Wait for actual consumption, not merely outbox publication.
	awaitGeneration := func() {
		t.Helper()
		var generation int64
		if err := pool.QueryRow(ctx, "SELECT generation FROM account_rule_sets WHERE account_id=$1", account).Scan(&generation); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(10 * time.Second)
		for {
			ready := true
			for _, r := range replicas {
				ready = ready && r.observer.generation.Load() >= generation
			}
			if ready {
				return
			}
			if time.Now().After(deadline) {
				t.Fatal("rule generation not consumed by both replicas")
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	awaitGeneration()
	// Rewarm only after registration has invalidated both replicas.
	for _, r := range replicas {
		results, err := r.cache.Match(ctx, "instagram", account, "post", "pricing")
		if err != nil || len(results) != 0 {
			t.Fatal("initial negative cache", err)
		}
	}
	if err := service.Save(ctx, "tenant", "pricing", document(account)); err != nil {
		t.Fatal(err)
	}
	if err := service.Publish(ctx, "tenant", "pricing"); err != nil {
		t.Fatal(err)
	}
	awaitGeneration()
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
	awaitGeneration()
	await(0)
	deadline := time.Now().Add(4 * time.Second)
	for {
		var pending int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM rule_changes").Scan(&pending); err != nil {
			t.Fatal(err)
		}
		if pending == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("outbox not drained")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
