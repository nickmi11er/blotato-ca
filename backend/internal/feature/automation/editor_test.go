package automation_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"blotato-comment-automation/internal/feature/automation"
	"blotato-comment-automation/internal/feature/workflow"
	"blotato-comment-automation/internal/testutil"
)

func TestEditorDraftVersionsAndPublication(t *testing.T) {
	pool := testutil.Database(t)
	s := automation.NewService(pool)
	ctx := t.Context()
	d := document("account")
	d.Trigger.Scopes = nil
	d.Trigger.Comment.Keywords = nil
	d.Workflow = json.RawMessage(`{"steps":[]}`)
	zero := int64(0)
	if err := s.SaveVersion(ctx, "tenant", "draft", d, &zero); err != nil {
		t.Fatal(err)
	}
	list, err := s.List(ctx, "tenant")
	if err != nil || len(list) != 1 || list[0].DraftVersion != 1 {
		t.Fatal(list, err)
	}
	if err = s.Publish(ctx, "tenant", "draft"); !errors.Is(err, automation.ErrInvalid) {
		t.Fatal("incomplete draft published", err)
	}
	d.Name = "Changed"
	one := int64(1)
	if err = s.SaveVersion(ctx, "tenant", "draft", d, &one); err != nil {
		t.Fatal(err)
	}
	if err = s.SaveVersion(ctx, "tenant", "draft", d, &one); !errors.Is(err, automation.ErrConflict) {
		t.Fatal("stale save accepted", err)
	}
	if err = s.MutateVersion(ctx, "tenant", "draft", "delete", &one); !errors.Is(err, automation.ErrConflict) {
		t.Fatal("stale deletion accepted", err)
	}
	other, err := s.List(ctx, "other")
	if err != nil || len(other) != 0 {
		t.Fatal("workspace leakage", other, err)
	}
	handler := automation.NewHTTPHandler(s, "secret")
	req := httptest.NewRequest(http.MethodPost, "/v1/workspaces/tenant/automations/draft/publish", nil)
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("If-Match", `"1"`)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatal(w.Code, w.Body.String())
	}
	raw, _ := json.Marshal(d)
	req = httptest.NewRequest(http.MethodPut, "/v1/workspaces/tenant/automations/draft", strings.NewReader(string(raw)))
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("If-Match", `"2"`)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatal(w.Code, w.Body.String())
	}
}
func TestEditorCatalogAndPinnedTemplates(t *testing.T) {
	pool := testutil.Database(t)
	s := automation.NewService(pool)
	ctx := t.Context()
	for _, account := range []string{"a", "b"} {
		if err := s.RegisterAccount(ctx, "tenant", account, "instagram"); err != nil {
			t.Fatal(err)
		}
		if err := s.SavePost(ctx, "tenant", automation.PostView{ID: "same-provider-id", AccountID: account, Title: "Post", PublishedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	catalog, err := s.Catalog(ctx, "tenant")
	if err != nil || len(catalog.Accounts) != 2 || len(catalog.Posts) != 2 {
		t.Fatal(catalog, err)
	}
	if err = s.SavePost(ctx, "other", automation.PostView{ID: "x", AccountID: "a", Title: "Post", PublishedAt: time.Now()}); !errors.Is(err, automation.ErrOwnership) {
		t.Fatal(err)
	}
	template := automation.EmailTemplate{ID: "pricing", Name: "Pricing", Subject: "Original", Body: "https://example.com"}
	if err = s.SaveTemplate(ctx, "tenant", template); err != nil {
		t.Fatal(err)
	}
	d := document("a")
	d.Workflow = json.RawMessage(`{"steps":[{"id":"ask","kind":"dm","text":"Email?"},{"id":"capture","kind":"wait_reply","expected":"email","timeoutHours":24,"invalidBehavior":"stop","timeoutBehavior":"stop"},{"id":"email","kind":"email","templateId":"pricing"}]}`)
	if err = s.Save(ctx, "tenant", "flow", d); err != nil {
		t.Fatal(err)
	}
	if err = s.Publish(ctx, "tenant", "flow"); err != nil {
		t.Fatal(err)
	}
	template.Revision = 1
	template.Subject = "Updated"
	if err = s.SaveTemplate(ctx, "tenant", template); err != nil {
		t.Fatal(err)
	}
	if err = s.SaveTemplate(ctx, "tenant", template); !errors.Is(err, automation.ErrConflict) {
		t.Fatal("stale template overwritten", err)
	}
	if err = s.DeleteTemplate(ctx, "tenant", "pricing", nil); !errors.Is(err, automation.ErrInvalid) {
		t.Fatal("referenced template deleted", err)
	}
	var raw []byte
	if err = pool.QueryRow(ctx, "SELECT execution_plan FROM automation_versions WHERE automation_id='flow' AND revision=1").Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var plan workflow.Plan
	if err = json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Templates["pricing"].Subject != "Original" || plan.Templates["pricing"].Revision != 1 {
		t.Fatal("published template mutated", plan)
	}
	if err = s.Publish(ctx, "tenant", "flow"); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, "SELECT execution_plan FROM automation_versions WHERE automation_id='flow' AND revision=2").Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Templates["pricing"].Subject != "Updated" || plan.Templates["pricing"].Revision != 2 {
		t.Fatal("republish omitted new template", plan)
	}
	if err = s.RegisterAccount(ctx, "tenant", "youtube", "youtube"); err != nil {
		t.Fatal(err)
	}
	d.Trigger.Scopes[0].AccountID = "youtube"
	if err = s.Save(ctx, "tenant", "bad", d); err != nil {
		t.Fatal(err)
	}
	if err = s.Publish(ctx, "tenant", "bad"); !errors.Is(err, automation.ErrInvalid) {
		t.Fatal("unsupported DM published", err)
	}
}
