package close_retry_archive

import (
	"context"
	"testing"
	"time"

	"cablewindow/internal/domain"
	"cablewindow/internal/store"
	"cablewindow/internal/workflow"
)

func TestCloseRetryPreservesArchiveResult(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	repo, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c, _ := domain.NewCase("case-close-retry", "SEG-1", "维护", now.Add(-time.Hour), now.Add(time.Hour), "协调员", "基线", now)
	c.State = domain.StatePendingClose
	for _, category := range domain.RequiredClosure {
		c.ClosureEvidence = append(c.ClosureEvidence, domain.ClosureEvidence{Category: category, Reference: "REF-" + category, Actor: "现场负责人", SubmittedAt: now})
	}
	if _, err = repo.Create(ctx, domain.Mutation{Case: c, RequestID: "setup", Signature: "setup", EventType: "case_created", Actor: "协调员"}); err != nil {
		t.Fatal(err)
	}
	service := workflow.New(repo, func() time.Time { return now })
	env := workflow.Envelope{RequestID: "close-once", Actor: "现场负责人", Role: "site_lead", ExpectedRevision: 1, Payload: []byte(`{}`)}
	first, err := service.Execute(ctx, c.ID, "close", env)
	if err != nil {
		t.Fatal(err)
	}
	if first.Case.Archive == nil || first.Case.Archive.Digest == "" {
		t.Fatal("首次关闭结果缺少归档包")
	}
	retry, err := service.Execute(ctx, c.ID, "close", env)
	if err != nil {
		t.Fatal(err)
	}
	if !retry.Idempotent || retry.Case.Archive == nil || retry.Case.Archive.Digest != first.Case.Archive.Digest {
		t.Fatalf("幂等重试没有返回首次关闭的完整归档结果: first_archive=%v retry_archive=%v", first.Case.Archive != nil, retry.Case.Archive != nil)
	}
}
