package execution

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	db "blotato-comment-automation/internal/db/sqlcDB/execution"
	"blotato-comment-automation/internal/feature/delivery"
	"github.com/jackc/pgx/v5"
)

func (e *Engine) Cancel(ctx context.Context, tenant, sid string) error {
	return e.store.transaction(ctx, sid, func(f *frame) error {
		if f.session.TenantID != tenant {
			return ErrNotFound
		}
		if terminal(f.session.Status) {
			return nil
		}
		from := f.session.Status
		if err := f.finish(ctx, "cancelled", "user_cancelled", ""); err != nil {
			return err
		}
		return f.save(ctx, "cancel", from, "user_cancelled")
	})
}
func (e *Engine) Resolve(ctx context.Context, tenant, sid, jobID string, result delivery.Result) error {
	if result.Status != "succeeded" && result.Status != "failed" {
		return ErrConflict
	}
	if result.Status == "succeeded" && result.ProviderMessageID == "" {
		return ErrConflict
	}
	return e.store.transaction(ctx, sid, func(f *frame) error {
		if f.session.TenantID != tenant {
			return ErrNotFound
		}
		if terminal(f.session.Status) {
			return ErrConflict
		}
		job, err := f.q.ClaimUnknownJob(ctx, db.ClaimUnknownJobParams{ID: jobID, SessionID: sid})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrConflict
		}
		if err != nil {
			return err
		}
		return e.complete(ctx, f, job, result)
	})
}

// Registered inside the existing internal API's authentication middleware.
func RegisterHTTP(engine *Engine) func(*http.ServeMux) {
	return func(mux *http.ServeMux) {
		respond := func(w http.ResponseWriter, err error) {
			status := http.StatusInternalServerError
			message := "internal server error"
			if errors.Is(err, ErrNotFound) || errors.Is(err, pgx.ErrNoRows) {
				status = http.StatusNotFound
				message = "session not found"
			}
			if errors.Is(err, ErrConflict) || errors.Is(err, ErrLease) {
				status = http.StatusConflict
				message = err.Error()
			}
			http.Error(w, message, status)
		}
		base := "/v1/workspaces/{tenant}/sessions/{id}"
		mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
			tx, err := engine.store.pool.BeginTx(r.Context(), pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
			if err != nil {
				respond(w, err)
				return
			}
			defer tx.Rollback(context.Background())
			q := db.New(tx)
			session, err := q.GetSession(r.Context(), r.PathValue("id"))
			if err != nil {
				respond(w, err)
				return
			}
			if session.TenantID != r.PathValue("tenant") {
				respond(w, ErrNotFound)
				return
			}
			steps, err := q.ListSteps(r.Context(), session.ID)
			if err != nil {
				respond(w, err)
				return
			}
			jobs, err := q.ListJobs(r.Context(), session.ID)
			if err != nil {
				respond(w, err)
				return
			}
			history, err := q.ListTransitions(r.Context(), session.ID)
			if err != nil {
				respond(w, err)
				return
			}
			if err = tx.Commit(r.Context()); err != nil {
				respond(w, err)
				return
			}
			// Raw JSON columns are objects in the API, never pgx byte slices encoded as base64.
			type stepView struct {
				db.SessionStep
				Output json.RawMessage `json:"output"`
			}
			type jobView struct {
				db.ExecutionJob
				Payload json.RawMessage `json:"payload"`
				Result  json.RawMessage `json:"result"`
			}
			sv := make([]stepView, 0, len(steps))
			jv := make([]jobView, 0, len(jobs))
			for _, step := range steps {
				sv = append(sv, stepView{step, step.Output})
			}
			for _, job := range jobs {
				jv = append(jv, jobView{job, job.Payload, job.Result})
			}
			value := struct {
				db.AutomationSession
				Context json.RawMessage        `json:"context"`
				Origin  json.RawMessage        `json:"origin"`
				Steps   []stepView             `json:"steps"`
				Jobs    []jobView              `json:"jobs"`
				History []db.SessionTransition `json:"history"`
			}{session, session.Context, session.Origin, sv, jv, history}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(value)
		})
		mux.HandleFunc("POST "+base+"/cancel", func(w http.ResponseWriter, r *http.Request) {
			if err := engine.Cancel(r.Context(), r.PathValue("tenant"), r.PathValue("id")); err != nil {
				respond(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
		mux.HandleFunc("POST "+base+"/resolve-delivery", func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				JobID             string `json:"job_id"`
				Status            string `json:"status"`
				ProviderMessageID string `json:"provider_message_id"`
				Reason            string `json:"reason"`
			}
			dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384))
			dec.DisallowUnknownFields()
			if dec.Decode(&body) != nil || dec.Decode(&struct{}{}) != io.EOF {
				http.Error(w, "invalid resolution", http.StatusBadRequest)
				return
			}
			err := engine.Resolve(r.Context(), r.PathValue("tenant"), r.PathValue("id"), body.JobID, delivery.Result{Status: body.Status, ProviderMessageID: body.ProviderMessageID, Reason: body.Reason})
			if err != nil {
				respond(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
	}
}
