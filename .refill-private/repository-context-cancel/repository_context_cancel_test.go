package repository_context_cancel

import (
	"context"
	"errors"
	"testing"

	"cablewindow/internal/store"
)

func TestCanceledContextStopsRepositoryQueries(t *testing.T) {
	repo, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = repo.List(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("空库 List 忽略已取消 context: %v", err)
	}
	if _, err = repo.Audit(ctx, "case-any", 0, 10); !errors.Is(err, context.Canceled) {
		t.Fatalf("Audit 忽略已取消 context: %v", err)
	}
}
