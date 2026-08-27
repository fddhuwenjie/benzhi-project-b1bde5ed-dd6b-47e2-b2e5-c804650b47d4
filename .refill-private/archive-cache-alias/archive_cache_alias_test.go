package archive_cache_alias_test

import (
	"context"
	"testing"
	"time"

	"cablewindow/internal/domain"
	"cablewindow/internal/store"
	"cablewindow/internal/workflow"
)

func TestArchiveCacheDoesNotShareMutableResult(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	closedAt := now
	c := &domain.MaintenanceCase{
		ID:           "case-archive-alias",
		CableSegment: "SCS-17/KP44",
		WorkScope:    "接续盒检查",
		State:        domain.StateClosed,
		Revision:     1,
		CreatedAt:    now,
		UpdatedAt:    now,
		ClosedAt:     &closedAt,
		Timeline: []domain.TimelineEntry{{
			Type:     "case_closed",
			Actor:    "协调员",
			Summary:  "个案关闭",
			At:       now,
			Revision: 1,
		}},
		CloseSummary: &domain.CloseSummary{
			CaseID:       "case-archive-alias",
			CableSegment: "SCS-17/KP44",
			WorkScope:    "接续盒检查",
			Coordinator:  "协调员",
			ClosedAt:     closedAt,
			Digest:       "close-digest",
		},
	}
	if err := c.BuildArchive(domain.AuditSummary{CaseID: c.ID, EventCount: 1, FinalRevision: 1, Digest: "audit-digest"}); err != nil {
		t.Fatal(err)
	}
	repo, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Create(ctx, domain.Mutation{Case: c, RequestID: "archive-create", Signature: "create", EventType: "case_created", Actor: "协调员"}); err != nil {
		t.Fatal(err)
	}
	service := workflow.New(repo, func() time.Time { return now })

	first, err := service.Archive(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	first.Case.WorkScope = "调用方篡改内容"
	first.Case.CloseSummary.WorkScope = "调用方篡改摘要"
	first.Timeline[0].Summary = "调用方篡改时间线"

	second, err := service.Archive(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if second.Case.WorkScope != "接续盒检查" || second.Case.CloseSummary.WorkScope != "接续盒检查" || second.Timeline[0].Summary != "个案关闭" {
		t.Fatalf("后续归档读取被首次调用方污染: scope=%q summary_scope=%q timeline=%q", second.Case.WorkScope, second.Case.CloseSummary.WorkScope, second.Timeline[0].Summary)
	}
}
