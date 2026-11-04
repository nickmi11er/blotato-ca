package inbound

import (
	"context"
	"errors"
	"testing"

	"blotato-comment-automation/internal/feature/automation"
)

type matcherStub struct {
	calls   int
	err     error
	matches []automation.Match
}

func (m *matcherStub) Match(context.Context, string, string, string, string) ([]automation.Match, error) {
	m.calls++
	return m.matches, m.err
}

type repositoryStub struct {
	calls int
	err   error
}

func (r *repositoryStub) Record(context.Context, Comment, []automation.Match) (bool, error) {
	r.calls++
	return true, r.err
}

const external = `{"type":"CommentReceived","schema_version":1,"platform":"instagram","account_id":"a","occurred_at":"2026-11-03T09:15:00Z","data":{"post_id":"p","comment_id":"c","author_id":"u","is_own_comment":false,"text":"pricing"}}`

func TestHandlerSkipsDatabaseForNonmatchesAndPropagatesFailures(t *testing.T) {
	m := &matcherStub{}
	r := &repositoryStub{}
	h := NewHandler(r, m)
	if err := h.Handle(t.Context(), []byte(external)); err != nil || r.calls != 0 {
		t.Fatal("nonmatch wrote to database", err)
	}
	m.err = errors.New("database offline")
	if err := h.Handle(t.Context(), []byte(external)); err == nil || r.calls != 0 {
		t.Fatal("failed fill acknowledged")
	}
	m.err = nil
	m.matches = []automation.Match{{AutomationID: "automation"}}
	r.err = errors.New("commit failed")
	if err := h.Handle(t.Context(), []byte(external)); err == nil || r.calls != 1 {
		t.Fatal("failed admission acknowledged")
	}
}
func TestOwnCommentAndInvalidEventsDoNotReachMatcher(t *testing.T) {
	m := &matcherStub{}
	h := NewHandler(&repositoryStub{}, m)
	own := `{"type":"CommentReceived","schema_version":1,"platform":"instagram","account_id":"a","occurred_at":"2026-11-03T09:15:00Z","data":{"post_id":"p","comment_id":"c","author_id":"a","is_own_comment":true,"text":"pricing"}}`
	if err := h.Handle(t.Context(), []byte(own)); err != nil {
		t.Fatal(err)
	}
	if err := h.Handle(t.Context(), []byte(`{"schema_version":99}`)); err == nil {
		t.Fatal("invalid event accepted")
	}
	if m.calls != 0 {
		t.Fatal("unexpected matcher call")
	}
}
