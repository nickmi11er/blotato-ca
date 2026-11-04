package workflow

import (
	"encoding/json"
	"testing"
	"time"
)

func compileDefinition(t *testing.T, d Definition) Plan {
	t.Helper()
	raw, _ := json.Marshal(d)
	p, err := Compile(raw)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func TestCompileLinksQuestionAndTerminalTimeoutBranch(t *testing.T) {
	d := Definition{Steps: []Step{
		{ID: "welcome", Kind: "parallel", Actions: []Step{{ID: "reply", Kind: "reply", Text: "Sent!"}, {ID: "ask", Kind: "dm", Text: "Email?"}}},
		{ID: "email", Kind: "wait_reply", Expected: "email", TimeoutHours: 1, InvalidBehavior: "retry", RetryMessage: "Again", MaxAttempts: 3, TimeoutBehavior: "branch", TimeoutSteps: []Step{{ID: "expired", Kind: "dm", Text: "Expired"}}},
		{ID: "send", Kind: "email", TemplateID: "pricing"},
	}, Templates: map[string]Template{"pricing": {Revision: 1, Subject: "Pricing", Body: "Hello {{email}}"}}}
	p := compileDefinition(t, d)
	if !p.HasReply || p.Nodes["ask"].ArmWait != "email" || p.Nodes["email"].Step.PromptNodeID != "ask" || p.Nodes["expired"].Next != "" || p.Nodes["email"].Next != "send" {
		t.Fatalf("incorrect graph: %+v", p)
	}
	d.Steps[1].TimeoutSteps = []Step{{ID: "bad-email", Kind: "email", TemplateID: "pricing"}}
	raw, _ := json.Marshal(d)
	if _, err := Compile(raw); err == nil {
		t.Fatal("timeout path falsely inherited successful capture")
	}
}
func TestRejectInvalidWorkflow(t *testing.T) {
	for _, raw := range []string{`{}`, `{"steps":[]}`, `{"steps":[{"id":"a","kind":"email","templateId":"missing"}]}`, `{"steps":[{"id":"a","kind":"reply","text":"ok"},{"id":"a","kind":"dm","text":"ok"}]}`, `{"steps":[{"id":"a","kind":"parallel","actions":[{"id":"b","kind":"delay","minutes":1},{"id":"c","kind":"dm","text":"ok"}]}]}`, `{"steps":[{"id":"a","kind":"schedule","mode":"next_time","time":"25:00","timezone":"UTC"}]}`} {
		if _, err := Compile([]byte(raw)); err == nil {
			t.Fatal("accepted", raw)
		}
	}
}
func TestEmailExtraction(t *testing.T) {
	for _, tc := range []struct{ text, want string }{{"Send it to me@example.com!", "me@example.com"}, {"<me@example.com>", "me@example.com"}, {"me@example.com me@example.com", "me@example.com"}, {"a@example.com b@example.com", ""}, {"not an email", ""}, {"a..b@example.com", ""}, {"a@localhost", ""}} {
		if got := ExtractEmail(tc.text); got != tc.want {
			t.Fatalf("%q: got %q want %q", tc.text, got, tc.want)
		}
	}
}
func TestScheduleDST(t *testing.T) {
	now, _ := time.Parse(time.RFC3339, "2026-03-08T06:00:00Z")
	due, err := Due(Step{Kind: "schedule", Mode: "next_time", Time: "02:30", Timezone: "America/New_York"}, now)
	if err != nil || due.Format(time.RFC3339) != "2026-03-09T06:30:00Z" {
		t.Fatal(due, err)
	}
	now, _ = time.Parse(time.RFC3339, "2026-11-01T05:45:00Z")
	due, err = Due(Step{Kind: "schedule", Mode: "next_time", Time: "01:30", Timezone: "America/New_York"}, now)
	if err != nil || due.Format(time.RFC3339) != "2026-11-01T06:30:00Z" {
		t.Fatal(due, err)
	}
}
