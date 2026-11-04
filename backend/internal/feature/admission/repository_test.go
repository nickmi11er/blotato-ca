package admission_test

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"blotato-comment-automation/internal/feature/admission"
	"blotato-comment-automation/internal/feature/automation"
	"blotato-comment-automation/internal/feature/inbound"
	"blotato-comment-automation/internal/testutil"

	"github.com/jackc/pgx/v5/pgxpool"
)

func setup(t *testing.T, mode string) (*pgxpool.Pool, *admission.Repository, automation.Match, inbound.Comment) {
	t.Helper()
	pool := testutil.Database(t)
	service := automation.NewService(pool)
	ctx := t.Context()
	if err := service.RegisterAccount(ctx, "tenant", "account", "instagram"); err != nil {
		t.Fatal(err)
	}
	d := automation.Document{Name: "Pricing", Trigger: automation.Trigger{SchemaVersion: 1, Scopes: []automation.Scope{{AccountID: "account", Posts: automation.Posts{Mode: "all"}}}, Comment: automation.Predicate{Mode: "any", Operator: "exact", Logic: "any"}, Repeat: automation.Repeat{Mode: mode}}, Workflow: json.RawMessage(`{"steps":[{"id":"reply","kind":"reply","text":"Thanks!"}]}`)}
	if err := service.Save(ctx, "tenant", "automation", d); err != nil {
		t.Fatal(err)
	}
	if err := service.Publish(ctx, "tenant", "automation"); err != nil {
		t.Fatal(err)
	}
	c := inbound.Comment{Type: "CommentReceived", Platform: "instagram", AccountID: "account"}
	c.Data.PostID = "post"
	c.Data.CommentID = "comment"
	c.Data.AuthorID = "participant"
	return pool, admission.NewRepository(pool), automation.Match{TenantID: "tenant", AutomationID: "automation", Revision: 1, Repeat: mode}, c
}
func count(t *testing.T, pool *pgxpool.Pool, table string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(t.Context(), "SELECT count(*) FROM "+table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
func TestConcurrentDuplicateCreatesOneSessionAndOutbox(t *testing.T) {
	pool, repo, m, c := setup(t, "every")
	var inserted atomic.Int32
	var wg sync.WaitGroup
	for range 12 {
		wg.Go(func() {
			ok, err := repo.Record(t.Context(), c, []automation.Match{m})
			if err != nil {
				t.Error(err)
			}
			if ok {
				inserted.Add(1)
			}
		})
	}
	wg.Wait()
	if inserted.Load() != 1 {
		t.Fatal(inserted.Load())
	}
	for _, table := range []string{"processed_comments", "comment_decisions", "automation_sessions", "execution_jobs"} {
		if count(t, pool, table) != 1 {
			t.Fatal(table)
		}
	}
}
func TestRepeatPolicyAcrossConcurrentComments(t *testing.T) {
	for _, mode := range []string{"every", "once", "skip_active"} {
		t.Run(mode, func(t *testing.T) {
			pool, repo, m, c := setup(t, mode)
			var wg sync.WaitGroup
			for i := range 10 {
				wg.Go(func() {
					event := c
					event.Data.CommentID = fmt.Sprint(i)
					if _, err := repo.Record(t.Context(), event, []automation.Match{m}); err != nil {
						t.Error(err)
					}
				})
			}
			wg.Wait()
			want := 1
			if mode == "every" {
				want = 10
			}
			if n := count(t, pool, "automation_sessions"); n != want {
				t.Fatalf("got %d sessions want %d", n, want)
			}
			if count(t, pool, "comment_decisions") != 10 {
				t.Fatal("denials not persisted")
			}
			if _, err := pool.Exec(t.Context(), "UPDATE automation_sessions SET status='completed'"); err != nil {
				t.Fatal(err)
			}
			for i := range 10 {
				event := c
				event.Data.CommentID = fmt.Sprint(i)
				if _, err := repo.Record(t.Context(), event, []automation.Match{m}); err != nil {
					t.Fatal(err)
				}
			}
			if count(t, pool, "automation_sessions") != want {
				t.Fatal("redelivery changed a committed decision")
			}
			c.Data.CommentID = "new"
			if _, err := repo.Record(t.Context(), c, []automation.Match{m}); err != nil {
				t.Fatal(err)
			}
			if mode != "once" {
				want++
			}
			if count(t, pool, "automation_sessions") != want {
				t.Fatal("new comment did not respect completed session")
			}
		})
	}
}
func TestAdmissionRollbackKeepsEventRetryable(t *testing.T) {
	pool, repo, m, c := setup(t, "every")
	invalid := m
	invalid.AutomationID = "missing"
	if _, err := repo.Record(t.Context(), c, []automation.Match{m, invalid}); err == nil {
		t.Fatal("expected foreign key failure")
	}
	for _, table := range []string{"processed_comments", "comment_decisions", "automation_sessions", "execution_jobs"} {
		if count(t, pool, table) != 0 {
			t.Fatal("partial transaction: " + table)
		}
	}
	if _, err := repo.Record(t.Context(), c, []automation.Match{m}); err != nil {
		t.Fatal(err)
	}
	if count(t, pool, "automation_sessions") != 1 {
		t.Fatal("retry lost")
	}
}
