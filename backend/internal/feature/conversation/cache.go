package conversation

import (
	"container/list"
	"context"
	"errors"
	"strings"
	"time"
)

// Router is confined to one sequential Kafka partition/assignment. A new assignment
// constructs a new router, so no negative entries or fills survive a rebalance.
type Config struct {
	MaxBytes    int           `yaml:"max_bytes"`
	IdleTTL     time.Duration `yaml:"idle_ttl"`
	FreshTTL    time.Duration `yaml:"fresh_ttl"`
	NegativeTTL time.Duration `yaml:"negative_ttl"`
}

func DefaultConfig() Config {
	return Config{MaxBytes: 8 << 20, IdleTTL: 20 * time.Minute, FreshTTL: time.Minute, NegativeTTL: 15 * time.Second}
}
func (c Config) Validate() error {
	if c.MaxBytes < 1024 || c.IdleTTL <= 0 || c.FreshTTL <= 0 || c.NegativeTTL <= 0 || c.NegativeTTL > c.FreshTTL {
		return errors.New("invalid DM cache configuration")
	}
	return nil
}

type key struct{ account, participant string }
type entry struct {
	key            key
	route          Route
	touched, fresh time.Time
	cost           int
}
type Router struct {
	source  Source
	cfg     Config
	entries map[key]*list.Element
	lru     list.List
	bytes   int
	now     func() time.Time
}

func NewRouter(source Source, cfg Config) *Router {
	return &Router{source: source, cfg: cfg, entries: map[key]*list.Element{}, now: time.Now}
}
func (r *Router) remove(e *list.Element) {
	v := e.Value.(*entry)
	delete(r.entries, v.key)
	r.bytes -= v.cost
	r.lru.Remove(e)
}
func (r *Router) Prune() {
	now := r.now()
	for e := r.lru.Back(); e != nil; e = r.lru.Back() {
		if now.Before(e.Value.(*entry).touched.Add(r.cfg.IdleTTL)) {
			break
		}
		r.remove(e)
	}
}
func (r *Router) forget(k key) {
	if e := r.entries[k]; e != nil {
		r.remove(e)
	}
}
func (r *Router) lookup(ctx context.Context, k key) (Route, error) {
	r.Prune()
	now := r.now()
	if e := r.entries[k]; e != nil {
		v := e.Value.(*entry)
		if now.Before(v.fresh) {
			v.touched = now
			r.lru.MoveToFront(e)
			return v.route, nil
		}
		r.remove(e)
	}
	route, err := r.source.Route(ctx, k.account, k.participant)
	if err != nil {
		return Route{}, err
	}
	if err = ctx.Err(); err != nil {
		return Route{}, err
	}
	ttl := r.cfg.FreshTTL
	if route.SessionID == "" {
		ttl = r.cfg.NegativeTTL
	}
	v := &entry{key: k, route: route, touched: r.now(), fresh: now.Add(ttl), cost: 256 + len(k.account) + len(k.participant) + len(route.SessionID) + len(route.Platform)}
	if v.cost <= r.cfg.MaxBytes {
		for r.bytes+v.cost > r.cfg.MaxBytes {
			r.remove(r.lru.Back())
		}
		r.entries[k] = r.lru.PushFront(v)
		r.bytes += v.cost
	}
	return route, nil
}
func (r *Router) Message(ctx context.Context, m Message) error {
	if *m.Data.Own || strings.TrimSpace(*m.Data.Text) == "" {
		return nil
	}
	k := key{m.AccountID, m.Data.ParticipantID}
	for range 3 {
		route, err := r.lookup(ctx, k)
		if err != nil {
			return err
		}
		if route.Platform != "" && route.Platform != m.Platform {
			return errors.New("DM platform does not match account")
		}
		if route.SessionID == "" {
			return nil
		}
		err = r.source.Accept(ctx, m, route)
		if errors.Is(err, ErrStale) {
			r.forget(k)
			continue
		}
		return err
	}
	return ErrStale
}
func (r *Router) Activate(ctx context.Context, a Activation) error {
	if err := a.Validate(); err != nil {
		return err
	}
	if err := r.source.Activate(ctx, a); err != nil {
		return err
	}
	// Replays must clear absence even if the activation was already committed.
	k := key{a.AccountID, a.ParticipantID}
	r.forget(k)
	_, err := r.lookup(ctx, k)
	return err
}
