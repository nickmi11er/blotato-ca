package matching

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"blotato-comment-automation/internal/feature/automation"
)

type fakeSource struct {
	mu                      sync.Mutex
	generation              int64
	selected                bool
	accountCalls, postCalls int
	fail                    bool
	block, started          chan struct{}
}

func fixtureRule() Rule {
	return Rule{Match: automation.Match{TenantID: "tenant", AutomationID: "automation", Revision: 1, Repeat: "every"}, Predicate: automation.Predicate{Mode: "keywords", Operator: "whole_word", Logic: "any", Keywords: []string{"pricing"}}}
}
func (f *fakeSource) Account(ctx context.Context, _ string) (Snapshot, error) {
	f.mu.Lock()
	f.accountCalls++
	generation, selected, fail := f.generation, f.selected, f.fail
	block, started := f.block, f.started
	f.block = nil
	f.started = nil
	f.mu.Unlock()
	if started != nil {
		close(started)
	}
	if block != nil {
		select {
		case <-ctx.Done():
			return Snapshot{}, ctx.Err()
		case <-block:
		}
	}
	if fail {
		return Snapshot{}, errors.New("database offline")
	}
	s := Snapshot{Generation: generation, HasSelected: selected, Platform: "instagram", TenantID: "tenant"}
	if generation > 0 && !selected {
		s.Rules = []Rule{fixtureRule()}
	}
	return s, nil
}
func (f *fakeSource) Post(_ context.Context, _, post string, generation int64) (Snapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.postCalls++
	if f.fail {
		return Snapshot{}, errors.New("database offline")
	}
	if generation != f.generation {
		return Snapshot{}, ErrGeneration
	}
	s := Snapshot{Generation: generation}
	if post == "selected" {
		s.Rules = []Rule{fixtureRule()}
	}
	return s, nil
}
func (f *fakeSource) Generations(_ context.Context, ids []string) (map[string]int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return nil, errors.New("database offline")
	}
	m := map[string]int64{}
	for _, id := range ids {
		m[id] = f.generation
	}
	return m, nil
}
func testCache(t *testing.T, f *fakeSource, cfg Config) (*Cache, *atomic.Int64) {
	t.Helper()
	c, err := NewCache(t.Context(), f, cfg)
	if err != nil {
		t.Fatal(err)
	}
	clock := &atomic.Int64{}
	clock.Store(time.Now().UnixNano())
	c.now = func() time.Time { return time.Unix(0, clock.Load()) }
	t.Cleanup(c.Close)
	return c, clock
}
func runMatch(t *testing.T, c *Cache, account, post string) int {
	t.Helper()
	matches, err := c.Match(t.Context(), "instagram", account, post, "pricing")
	if err != nil {
		t.Fatal(err)
	}
	return len(matches)
}
func TestNegativeAccountAvoidsPostAndDatabaseReads(t *testing.T) {
	f := &fakeSource{}
	c, _ := testCache(t, f, DefaultConfig())
	for i := range 1000 {
		if runMatch(t, c, "account", fmt.Sprint(i)) != 0 {
			t.Fatal("unexpected match")
		}
	}
	if f.accountCalls != 1 || f.postCalls != 0 {
		t.Fatalf("reads: account=%d post=%d", f.accountCalls, f.postCalls)
	}
}
func TestNegativePostAndAccountGeneration(t *testing.T) {
	f := &fakeSource{generation: 1, selected: true}
	c, _ := testCache(t, f, DefaultConfig())
	for range 20 {
		if runMatch(t, c, "account", "irrelevant") != 0 {
			t.Fatal("unexpected match")
		}
	}
	if runMatch(t, c, "account", "selected") != 1 {
		t.Fatal("selected post not matched")
	}
	if f.accountCalls != 1 || f.postCalls != 2 {
		t.Fatal("cache did not reuse post results")
	}
	f.generation = 2
	c.Invalidate("account", 2)
	c.Invalidate("account", 1)
	if runMatch(t, c, "account", "selected") != 1 || f.accountCalls != 2 || f.postCalls != 3 {
		t.Fatal("account and post must reload together")
	}
}
func TestHotAccountDoesNotKeepColdPosts(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IdleTTL = time.Minute
	cfg.FreshTTL = time.Hour
	f := &fakeSource{generation: 1, selected: true}
	c, clock := testCache(t, f, cfg)
	runMatch(t, c, "account", "selected")
	clock.Add(int64(40 * time.Second))
	runMatch(t, c, "account", "other")
	clock.Add(int64(30 * time.Second))
	if err := c.Maintain(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.entries[key{account: "account", post: "selected", generation: 1}]; ok {
		t.Fatal("cold post retained")
	}
	if _, ok := c.entries[key{account: "account", generation: -1}]; !ok {
		t.Fatal("hot account evicted")
	}
	runMatch(t, c, "account", "selected")
	if f.postCalls != 3 {
		t.Fatal("old post not reloaded")
	}
}
func TestFreshnessIndependentOfIdleAndFailuresNotEmpty(t *testing.T) {
	cfg := DefaultConfig()
	cfg.FreshTTL = time.Minute
	f := &fakeSource{generation: 1}
	c, clock := testCache(t, f, cfg)
	runMatch(t, c, "a", "p")
	clock.Add(int64(40 * time.Second))
	runMatch(t, c, "a", "p")
	clock.Add(int64(21 * time.Second))
	f.fail = true
	if _, err := c.Match(t.Context(), "instagram", "a", "p", "pricing"); err == nil {
		t.Fatal("expired snapshot served during outage")
	}
	f.fail = false
	runMatch(t, c, "a", "p")
	if f.accountCalls != 3 {
		t.Fatal("failure was cached")
	}
}
func TestMemoryAndMetadataStayBounded(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxBytes = 2048
	c, clock := testCache(t, &fakeSource{}, cfg)
	for i := range 1000 {
		runMatch(t, c, fmt.Sprint(i), "p")
	}
	if c.cost > cfg.MaxBytes || len(c.accounts) > 4 || len(c.entries) > 4 {
		t.Fatal("unbounded cache")
	}
	clock.Add(int64(cfg.IdleTTL))
	if err := c.Maintain(t.Context()); err != nil {
		t.Fatal(err)
	}
	if c.cost != 0 || len(c.entries) != 0 || len(c.accounts) != 0 {
		t.Fatal("idle entries or metadata leaked")
	}
	c.Invalidate("cold", 99)
	if len(c.accounts) != 0 {
		t.Fatal("notification populated cold metadata")
	}
}
func TestInvalidationDuringFillCannotResurrectOldRules(t *testing.T) {
	f := &fakeSource{generation: 1, block: make(chan struct{}), started: make(chan struct{})}
	block, started := f.block, f.started
	c, _ := testCache(t, f, DefaultConfig())
	done := make(chan error, 1)
	go func() { _, err := c.Match(t.Context(), "instagram", "a", "p", "pricing"); done <- err }()
	<-started
	f.mu.Lock()
	f.generation = 2
	f.mu.Unlock()
	c.Invalidate("a", 2)
	close(block)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, e := range c.entries {
		if e.Value.(*entry).value.Generation != 2 {
			t.Fatal("stale fill installed")
		}
	}
	if f.accountCalls != 2 {
		t.Fatal("stale load not retried")
	}
}
func TestSharedFillSurvivesWaiterCancellationAndLimitsWork(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxLoads = 1
	f := &fakeSource{generation: 1, block: make(chan struct{}), started: make(chan struct{})}
	block, started := f.block, f.started
	c, _ := testCache(t, f, cfg)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { _, err := c.Match(ctx, "instagram", "a", "p", "pricing"); done <- err }()
	<-started
	cancel()
	if !errors.Is(<-done, context.Canceled) {
		t.Fatal("waiter not canceled")
	}
	if _, err := c.Match(t.Context(), "instagram", "b", "p", "pricing"); !errors.Is(err, ErrBusy) {
		t.Fatal("fill bound ignored")
	}
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			matches, err := c.Match(t.Context(), "instagram", "a", "p", "pricing")
			if err != nil || len(matches) != 1 {
				t.Errorf("shared fill: %v", err)
			}
		})
	}
	close(block)
	wg.Wait()
	if f.accountCalls != 1 {
		t.Fatalf("got %d duplicate loads", f.accountCalls)
	}
}
func TestReconciliationRecoversMissedInvalidation(t *testing.T) {
	f := &fakeSource{}
	c, _ := testCache(t, f, DefaultConfig())
	runMatch(t, c, "a", "p")
	f.generation = 1
	if err := c.Maintain(t.Context()); err != nil {
		t.Fatal(err)
	}
	if runMatch(t, c, "a", "p") != 1 {
		t.Fatal("missed invalidation not recovered")
	}
}

func BenchmarkCachedNonmatch(b *testing.B) {
	source := &fakeSource{}
	cache, err := NewCache(b.Context(), source, DefaultConfig())
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(cache.Close)
	if _, err = cache.Match(b.Context(), "instagram", "account", "post", "hello"); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err = cache.Match(b.Context(), "instagram", "account", "post", "hello"); err != nil {
			b.Fatal(err)
		}
	}
	if source.accountCalls != 1 || source.postCalls != 0 {
		b.Fatal("cache hit performed a source read")
	}
}
