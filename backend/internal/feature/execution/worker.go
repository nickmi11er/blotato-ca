package execution

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math/rand/v2"
	"time"

	db "blotato-comment-automation/internal/db/sqlcDB/execution"
	"blotato-comment-automation/internal/feature/conversation"
	"blotato-comment-automation/internal/feature/delivery"
	"github.com/jackc/pgx/v5"
	"golang.org/x/sync/errgroup"
)

type Config struct {
	Workers      int           `yaml:"workers"`
	PollInterval time.Duration `yaml:"poll_interval"`
	Lease        time.Duration `yaml:"lease"`
	JobTimeout   time.Duration `yaml:"job_timeout"`
	MaxAttempts  int           `yaml:"max_attempts"`
}

func DefaultConfig() Config {
	return Config{Workers: 4, PollInterval: 500 * time.Millisecond, Lease: 30 * time.Second, JobTimeout: 10 * time.Second, MaxAttempts: 5}
}
func (c Config) Validate() error {
	if c.Workers < 1 || c.Workers > 64 || c.PollInterval <= 0 || c.JobTimeout <= 0 || c.Lease < 2*c.JobTimeout || c.MaxAttempts < 1 {
		return errors.New("invalid execution worker configuration")
	}
	return nil
}

type ActivationPublisher interface {
	PublishActivation(context.Context, conversation.Activation) error
}
type Worker struct {
	store     *Store
	engine    *Engine
	sender    delivery.Sender
	publisher ActivationPublisher
	cfg       Config
}

func NewWorker(store *Store, engine *Engine, sender delivery.Sender, publisher ActivationPublisher, cfg Config) *Worker {
	return &Worker{store: store, engine: engine, sender: sender, publisher: publisher, cfg: cfg}
}
func (w *Worker) Once(ctx context.Context) (bool, error) {
	q := db.New(w.store.pool)
	job, err := q.ClaimJob(ctx, w.cfg.Lease.Milliseconds())
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	bounded, cancel := context.WithTimeout(ctx, w.cfg.JobTimeout)
	defer cancel()
	switch job.Kind {
	case "deliver":
		err = w.engine.CheckSend(bounded, job)
		if err == nil {
			var request delivery.Request
			if e := json.Unmarshal(job.Payload, &request); e != nil {
				err = e
			} else {
				result := w.sender.Send(bounded, request)
				err = w.engine.Complete(bounded, job, result)
			}
		}
	case "activate":
		var activation conversation.Activation
		if err = json.Unmarshal(job.Payload, &activation); err == nil {
			err = w.publisher.PublishActivation(bounded, activation)
			result := delivery.Result{Status: "succeeded"}
			if err != nil {
				result = delivery.Result{Status: "retry", Reason: "activation publication unavailable"}
			}
			err = w.engine.Complete(bounded, job, result)
		}
	default:
		err = w.engine.Process(bounded, job)
	}
	if errors.Is(err, ErrLease) {
		return true, nil
	}
	if err != nil {
		// Delivery may already have happened. Its lease recovery uses the adapter's
		// idempotency contract; do not blindly requeue it after an uncertain commit.
		if job.Kind != "deliver" {
			_, _ = q.RetryJob(ctx, db.RetryJobParams{ID: job.ID, LeaseToken: job.LeaseToken, Result: []byte(`{"reason":"processing error"}`), DelayMs: 1000})
		}
		return true, err
	}
	return true, nil
}
func (w *Worker) Recover(ctx context.Context) error {
	_, err := db.New(w.store.pool).RecoverJobs(ctx, w.sender.Idempotent())
	return err
}
func (w *Worker) Run(ctx context.Context) error {
	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		ticker := time.NewTicker(w.cfg.Lease / 3)
		defer ticker.Stop()
		for {
			bounded, cancel := context.WithTimeout(ctx, w.cfg.JobTimeout)
			err := w.Recover(bounded)
			cancel()
			if err != nil && ctx.Err() == nil {
				slog.WarnContext(ctx, "job recovery will retry", "error", err)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
			}
		}
	})
	for range w.cfg.Workers {
		group.Go(func() error {
			for {
				if err := ctx.Err(); err != nil {
					return err
				}
				worked, err := w.Once(ctx)
				if err != nil && ctx.Err() == nil {
					slog.WarnContext(ctx, "execution job will recover", "error", err)
				}
				if !worked || err != nil {
					timer := time.NewTimer(time.Duration(float64(w.cfg.PollInterval) * (0.8 + rand.Float64()*0.4)))
					select {
					case <-ctx.Done():
						timer.Stop()
						return ctx.Err()
					case <-timer.C:
					}
				}
			}
		})
	}
	return group.Wait()
}
