package automation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	db "blotato-comment-automation/internal/db/sqlcDB/automation"
	"blotato-comment-automation/internal/feature/workflow"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var ErrConflict = errors.New("this item changed in another window; reload before saving")

func (s *Service) List(ctx context.Context, tenant string) ([]View, error) {
	rows, err := db.New(s.pool).ListAutomations(ctx, tenant)
	if err != nil {
		return nil, err
	}
	result := make([]View, 0, len(rows))
	for _, row := range rows {
		v := View{ID: row.ID, Enabled: row.Enabled, Revision: row.Revision, DraftVersion: row.DraftVersion, PublishedDraftVersion: row.PublishedDraftVersion, UpdatedAt: row.UpdatedAt.Time}
		if err = json.Unmarshal(row.Draft, &v.Draft); err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, nil
}
func (s *Service) MutateVersion(ctx context.Context, tenant, id, action string, expected *int64) error {
	return s.mutate(ctx, tenant, id, action, expected)
}

type AccountView struct {
	ID        string `json:"id"`
	Platform  string `json:"platform"`
	Name      string `json:"name"`
	Handle    string `json:"handle"`
	CanSendDM bool   `json:"canSendDM"`
}
type PostView struct {
	ID          string    `json:"id"`
	AccountID   string    `json:"accountId"`
	Title       string    `json:"title"`
	PublishedAt time.Time `json:"publishedAt"`
}
type Catalog struct {
	Accounts []AccountView `json:"accounts"`
	Posts    []PostView    `json:"posts"`
}

func (s *Service) Catalog(ctx context.Context, tenant string) (Catalog, error) {
	q := db.New(s.pool)
	result := Catalog{Accounts: []AccountView{}, Posts: []PostView{}}
	accounts, err := q.ListAccounts(ctx, tenant)
	if err != nil {
		return result, err
	}
	posts, err := q.ListPosts(ctx, tenant)
	if err != nil {
		return result, err
	}
	for _, a := range accounts {
		result.Accounts = append(result.Accounts, AccountView{a.ID, a.Platform, a.Name, a.Handle, a.CanSendDm})
	}
	for _, p := range posts {
		result.Posts = append(result.Posts, PostView{p.ID, p.AccountID, p.Title, p.PublishedAt.Time})
	}
	return result, nil
}
func (s *Service) AccountDetails(ctx context.Context, tenant string, a AccountView) error {
	if len(a.Name) > 200 || len(a.Handle) > 200 {
		return ErrInvalid
	}
	return s.transaction(ctx, tenant, func(q *db.Queries) error {
		old, err := q.GetAccount(ctx, a.ID)
		if err != nil {
			return err
		}
		if old.TenantID != tenant {
			return ErrOwnership
		}
		return q.UpdateAccountDetails(ctx, db.UpdateAccountDetailsParams{TenantID: tenant, ID: a.ID, Name: a.Name, Handle: a.Handle, CanSendDm: a.CanSendDM})
	})
}
func (s *Service) SavePost(ctx context.Context, tenant string, p PostView) error {
	if !ValidID(p.ID) || !ValidID(p.AccountID) || strings.TrimSpace(p.Title) == "" || len(p.Title) > 1000 || p.PublishedAt.IsZero() {
		return ErrInvalid
	}
	return s.transaction(ctx, tenant, func(q *db.Queries) error {
		a, err := q.GetAccount(ctx, p.AccountID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if a.TenantID != tenant {
			return ErrOwnership
		}
		return q.SavePost(ctx, db.SavePostParams{TenantID: tenant, AccountID: p.AccountID, ID: p.ID, Title: p.Title, PublishedAt: pgtype.Timestamptz{Time: p.PublishedAt, Valid: true}})
	})
}

type EmailTemplate struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Subject  string `json:"subject"`
	Body     string `json:"body"`
	Revision int64  `json:"revision"`
}

func (s *Service) Templates(ctx context.Context, tenant string) ([]EmailTemplate, error) {
	rows, err := db.New(s.pool).ListTemplates(ctx, tenant)
	if err != nil {
		return nil, err
	}
	result := make([]EmailTemplate, 0, len(rows))
	for _, t := range rows {
		result = append(result, EmailTemplate{t.ID, t.Name, t.Subject, t.Body, t.Revision})
	}
	return result, nil
}
func (s *Service) SaveTemplate(ctx context.Context, tenant string, t EmailTemplate) error {
	if !ValidID(t.ID) || strings.TrimSpace(t.Name) == "" || len(t.Name) > 100 || t.Revision < 0 {
		return ErrInvalid
	}
	// Use the execution compiler's template validation for the same send contract.
	raw, _ := json.Marshal(workflow.Definition{Steps: []workflow.Step{{ID: "validate", Kind: "reply", Text: "ok"}}, Templates: map[string]workflow.Template{t.ID: {Revision: 1, Subject: t.Subject, Body: t.Body}}})
	if _, err := workflow.Compile(raw); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	return s.transaction(ctx, tenant, func(q *db.Queries) error {
		old, err := q.GetTemplate(ctx, db.GetTemplateParams{TenantID: tenant, ID: t.ID})
		if errors.Is(err, pgx.ErrNoRows) {
			if t.Revision != 0 {
				return ErrConflict
			}
			n, e := q.CountTemplates(ctx, tenant)
			if e != nil {
				return e
			}
			if n >= 100 {
				return fmt.Errorf("%w: at most 100 templates", ErrInvalid)
			}
		} else if err != nil {
			return err
		} else if old.Revision != t.Revision {
			return ErrConflict
		}
		return q.SaveTemplate(ctx, db.SaveTemplateParams{TenantID: tenant, ID: t.ID, Name: t.Name, Subject: t.Subject, Body: t.Body})
	})
}
func (s *Service) DeleteTemplate(ctx context.Context, tenant, id string, expected *int64) error {
	return s.transaction(ctx, tenant, func(q *db.Queries) error {
		t, err := q.GetTemplate(ctx, db.GetTemplateParams{TenantID: tenant, ID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if expected != nil && t.Revision != *expected {
			return ErrConflict
		}
		automations, err := q.ListAutomations(ctx, tenant)
		if err != nil {
			return err
		}
		for _, a := range automations {
			var d Document
			if err = json.Unmarshal(a.Draft, &d); err != nil {
				return err
			}
			var w workflow.Definition
			if err = json.Unmarshal(d.Workflow, &w); err != nil {
				return err
			}
			if templateIDs(w.Steps)[id] {
				return fmt.Errorf("%w: remove this template from automation drafts before deleting it", ErrInvalid)
			}
		}
		return q.DeleteTemplate(ctx, db.DeleteTemplateParams{TenantID: tenant, ID: id})
	})
}
func templateIDs(steps []workflow.Step) map[string]bool {
	ids := map[string]bool{}
	var walk func([]workflow.Step)
	walk = func(ss []workflow.Step) {
		for _, s := range ss {
			if s.Kind == "email" {
				ids[s.TemplateID] = true
			}
			walk(s.Actions)
			walk(s.TimeoutSteps)
		}
	}
	walk(steps)
	return ids
}
func pinTemplates(ctx context.Context, q *db.Queries, tenant string, d *Document) error {
	var w workflow.Definition
	// Keep unknown fields intact so Compile still rejects unsupported definitions.
	if err := json.Unmarshal(d.Workflow, &w); err != nil {
		return fmt.Errorf("%w: invalid workflow", ErrInvalid)
	}
	if w.Templates == nil {
		w.Templates = map[string]workflow.Template{}
	}
	for id := range templateIDs(w.Steps) {
		t, err := q.GetTemplate(ctx, db.GetTemplateParams{TenantID: tenant, ID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		w.Templates[id] = workflow.Template{Revision: t.Revision, Subject: t.Subject, Body: t.Body}
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(d.Workflow, &raw); err != nil || raw == nil {
		return ErrInvalid
	}
	templates, err := json.Marshal(w.Templates)
	if err != nil {
		return err
	}
	raw["templates"] = templates
	d.Workflow, err = json.Marshal(raw)
	return err
}
func needsDM(plan workflow.Plan) bool {
	for _, n := range plan.Nodes {
		if n.Step.Kind == "dm" || n.Step.Kind == "wait_reply" {
			return true
		}
	}
	return false
}
