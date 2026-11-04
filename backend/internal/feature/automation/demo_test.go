package automation_test

import (
	"errors"
	"os"
	"testing"

	"blotato-comment-automation/internal/feature/automation"
	"blotato-comment-automation/internal/testutil"
)

func TestDemoInitializationPreservesEditsAndDeletions(t *testing.T) {
	pool := testutil.Database(t)
	sql, err := os.ReadFile("../../../scripts/seed-demo.sql")
	if err != nil {
		t.Fatal(err)
	}
	seed := func() {
		t.Helper()
		if _, err := pool.Exec(t.Context(), string(sql)); err != nil {
			t.Fatal(err)
		}
	}
	seed()
	service := automation.NewService(pool)
	templates, err := service.Templates(t.Context(), "northstar")
	if err != nil || len(templates) != 3 {
		t.Fatal(templates, err)
	}
	flows, err := service.List(t.Context(), "northstar")
	if err != nil || len(flows) != 1 || flows[0].Enabled {
		t.Fatal(flows, err)
	}
	catalog, err := service.Catalog(t.Context(), "northstar")
	if err != nil || len(catalog.Accounts) != 3 || len(catalog.Posts) != 4 {
		t.Fatal(catalog, err)
	}
	// The starter is a complete executable workflow, not merely a UI fixture.
	if err = service.Publish(t.Context(), "northstar", flows[0].ID); err != nil {
		t.Fatal(err)
	}
	changed := templates[0]
	changed.Subject = "My edited subject"
	if err = service.SaveTemplate(t.Context(), "northstar", changed); err != nil {
		t.Fatal(err)
	}
	if err = service.Delete(t.Context(), "northstar", flows[0].ID); err != nil {
		t.Fatal(err)
	}
	if err = service.DeleteTemplate(t.Context(), "northstar", templates[1].ID, nil); err != nil {
		t.Fatal(err)
	}
	seed()
	if _, err = service.Get(t.Context(), "northstar", flows[0].ID); !errors.Is(err, automation.ErrNotFound) {
		t.Fatal("deleted starter recreated", err)
	}
	templates, err = service.Templates(t.Context(), "northstar")
	if err != nil || len(templates) != 2 {
		t.Fatal("deleted template recreated", templates, err)
	}
	for _, template := range templates {
		if template.ID == changed.ID && (template.Subject != changed.Subject || template.Revision != changed.Revision+1) {
			t.Fatal("edited template overwritten", template)
		}
	}
}
