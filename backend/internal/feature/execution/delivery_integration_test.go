package execution_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	db "blotato-comment-automation/internal/db/sqlcDB/execution"
	"blotato-comment-automation/internal/feature/delivery"
	"blotato-comment-automation/internal/feature/execution"
	"blotato-comment-automation/internal/feature/workflow"
)

func TestUncertainHTTPDeliveryAtRetryLimitRemainsResolvable(t *testing.T) {
	for name, laterStatus := range map[string]int{
		"lost acknowledgements": http.StatusServiceUnavailable,
		"later rate limit":      http.StatusTooManyRequests,
		"later rejection":       http.StatusBadRequest,
	} {
		t.Run(name, func(t *testing.T) {
			f := setup(t, workflow.Definition{Steps: []workflow.Step{
				{ID: "send", Kind: "dm", Text: "Your link"},
				{ID: "after", Kind: "reply", Text: "Done"},
			}})
			// The provider sends once, deduplicates subsequent requests, but loses every
			// acknowledgement. Idempotency allows retries; it does not prove failure.
			var mu sync.Mutex
			requests := []delivery.Request{}
			sent := map[string]bool{}
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request delivery.Request
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					w.WriteHeader(400)
					return
				}
				if r.Header.Get("Idempotency-Key") != request.ID {
					t.Error("missing stable idempotency header")
				}
				mu.Lock()
				requests = append(requests, request)
				attempt := len(requests)
				sent[request.ID] = true
				mu.Unlock()
				if attempt > 1 && laterStatus != http.StatusServiceUnavailable {
					http.Error(w, "later attempt rejected", laterStatus)
					return
				}
				http.Error(w, "acknowledgement unavailable", http.StatusServiceUnavailable)
			}))
			defer provider.Close()
			adapter := delivery.NewHTTP(delivery.Config{Mode: "http", URL: provider.URL, Idempotency: true})
			f.worker = execution.NewWorker(f.store, f.engine, adapter, publisher{f.router}, execution.DefaultConfig())
			if _, err := f.worker.Once(t.Context()); err != nil {
				t.Fatal(err)
			} // start transition
			for attempt := 0; attempt < 3; attempt++ {
				if _, err := f.worker.Once(t.Context()); err != nil {
					t.Fatal(err)
				}
				// Advance only the retry clock; the production job claim/HTTP/commit path is unchanged.
				if _, err := f.pool.Exec(t.Context(), "UPDATE execution_jobs SET run_at=clock_timestamp() WHERE kind='deliver' AND status='ready'"); err != nil {
					t.Fatal(err)
				}
			}
			mu.Lock()
			copies := append([]delivery.Request(nil), requests...)
			deliveries := len(sent)
			mu.Unlock()
			expectedAttempts := 3
			if laterStatus == http.StatusBadRequest {
				expectedAttempts = 2
			}
			if len(copies) != expectedAttempts || deliveries != 1 {
				t.Fatalf("attempts=%d external sends=%d", len(copies), deliveries)
			}
			for _, r := range copies {
				if r != copies[0] {
					t.Fatal("retry changed immutable request")
				}
			}
			state := f.session(t)
			if state.Status != "waiting" || state.FinishReason != "delivery_unknown" {
				t.Fatalf("an unconfirmed send became terminal: status=%s reason=%s", state.Status, state.FinishReason)
			}
			jobs, err := db.New(f.pool).ListJobs(t.Context(), f.sid)
			if err != nil {
				t.Fatal(err)
			}
			var unknown string
			for _, j := range jobs {
				if j.Kind == "deliver" && j.NodeID == "send" && j.Status == "unknown" {
					unknown = j.ID
				}
			}
			if unknown == "" {
				t.Fatal("missing unknown delivery for reconciliation")
			}
			if err = f.engine.Resolve(t.Context(), "tenant", f.sid, unknown, delivery.Result{Status: "succeeded", ProviderMessageID: "provider-1"}); err != nil {
				t.Fatal(err)
			}
			// Reconciliation resumes the next action; it never resends the resolved one.
			f.worker = execution.NewWorker(f.store, f.engine, f.sender, publisher{f.router}, execution.DefaultConfig())
			f.drain(t)
			if f.session(t).Status != "completed" || count(t, f.pool, "SELECT count(*) FROM recorded_deliveries WHERE payload->>'kind'='reply'") != 1 {
				t.Fatal("reconciliation did not resume workflow")
			}

		})
	}
}

func TestMalformedHTTPReceiptDoesNotAdvanceSession(t *testing.T) {
	receipt := `{"status":"succeeded","provider_message_id":"provider-1"}`
	cases := map[string]string{
		"trailing JSON":     receipt + `{"status":"failed"}`,
		"trailing garbage":  receipt + "broken response",
		"oversized receipt": receipt + strings.Repeat(" ", 65536),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			f := setup(t, workflow.Definition{Steps: []workflow.Step{{ID: "send", Kind: "dm", Text: "hello"}}})
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, body) }))
			defer provider.Close()
			adapter := delivery.NewHTTP(delivery.Config{Mode: "http", URL: provider.URL, Idempotency: false})
			f.worker = execution.NewWorker(f.store, f.engine, adapter, publisher{f.router}, execution.DefaultConfig())
			f.drain(t)
			if state := f.session(t); state.Status != "waiting" || state.FinishReason != "delivery_unknown" {
				t.Fatalf("malformed receipt advanced session: %s %s", state.Status, state.FinishReason)
			}
		})
	}
}

func TestDefiniteHTTPRejectionExhaustsRetriesWithoutResendingParallelSibling(t *testing.T) {
	f := setup(t, workflow.Definition{Steps: []workflow.Step{{ID: "both", Kind: "parallel", Actions: []workflow.Step{{ID: "public", Kind: "reply", Text: "Hello"}, {ID: "dm", Kind: "dm", Text: "Link"}}}}})
	var mu sync.Mutex
	attempts := map[string]int{}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request delivery.Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		mu.Lock()
		attempts[request.Kind]++
		mu.Unlock()
		if request.Kind == "dm" {
			_, _ = io.WriteString(w, `{"status":"retry","reason":"not sent: temporary capacity limit"}`)
			return
		}
		_, _ = io.WriteString(w, `{"status":"succeeded","provider_message_id":"public-1"}`)
	}))
	defer provider.Close()
	f.worker = execution.NewWorker(f.store, f.engine, delivery.NewHTTP(delivery.Config{Mode: "http", URL: provider.URL}), publisher{f.router}, execution.DefaultConfig())
	for range 3 {
		f.drain(t)
		if _, err := f.pool.Exec(t.Context(), "UPDATE execution_jobs SET run_at=clock_timestamp() WHERE status='ready' AND kind='deliver'"); err != nil {
			t.Fatal(err)
		}
	}
	mu.Lock()
	public, dm := attempts["reply"], attempts["dm"]
	mu.Unlock()
	if public != 1 || dm != 3 {
		t.Fatalf("public=%d dm=%d", public, dm)
	}
	if state := f.session(t); state.Status != "failed" || state.FinishReason != "delivery_failed" {
		t.Fatal(state.Status, state.FinishReason)
	}
}

func TestCancellationFencesInFlightHTTPReceipt(t *testing.T) {
	f := setup(t, workflow.Definition{Steps: []workflow.Step{
		{ID: "send", Kind: "dm", Text: "In flight"},
		{ID: "after", Kind: "reply", Text: "Must not run"},
	}})
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	var mu sync.Mutex
	requests := []delivery.Request{}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request delivery.Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		mu.Lock()
		requests = append(requests, request)
		n := len(requests)
		mu.Unlock()
		if n == 1 {
			close(entered)
		}
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		_, _ = io.WriteString(w, `{"status":"succeeded","provider_message_id":"already-sent"}`)
	}))
	defer provider.Close()
	defer unblock()
	f.worker = execution.NewWorker(f.store, f.engine, delivery.NewHTTP(delivery.Config{Mode: "http", URL: provider.URL, Idempotency: true}), publisher{f.router}, execution.DefaultConfig())
	if _, err := f.worker.Once(t.Context()); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := f.worker.Once(t.Context()); done <- err }()
	select {
	case <-entered:
	case err := <-done:
		t.Fatalf("worker exited before send: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("HTTP send did not start")
	}
	if err := f.engine.Cancel(t.Context(), "tenant", f.sid); err != nil {
		t.Fatal(err)
	}
	unblock()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("worker did not finish")
	}
	f.drain(t)
	if got := f.session(t).Status; got != "cancelled" {
		t.Fatalf("late receipt changed state to %s", got)
	}
	mu.Lock()
	copies := append([]delivery.Request(nil), requests...)
	mu.Unlock()
	if len(copies) != 1 || copies[0].Text != "In flight" {
		t.Fatalf("cancellation allowed subsequent delivery: %+v", copies)
	}
}

func TestRecoveryPreservesUncertaintyAfterLostDeliveryCommit(t *testing.T) {
	f := setup(t, workflow.Definition{Steps: []workflow.Step{{ID: "send", Kind: "dm", Text: "Link"}}})
	var mu sync.Mutex
	attempts := 0
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts++
		n := attempts
		mu.Unlock()
		if n == 1 {
			_, _ = io.WriteString(w, `{"status":"succeeded","provider_message_id":"sent-before-crash"}`)
			return
		}
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer provider.Close()
	adapter := delivery.NewHTTP(delivery.Config{Mode: "http", URL: provider.URL, Idempotency: true})
	f.worker = execution.NewWorker(f.store, f.engine, adapter, publisher{f.router}, execution.DefaultConfig())
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
	var request delivery.Request
	if err = json.Unmarshal(job.Payload, &request); err != nil {
		t.Fatal(err)
	}
	if result := adapter.Send(t.Context(), request); result.Status != "succeeded" {
		t.Fatal(result)
	}
	// Simulate a process dying after the HTTP send and before Complete commits.
	if _, err = f.pool.Exec(t.Context(), "UPDATE execution_jobs SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1", job.ID); err != nil {
		t.Fatal(err)
	}
	if err = f.worker.Recover(t.Context()); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err = f.worker.Once(t.Context()); err != nil {
			t.Fatal(err)
		}
		if _, err = f.pool.Exec(t.Context(), "UPDATE execution_jobs SET run_at=clock_timestamp() WHERE status='ready'"); err != nil {
			t.Fatal(err)
		}
	}
	if state := f.session(t); state.Status != "waiting" || state.FinishReason != "delivery_unknown" {
		t.Fatalf("recovered send became terminal: %s %s", state.Status, state.FinishReason)
	}
	// Explicit reconciliation may also confirm failure; it must override uncertainty.
	if err = f.engine.Resolve(t.Context(), "tenant", f.sid, job.ID, delivery.Result{Status: "failed", Reason: "operator reconciled"}); err != nil {
		t.Fatal(err)
	}
	if state := f.session(t); state.Status != "failed" {
		t.Fatal("failed resolution did not terminate", state.Status)
	}
}
