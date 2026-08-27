package context_commit_ghost_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"cablewindow/internal/domain"
	"cablewindow/internal/store"
)

type cancelAtSecondCheck struct {
	mu     sync.Mutex
	done   chan struct{}
	checks int
}

func newCancelAtSecondCheck() *cancelAtSecondCheck {
	return &cancelAtSecondCheck{done: make(chan struct{})}
}

func (c *cancelAtSecondCheck) Deadline() (time.Time, bool) { return time.Time{}, false }

func (c *cancelAtSecondCheck) Done() <-chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks++
	if c.checks == 2 {
		close(c.done)
	}
	return c.done
}

func (c *cancelAtSecondCheck) Err() error {
	select {
	case <-c.done:
		return context.Canceled
	default:
		return nil
	}
}

func (c *cancelAtSecondCheck) Value(any) any { return nil }

func TestCancellationAfterDurableCommitDoesNotCreateGhost(t *testing.T) {
	dir := t.TempDir()
	repo, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	c, err := domain.NewCase("case-context-commit", "SCS-17/KP44", "接续盒更换", now, now.Add(8*time.Hour), "协调员", "常规风险", now)
	if err != nil {
		t.Fatal(err)
	}
	ctx := newCancelAtSecondCheck()
	_, createErr := repo.Create(ctx, domain.Mutation{
		Case: c, RequestID: "context-commit-1", Signature: "create-context-case", EventType: "case_created", Actor: "协调员",
	})
	if createErr == nil {
		committed, getErr := repo.Get(context.Background(), c.ID)
		if getErr != nil {
			t.Fatalf("提交成功但快照不可读: %v", getErr)
		}
		if committed.Revision != 1 {
			t.Fatalf("提交成功但修订错误: %d", committed.Revision)
		}
		return
	}
	if !errors.Is(createErr, context.Canceled) {
		t.Fatalf("创建返回了非取消错误: %v", createErr)
	}

	reopened, err := store.Open(dir)
	if err != nil {
		t.Fatalf("取消后重启失败: %v", err)
	}
	if recovered, getErr := reopened.Get(context.Background(), c.ID); getErr == nil {
		t.Fatalf("调用方收到取消错误后，重启却恢复出修订 %d 的幽灵提交", recovered.Revision)
	} else if !domain.IsCode(getErr, "not_found") {
		t.Fatalf("查询取消后的个案失败: %v", getErr)
	}
}
