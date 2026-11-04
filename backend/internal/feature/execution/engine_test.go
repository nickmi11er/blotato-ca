package execution_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	db "blotato-comment-automation/internal/db/sqlcDB/execution"
	"blotato-comment-automation/internal/feature/admission"
	"blotato-comment-automation/internal/feature/automation"
	"blotato-comment-automation/internal/feature/conversation"
	"blotato-comment-automation/internal/feature/delivery"
	"blotato-comment-automation/internal/feature/execution"
	"blotato-comment-automation/internal/feature/inbound"
	"blotato-comment-automation/internal/feature/workflow"
	"blotato-comment-automation/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

type publisher struct{ router *conversation.Router }

func (p publisher) PublishActivation(ctx context.Context, a conversation.Activation) error {
	return p.router.Activate(ctx, a)
}

type sender struct {
	base delivery.Sender
	hook func(context.Context, delivery.Request) delivery.Result
	safe bool
}

func (s *sender) Idempotent() bool { return s.safe }
func (s *sender) Send(ctx context.Context, r delivery.Request) delivery.Result {
	if s.hook != nil {
		return s.hook(ctx, r)
	}
	return s.base.Send(ctx, r)
}

type fixture struct {
	pool    *pgxpool.Pool
	store   *execution.Store
	engine  *execution.Engine
	worker  *execution.Worker
	router  *conversation.Router
	sender  *sender
	service *automation.Service
	sid     string
}

func definition() workflow.Definition {
	return workflow.Definition{Steps: []workflow.Step{
		{ID: "welcome", Kind: "parallel", Actions: []workflow.Step{{ID: "public", Kind: "reply", Text: "Sent you a DM!"}, {ID: "ask", Kind: "dm", Text: "Your email?"}}},
		{ID: "capture", Kind: "wait_reply", Expected: "email", TimeoutHours: 1, InvalidBehavior: "retry", RetryMessage: "Please send one email", MaxAttempts: 3, TimeoutBehavior: "stop"},
		{ID: "delay", Kind: "delay", Minutes: 60},
		{ID: "mail", Kind: "email", TemplateID: "pricing"},
	}, Templates: map[string]workflow.Template{"pricing": {Revision: 1, Subject: "Pricing", Body: "Link for {{email}}: https://example.com/pricing"}}}
}
func setup(t *testing.T, d workflow.Definition) *fixture {
	t.Helper()
	pool := testutil.Database(t)
	service := automation.NewService(pool)
	ctx := t.Context()
	if err := service.RegisterAccount(ctx, "tenant", "account", "instagram"); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(d)
	document := automation.Document{Name: "Pricing", Trigger: automation.Trigger{SchemaVersion: 1, Scopes: []automation.Scope{{AccountID: "account", Posts: automation.Posts{Mode: "all"}}}, Comment: automation.Predicate{Mode: "any", Operator: "exact", Logic: "any"}, Repeat: automation.Repeat{Mode: "every"}}, Workflow: raw}
	if err := service.Save(ctx, "tenant", "automation", document); err != nil {
		t.Fatal(err)
	}
	if err := service.Publish(ctx, "tenant", "automation"); err != nil {
		t.Fatal(err)
	}
	store := execution.NewStore(pool)
	engine := execution.NewEngine(store, 3)
	router := conversation.NewRouter(store, conversation.DefaultConfig())
	sender := &sender{base: delivery.NewRecorder(pool), safe: true}
	worker := execution.NewWorker(store, engine, sender, publisher{router}, execution.DefaultConfig())
	c := inbound.Comment{Type: "CommentReceived", Version: 1, Platform: "instagram", AccountID: "account", OccurredAt: time.Now()}
	c.Data.PostID = "post"
	c.Data.CommentID = "comment"
	c.Data.AuthorID = "person"
	own, text := false, "pricing"
	c.Data.Own = &own
	c.Data.Text = &text
	if _, err := admission.NewRepository(pool).Record(ctx, c, []automation.Match{{TenantID: "tenant", AutomationID: "automation", Revision: 1, Repeat: "every"}}); err != nil {
		t.Fatal(err)
	}
	var sid string
	if err := pool.QueryRow(ctx, "SELECT id FROM automation_sessions").Scan(&sid); err != nil {
		t.Fatal(err)
	}
	return &fixture{pool: pool, store: store, engine: engine, worker: worker, router: router, sender: sender, service: service, sid: sid}
}
func (f *fixture) drain(t *testing.T) {
	t.Helper()
	for range 100 {
		worked, err := f.worker.Once(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if !worked {
			return
		}
	}
	t.Fatal("job loop did not settle")
}
func (f *fixture) session(t *testing.T) db.AutomationSession {
	t.Helper()
	s, err := db.New(f.pool).GetSession(t.Context(), f.sid)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func (f *fixture) dm(t *testing.T, id, text string) {
	t.Helper()
	if err := f.router.Message(t.Context(), dm(id, text)); err != nil {
		t.Fatal(err)
	}
}
func dm(id, text string) conversation.Message {
	m := conversation.Message{Type: "MessageReceived", Version: 1, Platform: "instagram", AccountID: "account", OccurredAt: time.Now()}
	m.Data.ParticipantID = "person"
	m.Data.ConversationID = "conversation"
	m.Data.MessageID = id
	m.Data.Text = &text
	own := false
	m.Data.Own = &own
	return m
}
func count(t *testing.T, p *pgxpool.Pool, sql string) int {
	t.Helper()
	var n int
	if err := p.QueryRow(t.Context(), sql).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
func TestEarlyReplyParallelDelayAndPinnedTemplate(t *testing.T) {
	f := setup(t, definition())
	// Cache absence before activation; the activation handler must replace it.
	f.dm(t, "unrelated", "hello")
	f.sender.hook = func(ctx context.Context, r delivery.Request) delivery.Result {
		receipt := f.sender.base.Send(ctx, r)
		if r.Kind == "dm" && r.Purpose == "action" {
			if err := f.router.Message(ctx, dm("fast", "Use me@example.com")); err != nil {
				t.Error(err)
			}
		}
		return receipt
	}
	f.drain(t)
	s := f.session(t)
	if s.Status != "waiting" || s.CurrentNodeID != "delay" {
		t.Fatalf("early reply lost: %+v", s)
	}
	if count(t, f.pool, "SELECT count(*) FROM recorded_deliveries") != 2 {
		t.Fatal("parallel actions not sent independently")
	}
	if _, err := f.pool.Exec(t.Context(), "UPDATE session_steps SET deadline_at=clock_timestamp()-interval '1 second' WHERE node_id='delay'; UPDATE execution_jobs SET run_at=clock_timestamp()-interval '1 second' WHERE logical_key='timer:delay'"); err != nil {
		t.Fatal(err)
	}
	// A different process resumes from persisted state; no in-memory session is needed.
	f.worker = execution.NewWorker(execution.NewStore(f.pool), execution.NewEngine(execution.NewStore(f.pool), 3), f.sender, publisher{f.router}, execution.DefaultConfig())
	f.drain(t)
	s = f.session(t)
	if s.Status != "completed" {
		t.Fatal(s.Status, s.FinishReason)
	}
	var request delivery.Request
	var raw []byte
	if err := f.pool.QueryRow(t.Context(), "SELECT payload FROM recorded_deliveries WHERE payload->>'kind'='email'").Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &request); err != nil || request.Email != "me@example.com" || request.Text != "Link for me@example.com: https://example.com/pricing" {
		t.Fatal(request, err)
	}
	if count(t, f.pool, "SELECT count(*) FROM conversation_owners WHERE session_id IS NOT NULL") != 0 {
		t.Fatal("ownership leaked")
	}
	f.dm(t, "fast", "Use me@example.com")
	f.drain(t)
	if count(t, f.pool, "SELECT count(*) FROM recorded_deliveries") != 3 {
		t.Fatal("duplicate send")
	}
}
func TestInvalidReplyDedupAndOriginalDeadline(t *testing.T) {
	f := setup(t, definition())
	f.drain(t)
	before, _ := db.New(f.pool).ListSteps(t.Context(), f.sid)
	var deadline time.Time
	for _, s := range before {
		if s.NodeID == "capture" {
			deadline = *s.DeadlineAt
		}
	}
	f.dm(t, "bad1", "not an email")
	f.dm(t, "bad1", "not an email")
	f.drain(t)
	steps, _ := db.New(f.pool).ListSteps(t.Context(), f.sid)
	for _, s := range steps {
		if s.NodeID == "capture" && (s.InvalidAttempts != 1 || !s.DeadlineAt.Equal(deadline)) {
			t.Fatal("duplicate counted or deadline extended", s)
		}
	}
	f.dm(t, "bad2", "still no")
	f.drain(t)
	f.dm(t, "bad3", "invalid")
	f.drain(t)
	if s := f.session(t); s.Status != "stopped" || s.FinishReason != "invalid_reply_limit" {
		t.Fatal(s.Status, s.FinishReason)
	}
	if count(t, f.pool, "SELECT count(*) FROM recorded_deliveries") != 4 {
		t.Fatal("retry count incorrect")
	}
}
func TestReplyAcceptedBeforeDeadlineWinsDelayedTimer(t *testing.T) {
	f := setup(t, definition())
	f.drain(t)
	f.dm(t, "valid", "me@example.com")
	if _, err := f.pool.Exec(t.Context(), "UPDATE session_steps SET deadline_at=clock_timestamp() WHERE node_id='capture'; UPDATE execution_jobs SET run_at=clock_timestamp()-interval '1 second' WHERE logical_key='timeout:capture'"); err != nil {
		t.Fatal(err)
	}
	f.drain(t)
	if s := f.session(t); s.Status != "waiting" || s.CurrentNodeID != "delay" {
		t.Fatal("timeout overtook accepted reply", s)
	}
}
func TestTimeoutBranchDoesNotReturnToMainPath(t *testing.T) {
	d := definition()
	d.Steps[1].TimeoutBehavior = "branch"
	d.Steps[1].TimeoutSteps = []workflow.Step{{ID: "expire", Kind: "dm", Text: "Expired"}}
	f := setup(t, d)
	f.drain(t)
	if _, err := f.pool.Exec(t.Context(), "UPDATE session_steps SET deadline_at=clock_timestamp()-interval '1 second' WHERE node_id='capture'; UPDATE execution_jobs SET run_at=clock_timestamp()-interval '1 second' WHERE logical_key='timeout:capture'"); err != nil {
		t.Fatal(err)
	}
	f.drain(t)
	if s := f.session(t); s.Status != "completed" {
		t.Fatal(s.Status, s.FinishReason)
	}
	if count(t, f.pool, "SELECT count(*) FROM recorded_deliveries WHERE payload->>'kind'='email'") != 0 {
		t.Fatal("timeout returned to main path")
	}
}
func TestUnknownDeliveryManualResolutionAndCancellation(t *testing.T) {
	f := setup(t, workflow.Definition{Steps: []workflow.Step{{ID: "send", Kind: "dm", Text: "hello"}}})
	f.sender.safe = false
	f.sender.hook = func(context.Context, delivery.Request) delivery.Result {
		return delivery.Result{Status: "unknown", Reason: "connection lost"}
	}
	f.drain(t)
	if s := f.session(t); s.Status != "waiting" || s.FinishReason != "delivery_unknown" {
		t.Fatal(s)
	}
	if err := f.engine.Resolve(t.Context(), "other", f.sid, f.sid+"/send:send", delivery.Result{Status: "succeeded", ProviderMessageID: "receipt"}); !errors.Is(err, execution.ErrNotFound) {
		t.Fatal("cross tenant resolution", err)
	}
	if err := f.engine.Resolve(t.Context(), "tenant", f.sid, f.sid+"/send:send", delivery.Result{Status: "succeeded", ProviderMessageID: "receipt"}); err != nil {
		t.Fatal(err)
	}
	if s := f.session(t); s.Status != "completed" {
		t.Fatal(s)
	}
	g := setup(t, definition())
	g.drain(t)
	if err := g.engine.Cancel(t.Context(), "tenant", g.sid); err != nil {
		t.Fatal(err)
	}
	g.dm(t, "late", "me@example.com")
	g.drain(t)
	if s := g.session(t); s.Status != "cancelled" {
		t.Fatal("late event revived cancellation", s)
	}
}
func TestExpiredLeaseFencesOldWorkerAndRetriesAreIdempotent(t *testing.T) {
	f := setup(t, workflow.Definition{Steps: []workflow.Step{{ID: "send", Kind: "dm", Text: "hello"}}})
	if _, err := f.worker.Once(t.Context()); err != nil {
		t.Fatal(err)
	}
	q := db.New(f.pool)
	old, err := q.ClaimJob(t.Context(), 30000)
	if err != nil {
		t.Fatal(err)
	}
	var request delivery.Request
	_ = json.Unmarshal(old.Payload, &request)
	receipt := f.sender.Send(t.Context(), request)
	if _, err = f.pool.Exec(t.Context(), "UPDATE execution_jobs SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1", old.ID); err != nil {
		t.Fatal(err)
	}
	if err = f.worker.Recover(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err = f.engine.Complete(t.Context(), old, receipt); !errors.Is(err, execution.ErrLease) {
		t.Fatal("stale worker committed", err)
	}
	f.drain(t)
	if count(t, f.pool, "SELECT count(*) FROM recorded_deliveries") != 1 || f.session(t).Status != "completed" {
		t.Fatal("recovery duplicated or lost delivery")
	}
}
func TestConcurrentWorkersCreateOneSend(t *testing.T) {
	f := setup(t, workflow.Definition{Steps: []workflow.Step{{ID: "send", Kind: "dm", Text: "hello"}}})
	if _, err := f.worker.Once(t.Context()); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if _, err := f.worker.Once(t.Context()); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if count(t, f.pool, "SELECT count(*) FROM recorded_deliveries") != 1 || f.session(t).Status != "completed" {
		t.Fatal("concurrent claim duplicated send")
	}
}

func TestReplyToRetryReceiptIsBufferedUntilConfirmation(t *testing.T) {
	f := setup(t, definition())
	f.drain(t)
	f.dm(t, "invalid", "not an email")
	// Process the reply and enqueue a re-ask, but hold its external confirmation.
	if _, err := f.worker.Once(t.Context()); err != nil {
		t.Fatal(err)
	}
	q := db.New(f.pool)
	job, err := q.ClaimJob(t.Context(), 30000)
	if err != nil {
		t.Fatal(err)
	}
	var request delivery.Request
	if err = json.Unmarshal(job.Payload, &request); err != nil || request.Purpose != "retry" {
		t.Fatal("expected re-ask", request, err)
	}
	receipt := f.sender.Send(t.Context(), request)
	reply := dm("quick-retry-answer", "me@example.com")
	reply.Data.ReplyTo = receipt.ProviderMessageID
	if err = f.router.Message(t.Context(), reply); err != nil {
		t.Fatal(err)
	}
	// A different worker handles the DM before the send result is committed.
	if _, err = f.worker.Once(t.Context()); err != nil {
		t.Fatal(err)
	}
	if count(t, f.pool, "SELECT count(*) FROM session_inbox WHERE provider_message_id='quick-retry-answer' AND consumed_at IS NULL") != 1 {
		t.Fatal("unconfirmed reply-to discarded")
	}
	if err = f.engine.Complete(t.Context(), job, receipt); err != nil {
		t.Fatal(err)
	}
	if s := f.session(t); s.CurrentNodeID != "delay" {
		t.Fatal("buffered reply was not resumed", s)
	}
}
func TestObsoleteLeasedRetryIsNotSent(t *testing.T) {
	f := setup(t, definition())
	f.drain(t)
	f.dm(t, "invalid", "bad")
	if _, err := f.worker.Once(t.Context()); err != nil {
		t.Fatal(err)
	}
	q := db.New(f.pool)
	job, err := q.ClaimJob(t.Context(), 30000)
	if err != nil {
		t.Fatal(err)
	}
	f.dm(t, "valid", "me@example.com")
	if _, err = f.worker.Once(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err = f.engine.CheckSend(t.Context(), job); !errors.Is(err, execution.ErrLease) {
		t.Fatal("obsolete question permitted", err)
	}
	current, err := q.LockJob(t.Context(), job.ID)
	if err != nil || current.Status != "cancelled" {
		t.Fatal(current, err)
	}
	if count(t, f.pool, "SELECT count(*) FROM recorded_deliveries") != 2 {
		t.Fatal("unexpected retry send")
	}
}
func TestConversationOwnershipAndPriority(t *testing.T) {
	f := setup(t, definition())
	if err := f.engine.Cancel(t.Context(), "tenant", f.sid); err != nil {
		t.Fatal(err)
	}
	original, err := f.service.Get(t.Context(), "tenant", "automation")
	if err != nil {
		t.Fatal(err)
	}
	original.Draft.Priority = 10
	if err = f.service.Save(t.Context(), "tenant", "preferred", original.Draft); err != nil {
		t.Fatal(err)
	}
	if err = f.service.Publish(t.Context(), "tenant", "preferred"); err != nil {
		t.Fatal(err)
	}
	comment := inbound.Comment{Type: "CommentReceived", Platform: "instagram", AccountID: "account"}
	comment.Data.PostID = "post"
	comment.Data.CommentID = "competing"
	comment.Data.AuthorID = "person"
	matches := []automation.Match{{TenantID: "tenant", AutomationID: "automation", Revision: 1, Repeat: "every"}, {TenantID: "tenant", AutomationID: "preferred", Revision: 1, Repeat: "every", Priority: 10}}
	if _, err = admission.NewRepository(f.pool).Record(t.Context(), comment, matches); err != nil {
		t.Fatal(err)
	}
	var raw []byte
	if err = f.pool.QueryRow(t.Context(), "SELECT outcomes FROM comment_decisions WHERE comment_id='competing'").Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var outcomes []admission.Outcome
	if err = json.Unmarshal(raw, &outcomes); err != nil {
		t.Fatal(err)
	}
	if len(outcomes) != 2 || outcomes[0].AutomationID != "preferred" || outcomes[0].Denied || outcomes[1].Reason != "conversation_busy" {
		t.Fatalf("unexpected arbitration: %+v", outcomes)
	}
}
func TestUnknownRecoveryDoesNotResendWithoutIdempotency(t *testing.T) {
	f := setup(t, workflow.Definition{Steps: []workflow.Step{{ID: "send", Kind: "dm", Text: "hello"}}})
	f.sender.safe = false
	if _, err := f.worker.Once(t.Context()); err != nil {
		t.Fatal(err)
	}
	q := db.New(f.pool)
	job, err := q.ClaimJob(t.Context(), 30000)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.pool.Exec(t.Context(), "UPDATE execution_jobs SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1", job.ID); err != nil {
		t.Fatal(err)
	}
	if err = f.worker.Recover(t.Context()); err != nil {
		t.Fatal(err)
	}
	f.drain(t)
	current, err := q.LockJob(t.Context(), job.ID)
	if err != nil || current.Status != "unknown" || count(t, f.pool, "SELECT count(*) FROM recorded_deliveries") != 0 {
		t.Fatal("uncertain delivery retried", current, err)
	}
}

func TestLateReaskFailureDoesNotFailAdvancedSession(t *testing.T) {
	for _, outcome := range []string{"failed", "retry", "unknown"} {
		t.Run(outcome, func(t *testing.T) {
			f := setup(t, definition())
			f.drain(t)
			f.dm(t, "invalid", "bad")
			if _, err := f.worker.Once(t.Context()); err != nil {
				t.Fatal(err)
			}
			job, err := db.New(f.pool).ClaimJob(t.Context(), 30000)
			if err != nil {
				t.Fatal(err)
			}
			if err = f.engine.CheckSend(t.Context(), job); err != nil {
				t.Fatal(err)
			}
			// Sending has started; the participant answers before its result arrives.
			f.dm(t, "valid", "me@example.com")
			if _, err = f.worker.Once(t.Context()); err != nil {
				t.Fatal(err)
			}
			if err = f.engine.Complete(t.Context(), job, delivery.Result{Status: outcome, Reason: "late result"}); err != nil {
				t.Fatal(err)
			}
			if s := f.session(t); s.Status != "waiting" || s.CurrentNodeID != "delay" || s.FinishReason != "" {
				t.Fatal("obsolete re-ask affected next step", s)
			}
			current, err := db.New(f.pool).LockJob(t.Context(), job.ID)
			if err != nil || current.Status == "ready" {
				t.Fatal("obsolete re-ask retried", current, err)
			}
		})
	}
}
