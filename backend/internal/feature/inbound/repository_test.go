package inbound_test

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"blotato-comment-automation/internal/db"
	"blotato-comment-automation/internal/feature/inbound"
	"blotato-comment-automation/shared/postgres"
)

func TestConcurrentDeliveriesRecordOneComment(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to an isolated PostgreSQL database")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	database, err := postgres.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.Migrate(ctx, database.GetPool()); err != nil {
		t.Fatal(err)
	}
	c := inbound.Comment{Type: "CommentReceived", Platform: "instagram", AccountID: fmt.Sprintf("test-%d", time.Now().UnixNano())}
	c.Data.PostID, c.Data.CommentID = "post", "comment"
	defer database.Exec(ctx, "DELETE FROM processed_comments WHERE account_id = $1", c.AccountID)
	store := inbound.NewRepository(database)
	var inserted atomic.Int32
	done := make(chan error, 8)
	for range cap(done) {
		go func() {
			fresh, err := store.Record(ctx, c)
			if fresh {
				inserted.Add(1)
			}
			done <- err
		}()
	}
	for range cap(done) {
		if err := <-done; err != nil {
			t.Error(err)
		}
	}
	if inserted.Load() != 1 {
		t.Fatalf("got %d successful inserts, want 1", inserted.Load())
	}
}
