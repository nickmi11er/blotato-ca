package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"blotato-comment-automation/internal/feature/delivery"
	"blotato-comment-automation/internal/feature/workflow"
)

func TestPublishedAutomationRunsPinnedVersionThroughKafka(t *testing.T) {
	s := newSystem(t, 2)
	s.configure("tenant", "instagram", "account", "pricing-flow")
	for i, input := range []struct{ post, text string }{
		{"other-post", "pricing"}, {"post", "repricing"}, {"post", "pricing spam"},
	} {
		s.send("instagram", "account", "ignored", comment("instagram", "account", "ignored", fmt.Sprint(i), input.post, input.text), 1)
	}
	own := comment("instagram", "account", "ignored", "own", "post", "pricing")
	*own.Data.Own = true
	s.send("instagram", "account", "ignored", own, 1)
	if n := s.count("SELECT count(*) FROM automation_sessions"); n != 0 {
		t.Fatalf("irrelevant comments started %d sessions", n)
	}
	if n := s.count("SELECT count(*) FROM comment_decisions"); n != 0 {
		t.Fatalf("irrelevant comments wrote %d decisions", n)
	}
	c := comment("instagram", "account", "person", "comment", "post", "PRICING, please!")
	s.send("instagram", "account", "person", c, 3)
	sid := s.sessionID("pricing-flow", "person")
	s.wait("email wait", func() bool { v := s.session("tenant", sid); return v.Status == "waiting" && v.Node == "capture" })
	s.send("instagram", "account", "person", comment("instagram", "account", "person", "second-comment", "post", "pricing"), 1)
	if n := s.count("SELECT count(*) FROM automation_sessions"); n != 1 {
		t.Fatalf("duplicates/repeat policy started %d sessions", n)
	}

	// A running session must retain both its graph and its email template snapshot.
	s.apiCall("PUT", "tenant/templates/pricing", map[string]any{"name": "Pricing", "subject": "Changed pricing", "body": "Changed body", "revision": 1}, 204)
	doc := document("account")
	var graph workflow.Definition
	if err := json.Unmarshal(doc.Workflow, &graph); err != nil {
		t.Fatal(err)
	}
	graph.Steps[3].Text = "Changed flow"
	doc.Workflow, _ = json.Marshal(graph)
	s.apiCall("PUT", "tenant/automations/pricing-flow", doc, 204)
	s.apiCall("POST", "tenant/automations/pricing-flow/publish", nil, 204)
	s.syncRules("account")
	s.apiCall("POST", "tenant/automations/pricing-flow/disable", nil, 204)
	s.syncRules("account")
	s.matches("instagram", "account", "post", "pricing", 0)
	s.apiCall("DELETE", "tenant/automations/pricing-flow", nil, 204)
	s.syncRules("account")
	s.send("instagram", "account", "new-person", comment("instagram", "account", "new-person", "disabled", "post", "pricing"), 1)
	if n := s.count("SELECT count(*) FROM automation_sessions"); n != 1 {
		t.Fatalf("disabled automation admitted a session: %d", n)
	}

	s.send("instagram", "account", "person", dm("instagram", "account", "person", "invalid", "not an email"), 2)
	s.wait("one invalid reply and retry delivery", func() bool {
		return s.count("SELECT invalid_attempts FROM session_steps WHERE session_id=$1 AND node_id='capture'", sid) == 1 && s.count("SELECT count(*) FROM execution_jobs WHERE session_id=$1 AND payload->>'purpose'='retry' AND status='done'", sid) == 1
	})
	self := dm("instagram", "account", "person", "own-message", "wrong@example.com")
	*self.Data.Own = true
	s.send("instagram", "account", "person", self, 1)
	s.send("instagram", "account", "person", dm("instagram", "account", "person", "valid", "buyer@example.com"), 3)
	s.wait("completed original version", func() bool { return s.session("tenant", sid).Status == "completed" })
	v := s.session("tenant", sid)
	if v.Revision != 1 || v.Context.Contact.Email != "buyer@example.com" {
		t.Fatalf("wrong pinned version/context: %+v", v)
	}
	for i := 1; i < len(v.History); i++ {
		if v.History[i].Version <= v.History[i-1].Version {
			t.Fatal("history is not strictly ordered")
		}
	}
	requests := s.requests(sid)
	if len(requests) != 5 {
		t.Fatalf("got %d effects, want public + prompt + retry + email + final DM: %+v", len(requests), requests)
	}
	foundEmail, foundFinal := false, false
	for _, r := range requests {
		foundEmail = foundEmail || r.Kind == "email"
		foundFinal = foundFinal || r.Kind == "dm" && r.Text == "Original flow complete"
		if r.Kind == "email" && (r.Subject != "Original pricing" || r.Email != "buyer@example.com" || !strings.Contains(r.Text, "buyer@example.com")) {
			t.Fatalf("template was not pinned/rendered: %+v", r)
		}
		if r.Kind == "dm" && r.Purpose == "action" && r.Text != "Your email?" && r.Text != "Original flow complete" {
			t.Fatalf("graph changed in flight: %+v", r)
		}
	}
	if !foundEmail || !foundFinal {
		t.Fatalf("missing email or final DM: %+v", requests)
	}
	if n := s.count("SELECT count(*) FROM session_inbox WHERE session_id=$1", sid); n != 2 {
		t.Fatalf("got %d inbox entries, want one invalid and one valid reply", n)
	}
	if n := s.count("SELECT count(*) FROM conversation_owners WHERE session_id=$1", sid); n != 0 {
		t.Fatalf("owner not released: %d", n)
	}
	s.apiCall("GET", "other-workspace/sessions/"+sid, nil, 404)
}

func TestFastReplySurvivesNegativeRouteCacheAndUnconfirmedPrompt(t *testing.T) {
	s := newSystem(t, 2)
	s.configure("tenant", "instagram", "account", "flow")
	s.send("instagram", "account", "person", dm("instagram", "account", "person", "before-session", "old@example.com"), 1)
	if n := s.count("SELECT count(*) FROM session_inbox"); n != 0 {
		t.Fatalf("unrelated DM entered inbox: %d", n)
	}
	s.mu.Lock()
	s.providerHook = func(ctx context.Context, request delivery.Request) error {
		if request.Kind != "dm" || request.Text != "Your email?" {
			return nil
		}
		message := dm("instagram", "account", "person", "fast-reply", "fast@example.com")
		message.Data.ReplyTo = "provider:" + request.ID
		raw, err := s.emit(ctx, "instagram", "account", "person", message)
		if err != nil {
			return err
		}
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			s.mu.Lock()
			consumed := s.handled[raw] > 0
			s.mu.Unlock()
			if consumed {
				return nil
			} // The reply is durably routed before the HTTP receipt exists.
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
			}
		}
	}
	s.mu.Unlock()
	s.send("instagram", "account", "person", comment("instagram", "account", "person", "comment", "post", "pricing"), 1)
	sid := s.sessionID("flow", "person")
	s.wait("fast reply captured", func() bool { return s.session("tenant", sid).Status == "completed" })
	if got := s.session("tenant", sid).Context.Contact.Email; got != "fast@example.com" {
		t.Fatalf("captured %q", got)
	}
	if n := len(s.requests(sid)); n != 4 {
		t.Fatalf("got %d effects, want 4", n)
	}
}

func TestAccountsAndWorkspacesDoNotShareConversations(t *testing.T) {
	s := newSystem(t, 2)
	s.configure("tenant-a", "instagram", "account-a", "flow-a")
	s.configure("tenant-b", "facebook", "account-b", "flow-b")
	for _, input := range []struct{ tenant, platform, account, flow string }{
		{"tenant-a", "instagram", "account-a", "flow-a"}, {"tenant-b", "facebook", "account-b", "flow-b"},
	} {
		s.send(input.platform, input.account, "same-person", comment(input.platform, input.account, "same-person", "same-comment", "post", "pricing"), 1)
		sid := s.sessionID(input.flow, "same-person")
		s.wait("account waiting", func() bool { v := s.session(input.tenant, sid); return v.Status == "waiting" && v.Node == "capture" })
	}
	a, b := s.sessionID("flow-a", "same-person"), s.sessionID("flow-b", "same-person")
	s.apiCall("GET", "tenant-a/sessions/"+b, nil, 404)
	s.apiCall("POST", "tenant-a/sessions/"+b+"/cancel", nil, 404)
	s.send("instagram", "account-a", "same-person", dm("instagram", "account-a", "same-person", "same-message", "a@example.com"), 2)
	s.wait("only account A completed", func() bool { return s.session("tenant-a", a).Status == "completed" })
	if v := s.session("tenant-b", b); v.Status != "waiting" || v.Context.Contact.Email != "" {
		t.Fatalf("cross-account reply: %+v", v)
	}
	s.apiCall("POST", "tenant-b/sessions/"+b+"/cancel", nil, 204)
	s.send("facebook", "account-b", "same-person", dm("facebook", "account-b", "same-person", "same-message", "b@example.com"), 1)
	if v := s.session("tenant-b", b); v.Status != "cancelled" {
		t.Fatalf("late DM resumed cancelled session: %+v", v)
	}
	if n := len(s.requests(b)); n != 2 {
		t.Fatalf("cancelled session sent %d effects, want 2", n)
	}
}
