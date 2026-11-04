package inbound

import (
	"context"

	"blotato-comment-automation/internal/feature/automation"
)

type Repository interface {
	Record(context.Context, Comment, []automation.Match) (bool, error)
}
type Matcher interface {
	Match(context.Context, string, string, string, string) ([]automation.Match, error)
}
