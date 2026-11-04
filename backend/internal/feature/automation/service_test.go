package automation_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"blotato-comment-automation/internal/feature/automation"
	"blotato-comment-automation/internal/feature/matching"
	"blotato-comment-automation/internal/testutil"
)

func document(account string) automation.Document {
	return automation.Document{Name: "Pricing", Trigger: automation.Trigger{SchemaVersion: 1, Scopes: []automation.Scope{{AccountID: account, Posts: automation.Posts{Mode: "all"}}}, Comment: automation.Predicate{Mode: "keywords", Operator: "whole_word", Logic: "any", Keywords: []string{"pricing"}}, Repeat: automation.Repeat{Mode: "every"}}, Workflow: json.RawMessage(`{"steps":[{"id":"reply","kind":"reply","text":"Thanks!"}]}`)}
}
func TestPublishDisableDeleteAndDraftIsolation(t *testing.T) {
	pool := testutil.Database(t)
	s := automation.NewService(pool)
	ctx := t.Context()
	r := matching.NewRepository(pool)
	for _, id := range []string{"a", "b"} {
		if err := s.RegisterAccount(ctx, "tenant", id, "instagram"); err != nil {
			t.Fatal(err)
		}
	}
	d := document("a")
	if err := s.Save(ctx, "tenant", "pricing", d); err != nil {
		t.Fatal(err)
	}
	before, err := r.Account(ctx, "a")
	if err != nil || len(before.Rules) != 0 {
		t.Fatal("draft became live", err)
	}
	if err = s.Publish(ctx, "tenant", "pricing"); err != nil {
		t.Fatal(err)
	}
	live, err := r.Account(ctx, "a")
	if err != nil || len(live.Rules) != 1 {
		t.Fatal("published rules missing", err)
	}
	d.Trigger.Scopes = []automation.Scope{{AccountID: "b", Posts: automation.Posts{Mode: "selected", IDs: []string{"post"}}}}
	if err = s.Save(ctx, "tenant", "pricing", d); err != nil {
		t.Fatal(err)
	}
	still, err := r.Account(ctx, "a")
	if err != nil || still.Generation != live.Generation {
		t.Fatal("draft invalidated publication", err)
	}
	if err = s.Publish(ctx, "tenant", "pricing"); err != nil {
		t.Fatal(err)
	}
	moved, err := r.Account(ctx, "a")
	if err != nil || len(moved.Rules) != 0 || moved.Generation <= live.Generation {
		t.Fatal("old account not invalidated", err)
	}
	b, err := r.Account(ctx, "b")
	if err != nil || !b.HasSelected || len(b.Rules) != 0 {
		t.Fatal("selected projection incorrect", err)
	}
	post, err := r.Post(ctx, "b", "post", b.Generation)
	if err != nil || len(post.Rules) != 1 || post.Rules[0].Match.Revision != 2 {
		t.Fatal("post revision incorrect", err)
	}
	if err = s.Disable(ctx, "tenant", "pricing"); err != nil {
		t.Fatal(err)
	}
	disabled, err := r.Account(ctx, "b")
	if err != nil || disabled.HasSelected || disabled.Generation <= b.Generation {
		t.Fatal("disable did not invalidate", err)
	}
	if _, err = r.Post(ctx, "b", "post", b.Generation); !errors.Is(err, matching.ErrGeneration) {
		t.Fatal("mixed generations accepted", err)
	}
	if err = s.Delete(ctx, "tenant", "pricing"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Get(ctx, "tenant", "pricing"); !errors.Is(err, automation.ErrNotFound) {
		t.Fatal(err)
	}
	var versions, changes int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM automation_versions").Scan(&versions); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM rule_changes").Scan(&changes); err != nil {
		t.Fatal(err)
	}
	if versions != 2 || changes != 6 {
		t.Fatalf("versions=%d changes=%d", versions, changes)
	}
}
func TestWorkspaceLimitSerializesConcurrentCreation(t *testing.T) {
	pool := testutil.Database(t)
	s := automation.NewService(pool)
	ctx := t.Context()
	if err := s.RegisterAccount(ctx, "tenant", "a", "instagram"); err != nil {
		t.Fatal(err)
	}
	d := document("a")
	for i := range automation.MaxAutomations - 1 {
		if err := s.Save(ctx, "tenant", fmt.Sprint(i), d); err != nil {
			t.Fatal(err)
		}
	}
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for i := range 12 {
		wg.Go(func() {
			err := s.Save(ctx, "tenant", fmt.Sprintf("new-%d", i), d)
			if err == nil {
				accepted.Add(1)
			} else if !errors.Is(err, automation.ErrLimit) {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if accepted.Load() != 1 {
		t.Fatalf("accepted %d writes", accepted.Load())
	}
	if err := s.Save(ctx, "tenant", "0", d); err != nil {
		t.Fatal("editing at limit failed", err)
	}
	if err := s.Delete(ctx, "tenant", "0"); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(ctx, "tenant", "replacement", d); err != nil {
		t.Fatal(err)
	}
}
func TestAccountOwnershipAndInternalHTTP(t *testing.T) {
	pool := testutil.Database(t)
	s := automation.NewService(pool)
	ctx := t.Context()
	if err := s.RegisterAccount(ctx, "tenant", "a", "instagram"); err != nil {
		t.Fatal(err)
	}
	if err := s.RegisterAccount(ctx, "other", "a", "instagram"); !errors.Is(err, automation.ErrOwnership) {
		t.Fatal("account moved tenant", err)
	}
	if err := s.Save(ctx, "other", "automation", document("a")); !errors.Is(err, automation.ErrOwnership) {
		t.Fatal("cross-tenant scope allowed", err)
	}
	handler := automation.NewHTTPHandler(s, "internal-test-secret")
	request := func(method, path, body, auth string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", auth)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	path := "/v1/workspaces/tenant/automations/pricing"
	b, _ := json.Marshal(document("a"))
	if w := request("PUT", path, string(b), ""); w.Code != http.StatusUnauthorized {
		t.Fatal(w.Code)
	}
	if w := request("PUT", path, string(b), "Bearer internal-test-secret"); w.Code != http.StatusNoContent {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request("POST", path+"/publish", "", "Bearer internal-test-secret"); w.Code != http.StatusNoContent {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request("GET", path, "", "Bearer internal-test-secret"); w.Code != http.StatusOK {
		t.Fatal(w.Code, w.Body.String())
	}
}
