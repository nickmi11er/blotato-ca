package di

import (
	"context"
	"fmt"

	"blotato-comment-automation/internal/db"
	"blotato-comment-automation/shared/postgres"
)

func (s *Services) DB() (*postgres.DB, error) {
	if s.database != nil {
		return s.database, nil
	}
	database, err := postgres.New(s.Ctx(), s.Config().Database.URL)
	if err != nil {
		return nil, err
	}
	if err = db.Migrate(s.Ctx(), database.GetPool()); err != nil {
		database.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}
	s.Gc().Add(func(context.Context) error {
		database.Close()
		return nil
	})
	s.database = database
	return database, nil
}
