package integration_test

import (
	"encoding/json"
	"testing"

	"blotato-comment-automation/internal/feature/automation"
	"blotato-comment-automation/internal/feature/workflow"
)

// The core pricing flow must deliver a link by DM after capturing an email;
// email delivery is an optional action, not a requirement for this scenario.
func TestPricingCommentCapturesEmailAndSendsLinkByDM(t *testing.T) {
	s := newSystem(t, 2)
	s.apiCall("PUT", "tenant/accounts/account", map[string]any{"platform": "instagram"}, 204)
	doc := document("account")
	doc.Trigger.Scopes[0].Posts = automation.Posts{Mode: "all"}
	var graph workflow.Definition
	if err := json.Unmarshal(doc.Workflow, &graph); err != nil {
		t.Fatal(err)
	}
	const link = "Here is your pricing: https://example.com/pricing"
	graph.Steps = append(graph.Steps[:2], workflow.Step{ID: "link", Kind: "dm", Text: link})
	doc.Workflow, _ = json.Marshal(graph)
	// No email template is created: every external action here is a reply or DM.
	s.apiCall("PUT", "tenant/automations/pricing", doc, 204)
	s.apiCall("POST", "tenant/automations/pricing/publish", nil, 204)
	s.syncRules("account")

	for _, post := range []string{"first-post", "another-post"} {
		t.Run(post, func(t *testing.T) {
			person := "person-" + post
			s.send("instagram", "account", person, comment("instagram", "account", person, "comment-"+post, post, "pricing"), 2)
			sid := s.sessionID("pricing", person)
			s.wait("waiting for email", func() bool {
				return s.session("tenant", sid).Node == "capture" && s.session("tenant", sid).Status == "waiting"
			})
			initial := s.requests(sid)
			if len(initial) != 2 {
				t.Fatalf("want only public reply and prompt before email; got %+v", initial)
			}
			s.send("instagram", "account", person, dm("instagram", "account", person, "invalid-"+post, "please send the link"), 2)
			s.wait("one re-ask", func() bool {
				return s.count("SELECT count(*) FROM execution_jobs WHERE session_id=$1 AND payload->>'purpose'='retry' AND status='done'", sid) == 1
			})
			if s.session("tenant", sid).Status != "waiting" || len(s.requests(sid)) != 3 {
				t.Fatal("invalid email must re-ask without sending the link")
			}
			s.send("instagram", "account", person, dm("instagram", "account", person, "valid-"+post, "My email is buyer@example.com"), 3)
			s.wait("confirmed link delivery", func() bool { return s.session("tenant", sid).Status == "completed" })
			if s.session("tenant", sid).Context.Contact.Email != "buyer@example.com" {
				t.Fatal("email was not captured")
			}
			requests := s.requests(sid)
			if len(requests) != 4 {
				t.Fatalf("want reply, prompt, one re-ask and link; got %+v", requests)
			}
			public, prompt, final := 0, 0, 0
			for _, r := range requests {
				if r.AccountID != "account" || r.Platform != "instagram" || r.ParticipantID != person {
					t.Fatalf("delivery escaped the originating conversation: %+v", r)
				}
				if r.Kind == "email" {
					t.Fatal("core flow must send the link by DM, not email")
				}
				if r.Kind == "reply" && r.Text == "Sent you a DM!" {
					public++
				}
				if r.Kind == "dm" && r.Text == "Your email?" {
					prompt++
				}
				if r.Kind == "dm" && r.Text == link {
					final++
				}
			}
			if public != 1 || prompt != 1 || final != 1 {
				t.Fatalf("unexpected effects: %+v", requests)
			}
		})
	}
}
