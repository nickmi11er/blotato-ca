package delivery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPDeliveryContract(t *testing.T) {
	for _, tc := range []struct {
		name string
		code int
		body string
		safe bool
		want string
	}{
		{"confirmed", 200, `{"status":"succeeded","provider_message_id":"message"}`, false, "succeeded"},
		{"queued is not sent", 202, `{"status":"succeeded","provider_message_id":"message"}`, false, "unknown"},
		{"ambiguous", 503, ``, false, "unknown"},
		{"retry with idempotency", 503, ``, true, "retry"},
		{"rate limit", 429, ``, false, "retry"},
		{"bad receipt", 200, `{"status":"succeeded"}`, false, "unknown"},
		{"permanent", 400, ``, false, "failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Idempotency-Key") != "send-key" {
					t.Error("missing idempotency key")
				}
				w.WriteHeader(tc.code)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			sender := NewHTTP(Config{URL: server.URL, Idempotency: tc.safe})
			result := sender.Send(t.Context(), Request{ID: "send-key", Kind: "dm"})
			if result.Status != tc.want {
				t.Fatal(result)
			}
		})
	}
}
func TestCancelledRequestIsUnknownWithoutIdempotency(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	result := NewHTTP(Config{URL: "http://127.0.0.1:1"}).Send(ctx, Request{ID: "send"})
	if result.Status != "unknown" {
		t.Fatal(result)
	}
}
