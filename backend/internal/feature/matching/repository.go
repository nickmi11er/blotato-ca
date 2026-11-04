package matching

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	db "blotato-comment-automation/internal/db/sqlcDB/automation"
	"blotato-comment-automation/internal/feature/automation"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrGeneration = errors.New("rule generation changed")

type Rule struct {
	Match     automation.Match
	Predicate automation.Predicate
}
type Snapshot struct {
	TenantID, Platform string
	Generation         int64
	HasSelected        bool
	Rules              []Rule
}
type Source interface {
	Account(context.Context, string) (Snapshot, error)
	Post(context.Context, string, string, int64) (Snapshot, error)
	Generations(context.Context, []string) (map[string]int64, error)
}
type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }
func (r *Repository) Account(ctx context.Context, id string) (Snapshot, error) {
	return r.load(ctx, id, "", -1)
}
func (r *Repository) Post(ctx context.Context, id, post string, generation int64) (Snapshot, error) {
	return r.load(ctx, id, post, generation)
}
func (r *Repository) load(ctx context.Context, id, post string, generation int64) (Snapshot, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Snapshot{}, err
	}
	defer tx.Rollback(context.Background())
	q := db.New(tx)
	a, err := q.AccountSnapshot(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		if generation > 0 {
			return Snapshot{}, ErrGeneration
		}
		return Snapshot{}, nil
	}
	if err != nil {
		return Snapshot{}, err
	}
	if generation >= 0 && a.Generation != generation {
		return Snapshot{}, ErrGeneration
	}
	result := Snapshot{TenantID: a.TenantID, Platform: a.Platform, Generation: a.Generation, HasSelected: a.HasSelected && generation < 0}
	add := func(id string, revision int64, raw []byte) error {
		var d automation.Document
		if err := json.Unmarshal(raw, &d); err != nil {
			return err
		}
		if err := d.Validate(); err != nil {
			return fmt.Errorf("invalid published rule: %w", err)
		}
		result.Rules = append(result.Rules, Rule{Match: automation.Match{TenantID: a.TenantID, AutomationID: id, Revision: revision, Repeat: d.Trigger.Repeat.Mode}, Predicate: d.Trigger.Comment})
		return nil
	}
	if generation < 0 {
		rows, err := q.AccountRules(ctx, id)
		if err != nil {
			return Snapshot{}, err
		}
		for _, row := range rows {
			if err = add(row.AutomationID, row.Revision, row.Document); err != nil {
				return Snapshot{}, err
			}
		}
	} else {
		rows, err := q.PostRules(ctx, db.PostRulesParams{AccountID: id, PostID: post})
		if err != nil {
			return Snapshot{}, err
		}
		for _, row := range rows {
			if err = add(row.AutomationID, row.Revision, row.Document); err != nil {
				return Snapshot{}, err
			}
		}
	}
	return result, tx.Commit(ctx)
}
func (r *Repository) Generations(ctx context.Context, ids []string) (map[string]int64, error) {
	rows, err := db.New(r.pool).Generations(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[string]int64, len(rows))
	for _, row := range rows {
		result[row.AccountID] = row.Generation
	}
	return result, nil
}
