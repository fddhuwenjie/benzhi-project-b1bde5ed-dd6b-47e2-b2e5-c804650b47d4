package dashboard_snapshot_stale_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cablewindow/internal/domain"
	"cablewindow/internal/httpui"
	"cablewindow/internal/store"
	"cablewindow/internal/workflow"
)

func TestDashboardSnapshotRefreshesAfterCommand(t *testing.T) {
	now := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	repo, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	handler := httpui.New(
		workflow.New(repo, func() time.Time { return now }),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	created := postJSON[domain.CommitResult](t, handler, "/api/cases", map[string]any{
		"request_id": "dashboard-create", "actor": "协调员", "role": "coordinator", "expected_revision": 0,
		"input": map[string]any{
			"cable_segment": "SCS-OLD", "work_scope": "接续盒修复",
			"window_start": now.Add(time.Hour), "window_end": now.Add(4 * time.Hour),
			"coordinator": "协调员", "risk_baseline": "中等风险",
		},
	})
	first := getDashboard(t, handler)
	assertWorkItem(t, first, created.Case.ID, created.Case.Revision, "SCS-OLD")

	revised := postJSON[domain.CommitResult](t, handler, "/api/cases/"+created.Case.ID+"/commands/revise_draft", map[string]any{
		"request_id": "dashboard-revise", "actor": "协调员", "role": "coordinator", "expected_revision": created.Case.Revision,
		"payload": map[string]any{
			"cable_segment": "SCS-NEW", "work_scope": "接续盒修复",
			"window_start": now.Add(time.Hour), "window_end": now.Add(4 * time.Hour),
			"coordinator": "协调员", "risk_baseline": "中等风险",
		},
	})
	second := getDashboard(t, handler)
	assertWorkItem(t, second, revised.Case.ID, revised.Case.Revision, "SCS-NEW")
}

func postJSON[T any](t *testing.T, handler http.Handler, path string, body any) T {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code < 200 || rec.Code >= 300 {
		t.Fatalf("POST %s 返回 %d: %s", path, rec.Code, rec.Body.String())
	}
	var result T
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func getDashboard(t *testing.T, handler http.Handler) workflow.Dashboard {
	t.Helper()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/dashboard", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/dashboard 返回 %d: %s", rec.Code, rec.Body.String())
	}
	var result workflow.Dashboard
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func assertWorkItem(t *testing.T, dashboard workflow.Dashboard, caseID string, revision int64, segment string) {
	t.Helper()
	for _, item := range dashboard.WorkItems {
		if item.CaseID == caseID {
			if item.Revision != revision || item.CableSegment != segment {
				t.Fatalf("仪表盘仍返回旧快照：revision=%d cable_segment=%s，期望 revision=%d cable_segment=%s", item.Revision, item.CableSegment, revision, segment)
			}
			return
		}
	}
	t.Fatalf("仪表盘缺少个案 %s", caseID)
}
