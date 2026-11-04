package admission

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"sort"

	db "blotato-comment-automation/internal/db/sqlcDB/admission"
	inboundDB "blotato-comment-automation/internal/db/sqlcDB/inbound"
	"blotato-comment-automation/internal/feature/automation"
	"blotato-comment-automation/internal/feature/inbound"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

type Outcome struct {
	automation.Match
	SessionID string `json:"session_id,omitempty"`
	Denied    bool   `json:"denied"`
}

func (r *Repository) Record(ctx context.Context, c inbound.Comment, matches []automation.Match) (bool, error) {
	if len(matches) == 0 {
		return false, nil
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(context.Background())
	rows, err := inboundDB.New(tx).RecordComment(ctx, inboundDB.RecordCommentParams{Platform: c.Platform, AccountID: c.AccountID, PostID: c.Data.PostID, CommentID: c.Data.CommentID, EventType: c.Type})
	if err != nil {
		return false, err
	}
	if rows == 0 {
		return false, tx.Commit(ctx)
	}
	q := db.New(tx)
	// Stable lock order avoids deadlocks when multiple automations match.
	matches = append([]automation.Match(nil), matches...)
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].TenantID != matches[j].TenantID {
			return matches[i].TenantID < matches[j].TenantID
		}
		return matches[i].AutomationID < matches[j].AutomationID
	})
	outcomes := make([]Outcome, 0, len(matches))
	for _, m := range matches {
		if err = q.ReserveParticipant(ctx, db.ReserveParticipantParams{TenantID: m.TenantID, AutomationID: m.AutomationID, AccountID: c.AccountID, ParticipantID: c.Data.AuthorID}); err != nil {
			return false, err
		}
		if _, err = q.LockParticipant(ctx, db.LockParticipantParams{TenantID: m.TenantID, AutomationID: m.AutomationID, AccountID: c.AccountID, ParticipantID: c.Data.AuthorID}); err != nil {
			return false, err
		}
		prior, err := q.PriorSessions(ctx, db.PriorSessionsParams{TenantID: m.TenantID, AutomationID: m.AutomationID, AccountID: c.AccountID, ParticipantID: c.Data.AuthorID})
		if err != nil {
			return false, err
		}
		outcome := Outcome{Match: m, Denied: m.Repeat == "once" && prior.Started || m.Repeat == "skip_active" && prior.Active}
		if !outcome.Denied {
			var id [16]byte
			if _, err = rand.Read(id[:]); err != nil {
				return false, err
			}
			outcome.SessionID = hex.EncodeToString(id[:])
			if err = q.CreateSession(ctx, db.CreateSessionParams{ID: outcome.SessionID, TenantID: m.TenantID, AutomationID: m.AutomationID, Revision: m.Revision, AccountID: c.AccountID, ParticipantID: c.Data.AuthorID}); err != nil {
				return false, err
			}
			payload, err := json.Marshal(struct {
				Type    string          `json:"type"`
				Outcome Outcome         `json:"admission"`
				Comment inbound.Comment `json:"comment"`
			}{"SessionRequested", outcome, c})
			if err != nil {
				return false, err
			}
			if err = q.EnqueueSession(ctx, db.EnqueueSessionParams{SessionID: outcome.SessionID, Payload: payload}); err != nil {
				return false, err
			}
		}
		outcomes = append(outcomes, outcome)
	}
	b, err := json.Marshal(outcomes)
	if err != nil {
		return false, err
	}
	if err = q.RecordDecision(ctx, db.RecordDecisionParams{Platform: c.Platform, AccountID: c.AccountID, PostID: c.Data.PostID, CommentID: c.Data.CommentID, EventType: c.Type, Outcomes: b}); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}
