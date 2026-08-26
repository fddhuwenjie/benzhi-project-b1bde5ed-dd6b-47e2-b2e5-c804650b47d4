package store

import (
	"context"
	"testing"
	"time"

	"cablewindow/internal/domain"
)

func TestIdempotencyConflictAndRecovery(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	now := time.Now().UTC()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := domain.NewCase("case-store", "S", "范围", now, now.Add(time.Hour), "协调", "风险", now)
	m := domain.Mutation{Case: c, RequestID: "request-1", Signature: "same", EventType: "create", Actor: "协调"}
	first, err := s.Create(ctx, m)
	if err != nil {
		t.Fatal(err)
	}
	prior, ok, err := s.LookupRequest(ctx, "request-1", "same")
	if err != nil || !ok || !prior.Idempotent || prior.Case.ID != first.Case.ID {
		t.Fatalf("幂等读取失败：%v", err)
	}
	if _, _, err := s.LookupRequest(ctx, "request-1", "different"); !domain.IsCode(err, "idempotency_conflict") {
		t.Fatalf("载荷冲突未识别：%v", err)
	}
	reopened, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := reopened.Get(ctx, c.ID)
	if err != nil || got.Revision != 1 {
		t.Fatalf("恢复快照失败：%v", err)
	}
	events, err := reopened.Audit(ctx, c.ID, 0, 10)
	if err != nil || len(events) != 1 || events[0].PreviousHash != "" || events[0].EventHash == "" {
		t.Fatalf("恢复审计失败：%v", err)
	}
}

func TestRevisionConflict(t *testing.T) {
	s, _ := Open(t.TempDir())
	now := time.Now().UTC()
	c, _ := domain.NewCase("case-revision", "S", "范围", now, now.Add(time.Hour), "协调", "风险", now)
	_, _ = s.Create(context.Background(), domain.Mutation{Case: c, RequestID: "r1", Signature: "one", EventType: "create", Actor: "a"})
	c.Revision = 2
	_, err := s.Commit(context.Background(), domain.Mutation{Case: c, ExpectedRevision: 9, RequestID: "r2", Signature: "two", EventType: "update", Actor: "a"})
	if !domain.IsCode(err, "revision_conflict") {
		t.Fatalf("期望修订冲突，得到 %v", err)
	}
}

func TestPendingTransactionIsReplayedOnOpen(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	now := time.Now().UTC()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := domain.NewCase("case-transaction", "S", "范围", now, now.Add(time.Hour), "协调", "风险", now)
	_, err = s.Create(ctx, domain.Mutation{Case: c, RequestID: "tx-1", Signature: "create", EventType: "create", Actor: "协调"})
	if err != nil {
		t.Fatal(err)
	}
	c, _ = s.Get(ctx, c.ID)
	c.Revision++
	c.UpdatedAt = now.Add(time.Minute)
	previous := s.events[len(s.events)-1].EventHash
	event := domain.AuditEvent{Sequence: 2, CaseID: c.ID, RequestID: "tx-2", EventType: "synthetic_recovery", Actor: "协调", OccurredAt: c.UpdatedAt, FromRevision: 1, ToRevision: 2, PayloadDigest: "payload", PreviousHash: previous}
	event.EventHash, err = eventHash(event)
	if err != nil {
		t.Fatal(err)
	}
	record := requestRecord{Signature: "recover", CaseID: c.ID, Revision: 2, Case: cloneCase(c)}
	if _, err := s.writeTransaction(transaction{Case: c, Event: event, RequestID: "tx-2", Record: record}); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := reopened.Get(ctx, c.ID)
	if err != nil || recovered.Revision != 2 {
		t.Fatalf("事务恢复后的修订错误：%v", err)
	}
	result, ok, err := reopened.LookupRequest(ctx, "tx-2", "recover")
	if err != nil || !ok || result.Case.Revision != 2 {
		t.Fatalf("事务恢复后的幂等索引错误：%v", err)
	}
	verification, err := reopened.VerifyAudit(ctx, c.ID)
	if err != nil || !verification.Valid || verification.EventCount != 2 {
		t.Fatalf("事务恢复后的审计链错误：%v", err)
	}
}
