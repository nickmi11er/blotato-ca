package automation

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

const MaxDocumentBytes = 256 << 10
const MaxAutomations = 100

var (
	ErrNotFound  = errors.New("automation not found")
	ErrLimit     = errors.New("workspace automation limit reached")
	ErrOwnership = errors.New("account belongs to another workspace or platform")
)

type Document struct {
	Name     string          `json:"name"`
	Priority int             `json:"priority,omitempty"`
	Trigger  Trigger         `json:"trigger"`
	Workflow json.RawMessage `json:"workflow"`
}
type Trigger struct {
	SchemaVersion int       `json:"schema_version"`
	Scopes        []Scope   `json:"scopes"`
	Comment       Predicate `json:"comment"`
	Repeat        Repeat    `json:"repeat"`
}
type Scope struct {
	AccountID string `json:"account_id"`
	Posts     Posts  `json:"posts"`
}
type Posts struct {
	Mode string   `json:"mode"`
	IDs  []string `json:"ids,omitempty"`
}
type Predicate struct {
	Mode          string   `json:"mode"`
	Operator      string   `json:"operator"`
	Logic         string   `json:"logic"`
	Keywords      []string `json:"keywords"`
	Exclusions    []string `json:"exclusions"`
	CaseSensitive bool     `json:"case_sensitive"`
}
type Repeat struct {
	Mode string `json:"mode"`
}

type Match struct {
	TenantID     string `json:"tenant_id"`
	AutomationID string `json:"automation_id"`
	Revision     int64  `json:"revision"`
	Repeat       string `json:"repeat"`
	Priority     int    `json:"priority,omitempty"`
}

func ValidID(s string) bool {
	return len(s) > 0 && len(s) <= 200 && utf8.ValidString(s) && strings.TrimSpace(s) == s && !strings.ContainsAny(s, "\x00\r\n")
}

func (d Document) Validate() error      { return d.validate(false) }
func (d Document) ValidateDraft() error { return d.validate(true) }
func (d Document) validate(draft bool) error {
	if strings.TrimSpace(d.Name) == "" || len(d.Name) > 200 {
		return errors.New("name must contain 1–200 bytes")
	}
	b, err := json.Marshal(d)
	if err != nil || len(b) > MaxDocumentBytes {
		return errors.New("document exceeds size limit or contains invalid JSON")
	}
	if len(d.Workflow) == 0 || !json.Valid(d.Workflow) {
		return errors.New("workflow JSON is required")
	}
	t := d.Trigger
	if t.SchemaVersion != 1 {
		return errors.New("unsupported trigger schema_version")
	}
	if (!draft && len(t.Scopes) == 0) || len(t.Scopes) > 20 {
		return errors.New("select 1–20 accounts")
	}
	accounts := map[string]bool{}
	totalPosts := 0
	for _, scope := range t.Scopes {
		if !ValidID(scope.AccountID) || accounts[scope.AccountID] {
			return errors.New("invalid or duplicate account")
		}
		accounts[scope.AccountID] = true
		switch scope.Posts.Mode {
		case "all":
			if len(scope.Posts.IDs) != 0 {
				return errors.New("all-post scope cannot contain selected posts")
			}
		case "selected":
			if !draft && len(scope.Posts.IDs) == 0 {
				return errors.New("selected scope requires posts")
			}
			posts := map[string]bool{}
			for _, id := range scope.Posts.IDs {
				if !ValidID(id) || posts[id] {
					return errors.New("invalid or duplicate post")
				}
				posts[id] = true
			}
			totalPosts += len(scope.Posts.IDs)
		default:
			return errors.New("invalid post scope")
		}
	}
	if totalPosts > 1000 {
		return errors.New("at most 1000 selected posts are allowed")
	}
	if t.Repeat.Mode != "every" && t.Repeat.Mode != "once" && t.Repeat.Mode != "skip_active" {
		return errors.New("invalid repeat mode")
	}
	return t.Comment.validate(draft)
}

func (p Predicate) Validate() error { return p.validate(false) }
func (p Predicate) validate(draft bool) error {
	if p.Mode != "any" && p.Mode != "keywords" {
		return errors.New("invalid comment mode")
	}
	if p.Operator != "exact" && p.Operator != "contains" && p.Operator != "whole_word" {
		return errors.New("invalid matching operator")
	}
	if p.Logic != "any" && p.Logic != "all" {
		return errors.New("invalid keyword logic")
	}
	if !draft && p.Mode == "keywords" && len(p.Keywords) == 0 {
		return errors.New("keywords are required")
	}
	for name, terms := range map[string][]string{"keywords": p.Keywords, "exclusions": p.Exclusions} {
		if len(terms) > 32 {
			return fmt.Errorf("at most 32 %s are allowed", name)
		}
		for _, term := range terms {
			if strings.TrimSpace(term) == "" || len(term) > 256 || !utf8.ValidString(term) || strings.ContainsRune(term, 0) {
				return fmt.Errorf("invalid %s term", name)
			}
		}
	}
	return nil
}
