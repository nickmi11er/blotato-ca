package matching

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	"blotato-comment-automation/internal/feature/automation"
)

type Config struct {
	MaxBytes            int64         `yaml:"max_bytes"`
	IdleTTL             time.Duration `yaml:"idle_ttl"`
	FreshTTL            time.Duration `yaml:"fresh_ttl"`
	NegativeTTL         time.Duration `yaml:"negative_ttl"`
	LoadTimeout         time.Duration `yaml:"load_timeout"`
	MaxLoads            int           `yaml:"max_loads"`
	MaintenanceInterval time.Duration `yaml:"maintenance_interval"`
	ReconcileBatch      int           `yaml:"reconcile_batch"`
}

func DefaultConfig() Config {
	return Config{MaxBytes: 64 << 20, IdleTTL: 20 * time.Minute, FreshTTL: time.Minute, NegativeTTL: 15 * time.Second, LoadTimeout: 5 * time.Second, MaxLoads: 16, MaintenanceInterval: 5 * time.Second, ReconcileBatch: 256}
}
func (c Config) Validate() error {
	if c.MaxBytes < 1024 || c.IdleTTL <= 0 || c.FreshTTL <= 0 || c.NegativeTTL <= 0 || c.NegativeTTL > c.FreshTTL || c.LoadTimeout <= 0 || c.MaxLoads < 1 || c.MaintenanceInterval <= 0 || c.ReconcileBatch < 1 {
		return errors.New("invalid matching cache configuration")
	}
	return nil
}

var ErrBusy = errors.New("rule cache fill capacity exhausted")
var ErrPlatform = errors.New("event platform does not match registered account")

type key struct {
	account, post string
	generation    int64
}
type compiledRule struct {
	match     automation.Match
	predicate *predicate
}
type prepared struct {
	Snapshot
	rules  []compiledRule
	cost   int64
	loaded time.Time
}
type entry struct {
	key                 key
	value               *prepared
	touched, freshUntil time.Time
}
type accountState struct {
	refs    int
	minimum int64
	queue   *list.Element
	entries map[key]*list.Element
}
type flight struct {
	done  chan struct{}
	value *prepared
	err   error
}

// Cache contains only disposable configuration. Entries and in-flight version guards
// share a lifetime, so evicting an account also releases its invalidation metadata.
type Cache struct {
	source    Source
	cfg       Config
	ctx       context.Context
	cancel    context.CancelFunc
	now       func() time.Time
	mu        sync.Mutex
	entries   map[key]*list.Element
	lru       list.List
	accounts  map[string]*accountState
	reconcile list.List
	flights   map[key]*flight
	cost      int64
	workers   sync.WaitGroup
}

func NewCache(ctx context.Context, source Source, cfg Config) (*Cache, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	return &Cache{source: source, cfg: cfg, ctx: ctx, cancel: cancel, now: time.Now, entries: map[key]*list.Element{}, accounts: map[string]*accountState{}, flights: map[key]*flight{}}, nil
}
func (c *Cache) Close() { c.cancel(); c.mu.Lock(); c.mu.Unlock(); c.workers.Wait() }
func (c *Cache) retain(account string) *accountState {
	s := c.accounts[account]
	if s == nil {
		s = &accountState{entries: map[key]*list.Element{}, queue: c.reconcile.PushBack(account)}
		c.accounts[account] = s
	}
	s.refs++
	return s
}
func (c *Cache) release(account string) {
	s := c.accounts[account]
	s.refs--
	if s.refs == 0 {
		c.reconcile.Remove(s.queue)
		delete(c.accounts, account)
	}
}
func (c *Cache) remove(e *list.Element) {
	v := e.Value.(*entry)
	delete(c.entries, v.key)
	delete(c.accounts[v.key.account].entries, v.key)
	c.lru.Remove(e)
	c.cost -= v.value.cost
	c.release(v.key.account)
}
func (c *Cache) invalidate(account string, generation int64) {
	s := c.accounts[account]
	if s == nil || generation <= s.minimum {
		return
	}
	s.minimum = generation
	for _, e := range s.entries {
		if e.Value.(*entry).value.Generation < generation {
			c.remove(e)
		}
	}
}
func (c *Cache) Invalidate(account string, generation int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.invalidate(account, generation)
}
func (c *Cache) lifetime(v *prepared) time.Duration {
	ttl := c.cfg.FreshTTL
	if len(v.rules) == 0 && !v.HasSelected {
		ttl = c.cfg.NegativeTTL
	}
	// Jitter only shortens the configured freshness bound.
	return time.Duration(float64(ttl) * (0.9 + rand.Float64()*0.1))
}
func prepare(s Snapshot, at time.Time, k key) (*prepared, error) {
	p := &prepared{Snapshot: s, loaded: at, cost: int64(512 + len(k.account) + len(k.post) + len(s.TenantID) + len(s.Platform))}
	p.Snapshot.Rules = nil
	for _, r := range s.Rules {
		predicate, err := compile(r.Predicate)
		if err != nil {
			return nil, err
		}
		p.rules = append(p.rules, compiledRule{match: r.Match, predicate: predicate})
		p.cost += int64(512 + len(r.Match.AutomationID) + len(r.Match.TenantID))
		for _, terms := range [][]string{r.Predicate.Keywords, r.Predicate.Exclusions} {
			for _, term := range terms {
				p.cost += int64(1024 + len(term)*128)
			}
		}
	}
	return p, nil
}
func (c *Cache) get(ctx context.Context, k key) (*prepared, error) {
	c.mu.Lock()
	now := c.now()
	if e := c.entries[k]; e != nil {
		v := e.Value.(*entry)
		if now.Before(v.touched.Add(c.cfg.IdleTTL)) && now.Before(v.freshUntil) {
			v.touched = now
			c.lru.MoveToFront(e)
			result := v.value
			c.mu.Unlock()
			return result, nil
		}
		c.remove(e)
	}
	if err := c.ctx.Err(); err != nil {
		c.mu.Unlock()
		return nil, err
	}
	f := c.flights[k]
	if f == nil {
		if len(c.flights) >= c.cfg.MaxLoads {
			c.mu.Unlock()
			return nil, ErrBusy
		}
		f = &flight{done: make(chan struct{})}
		c.flights[k] = f
		c.retain(k.account)
		c.workers.Add(1)
		go c.fill(k, f)
	}
	c.mu.Unlock()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-f.done:
		return f.value, f.err
	}
}
func (c *Cache) fill(k key, f *flight) {
	defer c.workers.Done()
	ctx, cancel := context.WithTimeout(c.ctx, c.cfg.LoadTimeout)
	defer cancel()
	at := c.now()
	var s Snapshot
	var err error
	if k.generation < 0 {
		s, err = c.source.Account(ctx, k.account)
	} else {
		s, err = c.source.Post(ctx, k.account, k.post, k.generation)
	}
	var value *prepared
	if err == nil {
		value, err = prepare(s, at, k)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err == nil && c.ctx.Err() != nil {
		err = c.ctx.Err()
	}
	if err == nil && (value.Generation < c.accounts[k.account].minimum || k.generation >= 0 && value.Generation != k.generation) {
		err = ErrGeneration
	}
	var deadline time.Time
	if err == nil {
		deadline = at.Add(c.lifetime(value))
		if !c.now().Before(deadline) {
			err = errors.New("rule load exceeded freshness deadline")
		}
	}
	if err == nil {
		c.invalidate(k.account, value.Generation)
		if value.cost <= c.cfg.MaxBytes {
			for c.cost+value.cost > c.cfg.MaxBytes {
				c.remove(c.lru.Back())
			}
			c.retain(k.account)
			c.entries[k] = c.lru.PushFront(&entry{key: k, value: value, touched: c.now(), freshUntil: deadline})
			c.accounts[k.account].entries[k] = c.entries[k]
			c.cost += value.cost
		}
		f.value = value
	}
	if errors.Is(err, ErrGeneration) && k.generation >= 0 {
		c.invalidate(k.account, k.generation+1)
	}
	f.err = err
	delete(c.flights, k)
	c.release(k.account)
	close(f.done)
}

func (c *Cache) Match(ctx context.Context, platform, account, post, text string) ([]automation.Match, error) {
	// Normalize each representation at most once, independent of candidate count.
	texts := map[bool]string{}
	for attempt := 0; attempt < 3; attempt++ {
		a, err := c.get(ctx, key{account: account, generation: -1})
		if errors.Is(err, ErrGeneration) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if a.Platform != "" && a.Platform != platform {
			return nil, ErrPlatform
		}
		candidates := a.rules
		if a.HasSelected {
			p, err := c.get(ctx, key{account: account, post: post, generation: a.Generation})
			if errors.Is(err, ErrGeneration) {
				continue
			}
			if err != nil {
				return nil, err
			}
			candidates = append(append([]compiledRule(nil), a.rules...), p.rules...)
		}
		result := make([]automation.Match, 0, len(candidates))
		seen := map[string]bool{}
		for _, r := range candidates {
			normalized, ok := texts[r.predicate.sensitive]
			if !ok {
				normalized = normalize(text, r.predicate.sensitive)
				texts[r.predicate.sensitive] = normalized
			}
			if !seen[r.match.AutomationID] && r.predicate.matches(normalized) {
				result = append(result, r.match)
				seen[r.match.AutomationID] = true
			}
		}
		return result, nil
	}
	return nil, fmt.Errorf("rules changed repeatedly: %w", ErrGeneration)
}

// Maintain reconciles only resident accounts in bounded batches. It does not warm
// cold accounts or extend idle lifetimes. On failure normal freshness checks apply.
func (c *Cache) Maintain(ctx context.Context) error {
	c.mu.Lock()
	at := c.now()
	for e := c.lru.Back(); e != nil; e = c.lru.Back() {
		if at.Before(e.Value.(*entry).touched.Add(c.cfg.IdleTTL)) {
			break
		}
		c.remove(e)
	}
	count := min(c.cfg.ReconcileBatch, c.reconcile.Len())
	ids := make([]string, 0, count)
	for range count {
		next := c.reconcile.Front()
		ids = append(ids, next.Value.(string))
		c.reconcile.MoveToBack(next)
	}
	c.mu.Unlock()
	if len(ids) == 0 {
		return nil
	}
	generations, err := c.source.Generations(ctx, ids)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, generation := range generations {
		c.invalidate(id, generation)
		state := c.accounts[id]
		if state == nil {
			continue
		}
		for _, e := range state.entries {
			v := e.Value.(*entry)
			if v.value.Generation == generation && !v.value.loaded.After(at) {
				deadline := at.Add(c.lifetime(v.value))
				if deadline.After(v.freshUntil) {
					v.freshUntil = deadline
				}
			}
		}
	}
	return nil
}
func (c *Cache) Run(ctx context.Context) error {
	ticker := time.NewTicker(c.cfg.MaintenanceInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			bounded, cancel := context.WithTimeout(ctx, c.cfg.LoadTimeout)
			if err := c.Maintain(bounded); err != nil && ctx.Err() == nil {
				slog.WarnContext(ctx, "rule reconciliation failed", "error", err)
			}
			cancel()
		}
	}
}
