package conversation

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

type sourceStub struct {
	route           Route
	loads, accepted int
	fail            bool
}

func (s *sourceStub) Route(context.Context, string, string) (Route, error) {
	s.loads++
	if s.fail {
		return Route{}, errors.New("offline")
	}
	return s.route, nil
}
func (s *sourceStub) Accept(_ context.Context, _ Message, r Route) error {
	if r != s.route {
		return ErrStale
	}
	s.accepted++
	return nil
}
func (s *sourceStub) Activate(_ context.Context, a Activation) error {
	s.route = Route{SessionID: a.SessionID, Epoch: a.Epoch, Platform: a.Platform}
	return nil
}
func message(participant string) Message {
	m := Message{Platform: "instagram", AccountID: "a"}
	own, text := false, "email@example.com"
	m.Data.ParticipantID = participant
	m.Data.Own = &own
	m.Data.Text = &text
	return m
}
func TestNegativeCacheActivationReplayAndNewAssignment(t *testing.T) {
	s := &sourceStub{}
	cfg := DefaultConfig()
	r := NewRouter(s, cfg)
	for range 1000 {
		if err := r.Message(t.Context(), message("u")); err != nil {
			t.Fatal(err)
		}
	}
	if s.loads != 1 || s.accepted != 0 {
		t.Fatal("irrelevant DMs queried database")
	}
	a := Activation{Type: "ActivateConversation", Version: 1, Platform: "instagram", AccountID: "a", ParticipantID: "u", SessionID: "session", Epoch: 1}
	// Simulate a crash after database activation but before the old cache was updated.
	if err := s.Activate(t.Context(), a); err != nil {
		t.Fatal(err)
	}
	if err := r.Activate(t.Context(), a); err != nil {
		t.Fatal(err)
	}
	if err := r.Message(t.Context(), message("u")); err != nil || s.accepted != 1 {
		t.Fatal("activation did not clear cached absence", err)
	}
	fresh := NewRouter(s, cfg)
	if err := fresh.Message(t.Context(), message("u")); err != nil {
		t.Fatal(err)
	}
	if s.loads != 3 {
		t.Fatal("new assignment reused old cache")
	}
	s.route = Route{SessionID: "next", Epoch: 2, Platform: "instagram"}
	if err := r.Message(t.Context(), message("u")); err != nil || s.accepted != 3 {
		t.Fatal("stale positive did not reroute", err)
	}
}
func TestCacheBoundsAndFailure(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxBytes = 1024
	s := &sourceStub{}
	r := NewRouter(s, cfg)
	now := time.Now()
	r.now = func() time.Time { return now }
	for i := range 1000 {
		if err := r.Message(t.Context(), message(fmt.Sprint(i))); err != nil {
			t.Fatal(err)
		}
	}
	if r.bytes > cfg.MaxBytes || len(r.entries) > 4 {
		t.Fatal("cache unbounded")
	}
	now = now.Add(cfg.IdleTTL)
	r.Prune()
	if r.bytes != 0 {
		t.Fatal("idle entries retained")
	}
	s.fail = true
	if err := r.Message(t.Context(), message("new")); err == nil || len(r.entries) != 0 {
		t.Fatal("load failure cached as absence")
	}
}
