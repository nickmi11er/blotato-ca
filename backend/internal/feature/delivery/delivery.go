package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	db "blotato-comment-automation/internal/db/sqlcDB/execution"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Request struct {
	ID            string `json:"idempotency_key"`
	Kind          string `json:"kind"`
	AccountID     string `json:"account_id"`
	ParticipantID string `json:"participant_id"`
	Platform      string `json:"platform"`
	PostID        string `json:"post_id,omitempty"`
	CommentID     string `json:"comment_id,omitempty"`
	Text          string `json:"text,omitempty"`
	Email         string `json:"email,omitempty"`
	Subject       string `json:"subject,omitempty"`
	Purpose       string `json:"purpose,omitempty"`
}
type Result struct {
	Status            string        `json:"status"`
	ProviderMessageID string        `json:"provider_message_id,omitempty"`
	Reason            string        `json:"reason,omitempty"`
	RetryAfter        time.Duration `json:"retry_after,omitempty"`
}
type Sender interface {
	Send(context.Context, Request) Result
	Idempotent() bool
}
type Config struct {
	Mode        string `yaml:"mode"`
	URL         string `yaml:"url"`
	Token       string `yaml:"token"`
	Idempotency bool   `yaml:"idempotency"`
}

func (c Config) Validate() error {
	if c.Mode == "recording" {
		return nil
	}
	if c.Mode != "http" {
		return errors.New("delivery.mode must be recording or http")
	}
	u, err := url.Parse(c.URL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return errors.New("delivery.url must be an HTTP endpoint")
	}
	return nil
}

type Recorder struct{ q *db.Queries }

func NewRecorder(pool *pgxpool.Pool) *Recorder { return &Recorder{q: db.New(pool)} }
func (*Recorder) Idempotent() bool             { return true }
func (r *Recorder) Send(ctx context.Context, request Request) Result {
	raw, err := json.Marshal(request)
	if err != nil {
		return Result{Status: "failed", Reason: err.Error()}
	}
	if err = r.q.RecordDelivery(ctx, db.RecordDeliveryParams{IdempotencyKey: request.ID, Payload: raw}); err != nil {
		return Result{Status: "retry", Reason: "recording storage unavailable"}
	}
	stored, err := r.q.RecordedDelivery(ctx, request.ID)
	if err != nil {
		return Result{Status: "retry", Reason: err.Error()}
	}
	var old Request
	if json.Unmarshal(stored, &old) != nil || old != request {
		return Result{Status: "failed", Reason: "idempotency key reused with different payload"}
	}
	return Result{Status: "succeeded", ProviderMessageID: "recorded:" + request.ID}
}

type HTTP struct {
	cfg    Config
	client *http.Client
}

func NewHTTP(cfg Config) *HTTP {
	return &HTTP{cfg: cfg, client: &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (h *HTTP) Idempotent() bool { return h.cfg.Idempotency }
func (h *HTTP) Send(ctx context.Context, request Request) Result {
	raw, err := json.Marshal(request)
	if err != nil {
		return Result{Status: "failed", Reason: err.Error()}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.cfg.URL, bytes.NewReader(raw))
	if err != nil {
		return Result{Status: "failed", Reason: err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", request.ID)
	if h.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+h.cfg.Token)
	}
	response, err := h.client.Do(req)
	uncertain := func(reason string) Result {
		status := "unknown"
		if h.Idempotent() {
			status = "retry"
		}
		return Result{Status: status, Reason: reason}
	}
	if err != nil {
		return uncertain("integration response unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode == 429 {
		delay := time.Second
		if n, e := strconv.Atoi(response.Header.Get("Retry-After")); e == nil && n > 0 {
			delay = time.Duration(min(n, 3600)) * time.Second
		}
		return Result{Status: "retry", Reason: "integration rate limited", RetryAfter: delay}
	}
	if response.StatusCode >= 500 {
		return uncertain("integration server error")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return Result{Status: "failed", Reason: "integration rejected request"}
	}
	var result Result
	decoder := json.NewDecoder(io.LimitReader(response.Body, 65536))
	if decoder.Decode(&result) != nil {
		return uncertain("invalid integration response")
	}
	// Contract: succeeded means the integration confirmed the actual send. HTTP 202
	// or merely accepting a command does not establish successful delivery.
	if response.StatusCode == 202 {
		return uncertain("integration has not confirmed sending")
	}
	switch result.Status {
	case "succeeded":
		if result.ProviderMessageID == "" {
			return uncertain("missing provider receipt")
		}
	case "retry", "failed": // These explicitly guarantee that no external send occurred.
	case "unknown":
		if h.Idempotent() {
			result.Status = "retry"
		}
	default:
		return uncertain("unsupported integration outcome")
	}
	return result
}
