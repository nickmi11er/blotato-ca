package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"blotato-comment-automation/internal/feature/admission"
	"blotato-comment-automation/internal/feature/automation"
	"blotato-comment-automation/internal/feature/conversation"
	"blotato-comment-automation/internal/feature/delivery"
	"blotato-comment-automation/internal/feature/execution"
	"blotato-comment-automation/internal/feature/inbound"
	"blotato-comment-automation/internal/feature/ingress"
	"blotato-comment-automation/internal/feature/matching"
	"blotato-comment-automation/internal/feature/workflow"
	"blotato-comment-automation/internal/testutil"
	"blotato-comment-automation/shared/kafka"
	"github.com/jackc/pgx/v5/pgxpool"
	kafkago "github.com/segmentio/kafka-go"
)

// Real PostgreSQL, Kafka and HTTP. Only the external social/email service is a
// local HTTP fixture; no matcher, repository, state machine or worker is mocked.
type system struct {
	t            *testing.T
	ctx          context.Context
	cancel       context.CancelFunc
	pool         *pgxpool.Pool
	api          *httptest.Server
	client       *http.Client
	transport    *kafka.Publisher
	replicas     []*replica
	wg           sync.WaitGroup
	mu           sync.Mutex
	handled      map[string]int
	effects      map[string]delivery.Request
	providerHook func(context.Context, delivery.Request) error
	errors       []error
}
type replica struct {
	cache       *matching.Cache
	changes     *automation.Changes
	bus         *kafka.Broadcast
	consumer    *kafka.Consumer
	mu          sync.Mutex
	generations map[string]int64
}

func (r *replica) Invalidate(account string, generation int64) {
	r.cache.Invalidate(account, generation)
	r.mu.Lock()
	r.generations[account] = max(r.generations[account], generation)
	r.mu.Unlock()
}
func newSystem(t *testing.T, replicas int) *system {
	t.Helper()
	pool := testutil.Database(t)
	broker, topic := testutil.KafkaTopic(t, 3)
	_, rulesTopic := testutil.KafkaTopic(t, 1)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	s := &system{t: t, ctx: ctx, cancel: cancel, pool: pool, client: &http.Client{Timeout: 5 * time.Second}, handled: map[string]int{}, effects: map[string]delivery.Request{}}
	cfg := kafka.Config{Brokers: []string{broker}, Topic: topic, Partitions: 3, GroupID: topic + "-group"}
	transport, err := kafka.NewPublisher(cfg)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	s.transport = transport
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request delivery.Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			s.recordError(err)
			w.WriteHeader(400)
			return
		}
		if request.ID == "" || r.Header.Get("Idempotency-Key") != request.ID || r.Header.Get("Authorization") != "Bearer provider-secret" {
			s.recordError(fmt.Errorf("invalid delivery headers"))
			w.WriteHeader(400)
			return
		}
		s.mu.Lock()
		previous, duplicate := s.effects[request.ID]
		s.effects[request.ID] = request
		hook := s.providerHook
		s.mu.Unlock()
		if duplicate && previous != request {
			s.recordError(fmt.Errorf("idempotency key reused with different payload"))
			w.WriteHeader(409)
			return
		}
		if !duplicate && hook != nil {
			if err := hook(r.Context(), request); err != nil {
				s.recordError(err)
				w.WriteHeader(500)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(delivery.Result{Status: "succeeded", ProviderMessageID: "provider:" + request.ID})
	}))
	engine := execution.NewEngine(execution.NewStore(pool), 3)
	s.api = httptest.NewServer(automation.NewHTTPHandler(automation.NewService(pool), "api-secret", execution.RegisterHTTP(engine)))
	t.Cleanup(func() {
		cancel()
		done := make(chan struct{})
		go func() { s.wg.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("runtime did not stop within deadline")
		}
		for _, r := range s.replicas {
			_ = r.consumer.Close()
			_ = r.bus.Close()
			r.cache.Close()
		}
		_ = transport.Close()
		s.api.Close()
		provider.Close()
		s.client.CloseIdleConnections()
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, err := range s.errors {
			t.Error(err)
		}
	})
	for range replicas {
		r := &replica{generations: map[string]int64{}}
		cacheConfig := matching.DefaultConfig()
		// Longer than every assertion deadline: a missed invalidation cannot be hidden by TTL.
		cacheConfig.FreshTTL = 10 * time.Minute
		cacheConfig.NegativeTTL = 10 * time.Minute
		r.cache, err = matching.NewCache(ctx, matching.NewRepository(pool), cacheConfig)
		if err != nil {
			t.Fatal(err)
		}
		r.bus, err = kafka.NewBroadcast(ctx, []string{broker}, rulesTopic)
		if err != nil {
			r.cache.Close()
			t.Fatalf("create rules broadcast %s: %v", rulesTopic, err)
		}
		r.consumer, err = kafka.NewConsumer(cfg)
		if err != nil {
			_ = r.bus.Close()
			r.cache.Close()
			t.Fatalf("create rules broadcast %s: %v", rulesTopic, err)
		}
		r.changes = automation.NewChanges(pool, r.bus, r)
		s.replicas = append(s.replicas, r)
		store := execution.NewStore(pool)
		workerConfig := execution.DefaultConfig()
		workerConfig.PollInterval = 15 * time.Millisecond
		workerConfig.MaxAttempts = 3
		worker := execution.NewWorker(store, execution.NewEngine(store, 3), delivery.NewHTTP(delivery.Config{Mode: "http", URL: provider.URL, Token: "provider-secret", Idempotency: true}), execution.NewPublisher(transport), workerConfig)
		factory := ingress.NewFactory(inbound.NewHandler(admission.NewRepository(pool), r.cache), store, conversation.DefaultConfig())
		s.run(func() error { return r.changes.Consume(ctx) })
		s.run(func() error { return r.changes.Run(ctx) })
		s.run(func() error { return worker.Run(ctx) })
		s.run(func() error {
			return r.consumer.Consume(ctx, func(count int) kafka.PartitionHandler {
				handler := factory.Partition(count)
				handle := handler.Handle
				handler.Handle = func(ctx context.Context, msg kafkago.Message) error {
					err := handle(ctx, msg)
					if err == nil {
						s.mu.Lock()
						s.handled[string(msg.Value)]++
						s.mu.Unlock()
					}
					return err
				}
				return handler
			})
		})
	}
	return s
}
func (s *system) recordError(err error) { s.mu.Lock(); s.errors = append(s.errors, err); s.mu.Unlock() }
func (s *system) run(f func() error) {
	s.wg.Go(func() {
		if err := f(); err != nil && s.ctx.Err() == nil {
			s.recordError(err)
		}
	})
}
func (s *system) wait(label string, ready func() bool) {
	s.t.Helper()
	deadline := time.NewTimer(15 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if ready() {
			return
		}
		s.mu.Lock()
		errs := append([]error(nil), s.errors...)
		s.mu.Unlock()
		if len(errs) > 0 {
			s.t.Fatalf("%s: runtime errors: %v", label, errs)
		}
		select {
		case <-s.ctx.Done():
			s.t.Fatalf("%s: %v", label, s.ctx.Err())
		case <-deadline.C:
			s.t.Fatalf("timed out: %s", label)
		case <-ticker.C:
		}
	}
}
func (s *system) apiCall(method, path string, body any, want int) []byte {
	s.t.Helper()
	var encoded []byte
	if body != nil {
		var err error
		encoded, err = json.Marshal(body)
		if err != nil {
			s.t.Fatal(err)
		}
	}
	req, err := http.NewRequestWithContext(s.ctx, method, s.api.URL+"/v1/workspaces/"+path, bytes.NewReader(encoded))
	if err != nil {
		s.t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer api-secret")
	req.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(req)
	if err != nil {
		s.t.Fatal(err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		s.t.Fatal(err)
	}
	if response.StatusCode != want {
		s.t.Fatalf("%s %s: status %d, want %d: %s", method, path, response.StatusCode, want, raw)
	}
	return raw
}
func (s *system) syncRules(account string) {
	s.t.Helper()
	var generation int64
	if err := s.pool.QueryRow(s.ctx, "SELECT generation FROM account_rule_sets WHERE account_id=$1", account).Scan(&generation); err != nil {
		s.t.Fatal(err)
	}
	// Wait for the production outbox publisher and each replica's actual
	// notification; another event or TTL must not mask a missed invalidation.
	s.wait("rule generation on every replica", func() bool {
		for _, r := range s.replicas {
			r.mu.Lock()
			seen := r.generations[account]
			r.mu.Unlock()
			if seen < generation {
				return false
			}
		}
		return true
	})
}
func (s *system) matches(platform, account, post, text string, want int) {
	s.t.Helper()
	for _, r := range s.replicas {
		matches, err := r.cache.Match(s.ctx, platform, account, post, text)
		if err != nil || len(matches) != want {
			s.t.Fatalf("match(%s,%s,%s): %v %v; want %d", account, post, text, matches, err, want)
		}
	}
}
func (s *system) emit(ctx context.Context, platform, account, participant string, event any) (string, error) {
	raw, err := json.Marshal(event)
	if err != nil {
		return "", err
	}
	return string(raw), s.transport.Publish(ctx, kafka.RoutingKey(platform, account, participant), raw)
}
func (s *system) send(platform, account, participant string, event any, copies int) {
	s.t.Helper()
	raw, _ := json.Marshal(event)
	s.mu.Lock()
	before := s.handled[string(raw)]
	s.mu.Unlock()
	for range copies {
		if _, err := s.emit(s.ctx, platform, account, participant, event); err != nil {
			s.t.Fatal(err)
		}
	}
	s.wait("Kafka records handled", func() bool { s.mu.Lock(); defer s.mu.Unlock(); return s.handled[string(raw)] >= before+copies })
}
func (s *system) count(query string, args ...any) int {
	s.t.Helper()
	var n int
	if err := s.pool.QueryRow(s.ctx, query, args...).Scan(&n); err != nil {
		s.t.Fatal(err)
	}
	return n
}

type sessionView struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Node     string `json:"current_node_id"`
	Revision int64  `json:"revision"`
	Context  struct {
		Contact struct {
			Email string `json:"email"`
		} `json:"contact"`
	} `json:"context"`
	History []struct {
		Version int64 `json:"version"`
	} `json:"history"`
}

func (s *system) session(tenant, id string) sessionView {
	s.t.Helper()
	var v sessionView
	if err := json.Unmarshal(s.apiCall("GET", tenant+"/sessions/"+id, nil, 200), &v); err != nil {
		s.t.Fatal(err)
	}
	return v
}
func (s *system) sessionID(automation, participant string) string {
	s.t.Helper()
	var sid string
	s.wait("session admitted", func() bool {
		return s.pool.QueryRow(s.ctx, "SELECT id FROM automation_sessions WHERE automation_id=$1 AND participant_id=$2", automation, participant).Scan(&sid) == nil
	})
	return sid
}
func (s *system) requests(sid string) []delivery.Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []delivery.Request
	for _, r := range s.effects {
		if strings.HasPrefix(r.ID, sid+"/") {
			result = append(result, r)
		}
	}
	return result
}
func comment(platform, account, person, id, post, text string) inbound.Comment {
	c := inbound.Comment{Type: "CommentReceived", Version: 1, Platform: platform, AccountID: account, OccurredAt: time.Now().UTC()}
	own := false
	c.Data.AuthorID = person
	c.Data.CommentID = id
	c.Data.PostID = post
	c.Data.Own = &own
	c.Data.Text = &text
	return c
}
func dm(platform, account, person, id, text string) conversation.Message {
	m := conversation.Message{Type: "MessageReceived", Version: 1, Platform: platform, AccountID: account, OccurredAt: time.Now().UTC()}
	own := false
	m.Data.ParticipantID = person
	m.Data.MessageID = id
	m.Data.ConversationID = "thread-" + person
	m.Data.Text = &text
	m.Data.Own = &own
	return m
}
func document(account string) automation.Document {
	w := workflow.Definition{Steps: []workflow.Step{
		{ID: "welcome", Kind: "parallel", Actions: []workflow.Step{{ID: "public", Kind: "reply", Text: "Sent you a DM!"}, {ID: "ask", Kind: "dm", Text: "Your email?"}}},
		{ID: "capture", Kind: "wait_reply", PromptNodeID: "ask", Expected: "email", TimeoutHours: 1, InvalidBehavior: "retry", RetryMessage: "Please send one email", MaxAttempts: 3, TimeoutBehavior: "stop"},
		{ID: "mail", Kind: "email", TemplateID: "pricing"},
		{ID: "finish", Kind: "dm", Text: "Original flow complete"},
	}}
	raw, _ := json.Marshal(w)
	return automation.Document{Name: "Pricing", Trigger: automation.Trigger{SchemaVersion: 1, Scopes: []automation.Scope{{AccountID: account, Posts: automation.Posts{Mode: "selected", IDs: []string{"post"}}}}, Comment: automation.Predicate{Mode: "keywords", Operator: "whole_word", Logic: "any", Keywords: []string{"pricing"}, Exclusions: []string{"spam"}}, Repeat: automation.Repeat{Mode: "skip_active"}}, Workflow: raw}
}
func (s *system) configure(tenant, platform, account, id string) {
	s.t.Helper()
	s.apiCall("PUT", tenant+"/accounts/"+account, map[string]any{"platform": platform}, 204)
	s.apiCall("PUT", tenant+"/templates/pricing", map[string]any{"name": "Pricing", "subject": "Original pricing", "body": "Link for {{email}}: https://example.com/pricing", "revision": 0}, 204)
	s.syncRules(account)
	s.matches(platform, account, "post", "pricing", 0) // warm a genuine negative after registration was consumed
	s.apiCall("PUT", tenant+"/automations/"+id, document(account), 204)
	s.matches(platform, account, "post", "pricing", 0) // a draft never admits
	s.apiCall("POST", tenant+"/automations/"+id+"/publish", nil, 204)
	s.syncRules(account)
	s.matches(platform, account, "post", "pricing", 1)
}
