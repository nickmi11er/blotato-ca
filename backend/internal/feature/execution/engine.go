package execution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	db "blotato-comment-automation/internal/db/sqlcDB/execution"
	"blotato-comment-automation/internal/feature/conversation"
	"blotato-comment-automation/internal/feature/delivery"
	"blotato-comment-automation/internal/feature/workflow"
	"github.com/jackc/pgx/v5"
)

type Engine struct {
	store       *Store
	maxAttempts int
}

func NewEngine(store *Store, maxAttempts int) *Engine {
	return &Engine{store: store, maxAttempts: maxAttempts}
}

type variables struct {
	Contact struct {
		Email string `json:"email,omitempty"`
	} `json:"contact"`
	FirstName string `json:"first_name,omitempty"`
}
type stepOutput struct {
	ProviderMessageID string   `json:"provider_message_id,omitempty"`
	RetryMessageIDs   []string `json:"retry_message_ids,omitempty"`
	Email             string   `json:"email,omitempty"`
	Text              string   `json:"text,omitempty"`
}

func output(s *db.SessionStep) stepOutput {
	var v stepOutput
	_ = json.Unmarshal(s.Output, &v)
	return v
}
func setOutput(s *db.SessionStep, v stepOutput) { s.Output, _ = json.Marshal(v) }
func (f *frame) arm(waitID string) {
	s := f.steps[waitID]
	if s != nil && s.Status == "pending" {
		s.Status = "armed"
		s.WaitKind = "reply"
		s.ListenFromSequence = f.owner.NextSequence
	}
}
func (f *frame) send(ctx context.Context, node workflow.Node, purpose, key, text string) error {
	var origin struct {
		Platform string `json:"platform"`
		Data     struct {
			PostID    string `json:"post_id"`
			CommentID string `json:"comment_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(f.session.Origin, &origin); err != nil {
		return err
	}
	var vars variables
	if err := json.Unmarshal(f.session.Context, &vars); err != nil {
		return err
	}
	kind := node.Step.Kind
	if purpose != "action" {
		kind = "dm"
	}
	request := delivery.Request{ID: f.session.ID + "/" + key, Kind: kind, Platform: origin.Platform, AccountID: f.session.AccountID, ParticipantID: f.session.ParticipantID, PostID: origin.Data.PostID, CommentID: origin.Data.CommentID, Purpose: purpose}
	request.Text = workflow.Render(text, vars.Contact.Email, vars.FirstName)
	if kind == "email" {
		template, ok := f.plan.Templates[node.Step.TemplateID]
		if !ok || vars.Contact.Email == "" {
			return errors.New("missing email or template at execution")
		}
		request.Email = vars.Contact.Email
		request.Subject = workflow.Render(template.Subject, vars.Contact.Email, vars.FirstName)
		request.Text = workflow.Render(template.Body, vars.Contact.Email, vars.FirstName)
	}
	if node.ArmWait != "" {
		f.arm(node.ArmWait)
	}
	return enqueue(ctx, f.q, f.session.ID, node.Step.ID, key, "deliver", f.now, request)
}
func (f *frame) enterAction(ctx context.Context, node workflow.Node) error {
	step := f.steps[node.Step.ID]
	if step.Status != "pending" {
		return nil
	}
	step.Status = "waiting"
	step.WaitKind = "action"
	return f.send(ctx, node, "action", "send:"+node.Step.ID, node.Step.Text)
}
func (f *frame) next(ctx context.Context, node workflow.Node, currentJob string) error {
	if err := f.q.CancelWaitJobs(ctx, db.CancelWaitJobsParams{SessionID: f.session.ID, NodeID: node.Step.ID, ID: currentJob}); err != nil {
		return err
	}
	f.steps[node.Step.ID].Status = "succeeded"
	f.session.CurrentNodeID = node.Next
	return nil
}
func (f *frame) run(ctx context.Context, currentJob string) error {
	if f.planError != nil {
		return f.finish(ctx, "failed", "invalid_published_workflow", currentJob)
	}
	if err := f.init(ctx); err != nil {
		return err
	}
	if f.session.Status == "pending" {
		f.session.CurrentNodeID = f.plan.First
	}
	if f.plan.HasReply && (!f.owner.Active || f.owner.SessionID == nil || *f.owner.SessionID != f.session.ID) {
		// Previously-created pending sessions may predate conversation reservations.
		if f.owner.SessionID != nil && *f.owner.SessionID != f.session.ID {
			return f.finish(ctx, "stopped", "conversation_busy", currentJob)
		}
		epoch := f.owner.Epoch
		if f.owner.SessionID == nil {
			var err error
			epoch, err = f.q.ReserveConversation(ctx, db.ReserveConversationParams{AccountID: f.session.AccountID, ParticipantID: f.session.ParticipantID, SessionID: &f.session.ID})
			if err != nil {
				return err
			}
		}
		var origin struct {
			Platform string `json:"platform"`
		}
		if err := json.Unmarshal(f.session.Origin, &origin); err != nil {
			return err
		}
		activation := conversation.Activation{Type: "ActivateConversation", Version: 1, Platform: origin.Platform, AccountID: f.session.AccountID, ParticipantID: f.session.ParticipantID, SessionID: f.session.ID, Epoch: epoch}
		if err := enqueue(ctx, f.q, f.session.ID, "", fmt.Sprintf("activate:%d", epoch), "activate", f.now, activation); err != nil {
			return err
		}
		f.session.Status = "waiting"
		return nil
	}
	f.session.Status = "running"
	for range 160 {
		if f.session.CurrentNodeID == "" {
			return f.finish(ctx, "completed", "workflow_finished", currentJob)
		}
		node, ok := f.plan.Nodes[f.session.CurrentNodeID]
		if !ok {
			return f.finish(ctx, "failed", "unknown_node", currentJob)
		}
		step := f.steps[node.Step.ID]
		if step.Status == "succeeded" {
			f.session.CurrentNodeID = node.Next
			continue
		}
		switch node.Step.Kind {
		case "reply", "dm", "email":
			if err := f.enterAction(ctx, node); err != nil {
				return err
			}
		case "parallel":
			step.Status = "waiting"
			step.WaitKind = "parallel"
			complete := true
			for _, id := range node.Children {
				if err := f.enterAction(ctx, f.plan.Nodes[id]); err != nil {
					return err
				}
				complete = complete && f.steps[id].Status == "succeeded"
			}
			if complete {
				if err := f.next(ctx, node, currentJob); err != nil {
					return err
				}
				continue
			}
		case "delay", "schedule":
			if step.DeadlineAt == nil {
				due, err := workflow.Due(node.Step, f.now)
				if err != nil {
					return f.finish(ctx, "failed", "invalid_schedule", currentJob)
				}
				step.DeadlineAt = &due
				step.Status = "waiting"
				step.WaitKind = "timer"
				if due.After(f.now) {
					if err = enqueue(ctx, f.q, f.session.ID, node.Step.ID, "timer:"+node.Step.ID, "wake", due, map[string]any{}); err != nil {
						return err
					}
				}
			}
			if !step.DeadlineAt.After(f.now) {
				if err := f.next(ctx, node, currentJob); err != nil {
					return err
				}
				continue
			}
		case "wait_reply":
			if step.Status == "pending" {
				f.arm(node.Step.ID)
			}
			if step.DeadlineAt == nil {
				due := f.now.Add(time.Duration(node.Step.TimeoutHours) * time.Hour)
				step.DeadlineAt = &due
				step.Status = "waiting"
				step.WaitKind = "reply"
				if err := enqueue(ctx, f.q, f.session.ID, node.Step.ID, "timeout:"+node.Step.ID, "wake", due, map[string]any{}); err != nil {
					return err
				}
			}
			advance, err := f.replies(ctx, node, currentJob)
			if err != nil {
				return err
			}
			if terminal(f.session.Status) {
				return nil
			}
			if advance {
				continue
			}
		default:
			return f.finish(ctx, "failed", "unsupported_node", currentJob)
		}
		f.session.Status = "waiting"
		return nil
	}
	return errors.New("workflow exceeded transition bound")
}
func (f *frame) replies(ctx context.Context, node workflow.Node, currentJob string) (bool, error) {
	step := f.steps[node.Step.ID]
	for range 16 {
		message, err := f.q.NextReply(ctx, db.NextReplyParams{SessionID: f.session.ID, Sequence: step.ListenFromSequence, AcceptedAt: *step.DeadlineAt})
		if errors.Is(err, pgx.ErrNoRows) {
			if f.now.Before(*step.DeadlineAt) {
				return false, nil
			}
			// A final timeout DM remains a waiting step until that send is confirmed.
			switch node.Step.TimeoutBehavior {
			case "branch":
				step.Status = "stopped"
				step.WaitKind = "timeout"
				if err = f.q.CancelWaitJobs(ctx, db.CancelWaitJobsParams{SessionID: f.session.ID, NodeID: node.Step.ID, ID: currentJob}); err != nil {
					return false, err
				}
				f.session.CurrentNodeID = node.TimeoutNext
				return true, nil
			case "dm":
				step.WaitKind = "timeout_delivery"
				return false, f.send(ctx, node, "timeout", "timeout-dm:"+node.Step.ID, node.Step.TimeoutMessage)
			default:
				return false, f.finish(ctx, "stopped", "reply_timeout", currentJob)
			}
		}
		if err != nil {
			return false, err
		}
		var dm conversation.Message
		if err = json.Unmarshal(message.Payload, &dm); err != nil {
			return false, err
		}
		if message.ConversationEpoch != f.owner.Epoch || dm.Data.Text == nil {
			if err = f.q.ConsumeReply(ctx, message.ID); err != nil {
				return false, err
			}
			continue
		}
		if dm.Data.ReplyTo != "" && node.Step.PromptNodeID != "" {
			valid := dm.Data.ReplyTo == output(f.steps[node.Step.PromptNodeID]).ProviderMessageID
			for _, id := range output(step).RetryMessageIDs {
				valid = valid || dm.Data.ReplyTo == id
			}
			if !valid {
				// A reply can beat the sender's receipt transaction. Do not consume
				// it until outstanding re-asks establish their provider IDs.
				jobs, err := f.q.ListJobs(ctx, f.session.ID)
				if err != nil {
					return false, err
				}
				for _, job := range jobs {
					if job.NodeID == node.Step.ID && job.Kind == "deliver" && (job.Status == "ready" || job.Status == "leased" || job.Status == "unknown") {
						var request delivery.Request
						if json.Unmarshal(job.Payload, &request) == nil && request.Purpose == "retry" {
							return false, nil
						}
					}
				}
				if err = f.q.ConsumeReply(ctx, message.ID); err != nil {
					return false, err
				}
				continue
			}
		}
		if err = f.q.ConsumeReply(ctx, message.ID); err != nil {
			return false, err
		}
		text := strings.TrimSpace(*dm.Data.Text)
		if text == "" {
			continue
		}
		email := workflow.ExtractEmail(text)
		if node.Step.Expected == "email" && email == "" {
			step.InvalidAttempts++
			if node.Step.InvalidBehavior == "stop" || int(step.InvalidAttempts) >= node.Step.MaxAttempts {
				return false, f.finish(ctx, "stopped", "invalid_reply_limit", currentJob)
			}
			if err = f.send(ctx, node, "retry", fmt.Sprintf("retry:%s:%d", node.Step.ID, step.InvalidAttempts), node.Step.RetryMessage); err != nil {
				return false, err
			}
			continue
		}
		v := output(step)
		v.Text = text
		if node.Step.Expected == "email" {
			v.Email = email
			var vars variables
			if err = json.Unmarshal(f.session.Context, &vars); err != nil {
				return false, err
			}
			vars.Contact.Email = email
			f.session.Context, _ = json.Marshal(vars)
		}
		setOutput(step, v)
		return true, f.next(ctx, node, currentJob)
	}
	// Do not let a timer overtake already-accepted replies when a batch is full.
	return false, enqueue(ctx, f.q, f.session.ID, node.Step.ID, fmt.Sprintf("drain:%d", f.session.Version), "advance", f.now, map[string]any{})
}
func (e *Engine) Process(ctx context.Context, job db.ExecutionJob) error {
	return e.store.transaction(ctx, job.SessionID, func(f *frame) error {
		current, err := f.job(ctx, job)
		if err != nil {
			return err
		}
		rows, err := f.q.FinishJob(ctx, db.FinishJobParams{ID: current.ID, LeaseToken: current.LeaseToken, Status: "done", Result: []byte(`{}`)})
		if err != nil {
			return err
		}
		if rows != 1 {
			return ErrLease
		}
		if terminal(f.session.Status) {
			return nil
		}
		if job.Kind == "wake" && f.session.CurrentNodeID != job.NodeID {
			return nil
		}
		from := f.session.Status
		if err = f.run(ctx, job.ID); err != nil {
			return err
		}
		return f.save(ctx, job.ID, from, job.Kind)
	})
}
func (e *Engine) CheckSend(ctx context.Context, job db.ExecutionJob) error {
	obsolete := false
	err := e.store.transaction(ctx, job.SessionID, func(f *frame) error {
		if _, err := f.job(ctx, job); err != nil {
			return err
		}
		var request delivery.Request
		if err := json.Unmarshal(job.Payload, &request); err != nil {
			return err
		}
		step := f.steps[job.NodeID]
		obsolete = terminal(f.session.Status) || step == nil || step.Status != "waiting"
		if request.Purpose == "retry" && step != nil {
			obsolete = obsolete || step.WaitKind != "reply" || step.DeadlineAt == nil || !step.DeadlineAt.After(f.now)
		}
		if obsolete {
			rows, err := f.q.FinishJob(ctx, db.FinishJobParams{ID: job.ID, LeaseToken: job.LeaseToken, Status: "cancelled", Result: []byte(`{}`)})
			if err != nil {
				return err
			}
			if rows != 1 {
				return ErrLease
			}
			return enqueue(ctx, f.q, f.session.ID, job.NodeID, "obsolete:"+job.LogicalKey, "advance", f.now, map[string]any{})
		}
		if f.plan.HasReply && (!f.owner.Active || f.owner.SessionID == nil || *f.owner.SessionID != f.session.ID) {
			return ErrConflict
		}
		return nil
	})
	if err != nil {
		return err
	}
	if obsolete {
		return ErrLease
	}
	return nil
}
func (e *Engine) Complete(ctx context.Context, job db.ExecutionJob, result delivery.Result) error {
	if job.Kind == "deliver" && result.Status != "succeeded" {
		var previous delivery.Result
		if err := json.Unmarshal(job.Result, &previous); err != nil {
			return err
		}
		// A later rejection (including rate limiting) says nothing about an
		// earlier attempt whose receipt was lost. Only confirmation or explicit
		// reconciliation clears that uncertainty; Resolve bypasses this merge.
		result.Uncertain = result.Uncertain || previous.Uncertain
	}
	return e.store.transaction(ctx, job.SessionID, func(f *frame) error { return e.complete(ctx, f, job, result) })
}
func (e *Engine) complete(ctx context.Context, f *frame, job db.ExecutionJob, result delivery.Result) error {
	current, err := f.job(ctx, job)
	if err != nil {
		return err
	}
	if terminal(f.session.Status) {
		_, err = f.q.FinishJob(ctx, db.FinishJobParams{ID: job.ID, LeaseToken: job.LeaseToken, Status: "cancelled", Result: []byte(`{}`)})
		return err
	}
	from := f.session.Status
	var request delivery.Request
	if job.Kind == "deliver" {
		if err = json.Unmarshal(job.Payload, &request); err != nil {
			return err
		}
	}
	step := f.steps[job.NodeID]
	// An in-flight re-ask may finish after a valid answer advanced the workflow.
	// Preserve its outcome, but do not retry it or fail the subsequent step.
	obsoleteRetry := request.Purpose == "retry" && (step == nil || step.Status != "waiting" || step.WaitKind != "reply")
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if result.Status == "retry" && !obsoleteRetry && (job.Kind == "activate" || int(current.Attempt) < e.maxAttempts) {
		delay := max(time.Duration(1<<min(current.Attempt, 8))*time.Second, result.RetryAfter)
		_, err = f.q.RetryJob(ctx, db.RetryJobParams{ID: job.ID, LeaseToken: job.LeaseToken, Result: raw, DelayMs: delay.Milliseconds()})
		return err
	}
	status := "done"
	if result.Status == "unknown" {
		status = "unknown"
	}
	if result.Status == "failed" || result.Status == "retry" {
		status = "failed"
	}
	// Exhausting safe retries does not prove the external effect did not happen.
	// Preserve ambiguous outcomes for reconciliation instead of ending the run.
	if (result.Status == "retry" || result.Status == "failed") && result.Uncertain {
		status = "unknown"
	}
	rows, err := f.q.FinishJob(ctx, db.FinishJobParams{ID: job.ID, LeaseToken: job.LeaseToken, Status: status, Result: raw})
	if err != nil {
		return err
	}
	if rows != 1 {
		return ErrLease
	}
	if job.Kind == "activate" || obsoleteRetry {
		return nil
	}
	if status == "unknown" {
		f.session.Status = "waiting"
		f.session.FinishReason = "delivery_unknown"
		return f.save(ctx, job.ID, from, "delivery_unknown")
	}
	if status == "failed" {
		if err = f.finish(ctx, "failed", "delivery_failed", job.ID); err != nil {
			return err
		}
		return f.save(ctx, job.ID, from, result.Reason)
	}
	f.session.FinishReason = ""
	if err = f.init(ctx); err != nil {
		return err
	}
	step = f.steps[job.NodeID]
	if step == nil {
		return errors.New("delivery has no step")
	}
	switch request.Purpose {
	case "action":
		v := output(step)
		v.ProviderMessageID = result.ProviderMessageID
		setOutput(step, v)
		step.Status = "succeeded"
	case "retry":
		v := output(step)
		v.RetryMessageIDs = append(v.RetryMessageIDs, result.ProviderMessageID)
		setOutput(step, v)
	case "timeout":
		if err = f.finish(ctx, "stopped", "reply_timeout", job.ID); err != nil {
			return err
		}
		return f.save(ctx, job.ID, from, "reply_timeout")
	default:
		return errors.New("unknown delivery purpose")
	}
	if err = f.run(ctx, job.ID); err != nil {
		return err
	}
	return f.save(ctx, job.ID, from, "delivery_confirmed")
}
