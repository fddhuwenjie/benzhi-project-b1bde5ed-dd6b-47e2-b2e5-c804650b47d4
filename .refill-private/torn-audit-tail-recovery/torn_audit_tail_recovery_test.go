package torn_audit_tail_recovery

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cablewindow/internal/domain"
	"cablewindow/internal/store"
)

type requestRecord struct {
	Signature string                  `json:"signature"`
	CaseID    string                  `json:"case_id"`
	Revision  int64                   `json:"revision"`
	Case      *domain.MaintenanceCase `json:"case"`
}

type transaction struct {
	Case      *domain.MaintenanceCase `json:"case"`
	Event     domain.AuditEvent       `json:"event"`
	RequestID string                  `json:"request_id"`
	Record    requestRecord           `json:"record"`
}

func TestPendingTransactionRepairsTornAuditTail(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	now := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	repo, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := domain.NewCase("case-torn-tail", "SEG-1", "维护", now, now.Add(time.Hour), "协调员", "基线", now)
	if _, err = repo.Create(ctx, domain.Mutation{Case: c, RequestID: "request-1", Signature: "first", EventType: "case_created", Actor: "协调员"}); err != nil {
		t.Fatal(err)
	}

	auditFile, err := os.Open(filepath.Join(dir, "audit.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var previous domain.AuditEvent
	if err = json.NewDecoder(bufio.NewReader(auditFile)).Decode(&previous); err != nil {
		auditFile.Close()
		t.Fatal(err)
	}
	if err = auditFile.Close(); err != nil {
		t.Fatal(err)
	}
	c.Revision = 2
	c.UpdatedAt = now.Add(time.Minute)
	event := domain.AuditEvent{Sequence: 2, CaseID: c.ID, RequestID: "request-2", EventType: "synthetic_update", Actor: "协调员", OccurredAt: c.UpdatedAt, FromRevision: 1, ToRevision: 2, PayloadDigest: "second", PreviousHash: previous.EventHash}
	b, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(b)
	event.EventHash = hex.EncodeToString(sum[:])
	tx := transaction{Case: c, Event: event, RequestID: event.RequestID, Record: requestRecord{Signature: "second", CaseID: c.ID, Revision: 2, Case: c}}
	b, err = json.Marshal(tx)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "transactions", event.EventHash+".json"), b, 0o640); err != nil {
		t.Fatal(err)
	}
	auditFile, err = os.OpenFile(filepath.Join(dir, "audit.jsonl"), os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = auditFile.WriteString(`{"sequence":2,"case_id":"case-torn-tail"`); err != nil {
		auditFile.Close()
		t.Fatal(err)
	}
	if err = auditFile.Close(); err != nil {
		t.Fatal(err)
	}

	recovered, err := store.Open(dir)
	if err != nil {
		t.Fatalf("完整事务日志应修复未写完的审计尾记录，实际返回: %v", err)
	}
	verification, err := recovered.VerifyAudit(ctx, c.ID)
	if err != nil || !verification.Valid || verification.EventCount != 2 {
		t.Fatalf("恢复后的审计链无效: verification=%+v err=%v", verification, err)
	}
}
