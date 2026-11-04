package execution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	db "blotato-comment-automation/internal/db/sqlcDB/execution"
	"blotato-comment-automation/internal/feature/conversation"
	"blotato-comment-automation/internal/feature/workflow"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrLease = errors.New("job lease is no longer current")
var ErrNotFound = errors.New("session not found")
var ErrConflict = errors.New("operation conflicts with session state")

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }
func terminal(status string) bool {
	return status == "completed" || status == "stopped" || status == "failed" || status == "cancelled"
}
func enqueue(ctx context.Context, q *db.Queries, sid, node, key, kind string, at time.Time, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return q.Enqueue(ctx, db.EnqueueParams{ID: sid + "/" + key, SessionID: sid, NodeID: node, LogicalKey: key, Kind: kind, RunAt: at, Payload: raw})
}

type frame struct {
	q         *db.Queries
	session   db.AutomationSession
	owner     db.ConversationOwner
	plan      workflow.Plan
	planError error
	steps     map[string]*db.SessionStep
	now       time.Time
}

func (s *Store) transaction(ctx context.Context, sid string, f func(*frame) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	q := db.New(tx)
	initial, err := q.GetSession(ctx, sid)
	if err != nil {
		return err
	}
	if err = q.EnsureConversation(ctx, db.EnsureConversationParams{TenantID: initial.TenantID, AccountID: initial.AccountID, ParticipantID: initial.ParticipantID}); err != nil {
		return err
	}
	owner, err := q.LockConversation(ctx, db.LockConversationParams{AccountID: initial.AccountID, ParticipantID: initial.ParticipantID})
	if err != nil {
		return err
	}
	session, err := q.LockSession(ctx, sid)
	if err != nil {
		return err
	}
	now, err := q.DatabaseTime(ctx)
	if err != nil {
		return err
	}
	state := &frame{q: q, session: session, owner: owner, now: now, steps: map[string]*db.SessionStep{}}
	version, err := q.GetPlan(ctx, db.GetPlanParams{TenantID: session.TenantID, AutomationID: session.AutomationID, Revision: session.Revision})
	if err != nil {
		return err
	}
	if len(version.ExecutionPlan) > 0 {
		state.planError = json.Unmarshal(version.ExecutionPlan, &state.plan)
	} else {
		var document struct {
			Workflow json.RawMessage `json:"workflow"`
		}
		state.planError = json.Unmarshal(version.Document, &document)
		if state.planError == nil {
			state.plan, state.planError = workflow.Compile(document.Workflow)
		}
	}
	if state.planError == nil && (state.plan.Version != 1 || state.plan.First == "") {
		state.planError = errors.New("unsupported compiled plan")
	}
	rows, err := q.ListSteps(ctx, sid)
	if err != nil {
		return err
	}
	for i := range rows {
		row := rows[i]
		state.steps[row.NodeID] = &row
	}
	if err = f(state); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (f *frame) init(ctx context.Context) error {
	for id, n := range f.plan.Nodes {
		if f.steps[id] != nil {
			continue
		}
		if err := f.q.InitStep(ctx, db.InitStepParams{SessionID: f.session.ID, NodeID: id, ParentNodeID: n.Parent}); err != nil {
			return err
		}
		f.steps[id] = &db.SessionStep{SessionID: f.session.ID, NodeID: id, ParentNodeID: n.Parent, Status: "pending", Output: []byte(`{}`)}
	}
	return nil
}
func (f *frame) save(ctx context.Context, event, from, reason string) error {
	for _, s := range f.steps {
		if err := f.q.SaveStep(ctx, db.SaveStepParams{SessionID: s.SessionID, NodeID: s.NodeID, Status: s.Status, WaitKind: s.WaitKind, ListenFromSequence: s.ListenFromSequence, DeadlineAt: s.DeadlineAt, InvalidAttempts: s.InvalidAttempts, Output: s.Output}); err != nil {
			return err
		}
	}
	if err := f.q.UpdateSession(ctx, db.UpdateSessionParams{ID: f.session.ID, Status: f.session.Status, CurrentNodeID: f.session.CurrentNodeID, Context: f.session.Context, FinishReason: f.session.FinishReason}); err != nil {
		return err
	}
	return f.q.Transition(ctx, db.TransitionParams{SessionID: f.session.ID, Version: f.session.Version + 1, NodeID: f.session.CurrentNodeID, EventID: event, FromState: from, ToState: f.session.Status, Reason: reason})
}
func (f *frame) finish(ctx context.Context, status, reason, currentJob string) error {
	f.session.Status = status
	f.session.FinishReason = reason
	for _, s := range f.steps {
		if s.Status == "waiting" || s.Status == "armed" || s.Status == "pending" {
			s.Status = "cancelled"
		}
	}
	if err := f.q.CancelJobs(ctx, db.CancelJobsParams{SessionID: f.session.ID, ID: currentJob}); err != nil {
		return err
	}
	return f.q.ReleaseConversation(ctx, db.ReleaseConversationParams{AccountID: f.session.AccountID, ParticipantID: f.session.ParticipantID, SessionID: &f.session.ID})
}
func (s *Store) Route(ctx context.Context, account, participant string) (conversation.Route, error) {
	row, err := db.New(s.pool).Route(ctx, db.RouteParams{AccountID: account, ParticipantID: participant})
	if errors.Is(err, pgx.ErrNoRows) {
		return conversation.Route{}, nil
	}
	if err != nil {
		return conversation.Route{}, err
	}
	route := conversation.Route{Epoch: row.Epoch, Platform: row.Platform}
	if row.Active && row.SessionID != nil {
		route.SessionID = *row.SessionID
	}
	return route, nil
}
func (s *Store) Activate(ctx context.Context, a conversation.Activation) error {
	return s.transaction(ctx, a.SessionID, func(f *frame) error {
		if f.session.AccountID != a.AccountID || f.session.ParticipantID != a.ParticipantID {
			return errors.New("activation identity does not match session")
		}
		var origin struct {
			Platform string `json:"platform"`
		}
		if err := json.Unmarshal(f.session.Origin, &origin); err != nil {
			return err
		}
		if origin.Platform != a.Platform {
			return errors.New("activation platform mismatch")
		}
		if terminal(f.session.Status) || f.owner.SessionID == nil || *f.owner.SessionID != a.SessionID || f.owner.Epoch != a.Epoch {
			return nil
		}
		if _, err := f.q.ActivateConversation(ctx, db.ActivateConversationParams{AccountID: a.AccountID, ParticipantID: a.ParticipantID, SessionID: &a.SessionID, Epoch: a.Epoch}); err != nil {
			return err
		}
		return enqueue(ctx, f.q, a.SessionID, "", fmt.Sprintf("activated:%d", a.Epoch), "advance", f.now, map[string]any{})
	})
}
func (s *Store) Accept(ctx context.Context, m conversation.Message, route conversation.Route) error {
	return s.transaction(ctx, route.SessionID, func(f *frame) error {
		if !f.owner.Active || f.owner.SessionID == nil || *f.owner.SessionID != route.SessionID || f.owner.Epoch != route.Epoch || terminal(f.session.Status) || f.session.AccountID != m.AccountID || f.session.ParticipantID != m.Data.ParticipantID {
			return conversation.ErrStale
		}
		sequence, err := f.q.NextSequence(ctx, db.NextSequenceParams{AccountID: m.AccountID, ParticipantID: m.Data.ParticipantID})
		if err != nil {
			return err
		}
		raw, err := json.Marshal(m)
		if err != nil {
			return err
		}
		id, err := f.q.InsertDM(ctx, db.InsertDMParams{AccountID: m.AccountID, ParticipantID: m.Data.ParticipantID, ConversationID: m.Data.ConversationID, ProviderMessageID: m.Data.MessageID, SessionID: route.SessionID, ConversationEpoch: route.Epoch, Sequence: sequence, Payload: raw})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		return enqueue(ctx, f.q, route.SessionID, "", fmt.Sprintf("dm:%d", id), "advance", f.now, map[string]any{})
	})
}
func (f *frame) job(ctx context.Context, j db.ExecutionJob) (db.ExecutionJob, error) {
	current, err := f.q.LockJob(ctx, j.ID)
	if err != nil {
		return current, err
	}
	if current.SessionID != f.session.ID || current.Status != "leased" || current.LeaseToken != j.LeaseToken || current.LeaseUntil == nil || !current.LeaseUntil.After(f.now) {
		return current, ErrLease
	}
	return current, nil
}
