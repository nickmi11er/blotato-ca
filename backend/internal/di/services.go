package di

import (
	"context"

	"blotato-comment-automation/internal/config"
	"blotato-comment-automation/shared/graceful"
	"blotato-comment-automation/shared/postgres"
)

// Services resolves dependencies once during single-threaded startup.
type Services struct {
	ctx      context.Context
	cfg      config.Config
	gc       graceful.Graceful
	database *postgres.DB
	kafka    *KafkaModule
	inbound  *InboundModule
}

func NewServices(ctx context.Context, cfg config.Config) *Services {
	return &Services{ctx: ctx, cfg: cfg, gc: graceful.New()}
}

func (s *Services) Ctx() context.Context { return s.ctx }

func (s *Services) Gc() graceful.Graceful { return s.gc }
