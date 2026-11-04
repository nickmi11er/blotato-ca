package automation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	db "blotato-comment-automation/internal/db/sqlcDB/automation"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInvalid = errors.New("invalid request")

type Service struct{ pool *pgxpool.Pool }

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

type View struct {
	ID       string   `json:"id"`
	Enabled  bool     `json:"enabled"`
	Revision int64    `json:"revision"`
	Draft    Document `json:"draft"`
}

func (s *Service) Get(ctx context.Context, tenant, id string) (View, error) {
	row, err := db.New(s.pool).GetAutomation(ctx, db.GetAutomationParams{TenantID: tenant, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return View{}, ErrNotFound
	}
	if err != nil {
		return View{}, err
	}
	v := View{ID: row.ID, Enabled: row.Enabled, Revision: row.Revision}
	err = json.Unmarshal(row.Draft, &v.Draft)
	return v, err
}

// Serialize configuration writes per workspace: limits, revisions and binding changes
// remain atomic even when requests are served by different replicas.
func (s *Service) transaction(ctx context.Context, tenant string, f func(*db.Queries) error) error {
	if !ValidID(tenant) {
		return ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	q := db.New(tx)
	if err = q.EnsureWorkspace(ctx, tenant); err != nil {
		return err
	}
	if _, err = q.LockWorkspace(ctx, tenant); err != nil {
		return err
	}
	if err = f(q); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Service) RegisterAccount(ctx context.Context, tenant, id, platform string) error {
	if !ValidID(id) || !ValidID(platform) {
		return ErrInvalid
	}
	return s.transaction(ctx, tenant, func(q *db.Queries) error {
		added, err := q.RegisterAccount(ctx, db.RegisterAccountParams{ID: id, TenantID: tenant, Platform: platform})
		if err != nil {
			return err
		}
		if added == 0 {
			a, err := q.GetAccount(ctx, id)
			if err != nil {
				return err
			}
			if a.TenantID != tenant || a.Platform != platform {
				return ErrOwnership
			}
			return nil
		}
		return advance(ctx, q, []string{id})
	})
}
func checkAccounts(ctx context.Context, q *db.Queries, tenant string, d Document) error {
	for _, scope := range d.Trigger.Scopes {
		a, err := q.GetAccount(ctx, scope.AccountID)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: account is not registered", ErrInvalid)
		}
		if err != nil {
			return err
		}
		if a.TenantID != tenant {
			return ErrOwnership
		}
	}
	return nil
}
func (s *Service) Save(ctx context.Context, tenant, id string, d Document) error {
	if !ValidID(id) {
		return ErrInvalid
	}
	if err := d.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	return s.transaction(ctx, tenant, func(q *db.Queries) error {
		_, err := q.GetAutomation(ctx, db.GetAutomationParams{TenantID: tenant, ID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			count, e := q.CountAutomations(ctx, tenant)
			if e != nil {
				return e
			}
			if count >= MaxAutomations {
				return ErrLimit
			}
		} else if err != nil {
			return err
		}
		if err = checkAccounts(ctx, q, tenant, d); err != nil {
			return err
		}
		b, err := json.Marshal(d)
		if err != nil {
			return err
		}
		return q.SaveDraft(ctx, db.SaveDraftParams{TenantID: tenant, ID: id, Draft: b})
	})
}
func (s *Service) Publish(ctx context.Context, tenant, id string) error {
	return s.mutate(ctx, tenant, id, "publish")
}
func (s *Service) Disable(ctx context.Context, tenant, id string) error {
	return s.mutate(ctx, tenant, id, "disable")
}
func (s *Service) Delete(ctx context.Context, tenant, id string) error {
	return s.mutate(ctx, tenant, id, "delete")
}
func (s *Service) mutate(ctx context.Context, tenant, id, action string) error {
	if !ValidID(id) {
		return ErrInvalid
	}
	return s.transaction(ctx, tenant, func(q *db.Queries) error {
		a, err := q.GetAutomation(ctx, db.GetAutomationParams{TenantID: tenant, ID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		affected, err := q.BoundAccounts(ctx, db.BoundAccountsParams{TenantID: tenant, AutomationID: id})
		if err != nil {
			return err
		}
		if err = q.ClearBindings(ctx, db.ClearBindingsParams{TenantID: tenant, AutomationID: id}); err != nil {
			return err
		}
		switch action {
		case "publish":
			var d Document
			if err = json.Unmarshal(a.Draft, &d); err != nil {
				return err
			}
			if err = d.Validate(); err != nil {
				return err
			}
			if err = checkAccounts(ctx, q, tenant, d); err != nil {
				return err
			}
			revision, err := q.NextRevision(ctx, db.NextRevisionParams{TenantID: tenant, AutomationID: id})
			if err != nil {
				return err
			}
			if err = q.AddVersion(ctx, db.AddVersionParams{TenantID: tenant, AutomationID: id, Revision: int64(revision), Document: a.Draft}); err != nil {
				return err
			}
			if err = q.SetPublished(ctx, db.SetPublishedParams{TenantID: tenant, ID: id, Revision: int64(revision)}); err != nil {
				return err
			}
			for _, scope := range d.Trigger.Scopes {
				affected = append(affected, scope.AccountID)
				if err = q.BindAccount(ctx, db.BindAccountParams{TenantID: tenant, AccountID: scope.AccountID, AutomationID: id, Revision: int64(revision), PostScope: scope.Posts.Mode}); err != nil {
					return err
				}
				for _, post := range scope.Posts.IDs {
					if err = q.BindPost(ctx, db.BindPostParams{TenantID: tenant, AccountID: scope.AccountID, PostID: post, AutomationID: id}); err != nil {
						return err
					}
				}
			}
		case "disable":
			err = q.DisableAutomation(ctx, db.DisableAutomationParams{TenantID: tenant, ID: id})
		case "delete":
			err = q.DeleteAutomation(ctx, db.DeleteAutomationParams{TenantID: tenant, ID: id})
		}
		if err != nil {
			return err
		}
		return advance(ctx, q, affected)
	})
}
func advance(ctx context.Context, q *db.Queries, accounts []string) error {
	sort.Strings(accounts)
	previous := ""
	for _, id := range accounts {
		if id == previous {
			continue
		}
		previous = id
		generation, err := q.AdvanceGeneration(ctx, id)
		if err != nil {
			return err
		}
		if err = q.AddRuleChange(ctx, db.AddRuleChangeParams{AccountID: id, Generation: generation}); err != nil {
			return err
		}
	}
	return nil
}
