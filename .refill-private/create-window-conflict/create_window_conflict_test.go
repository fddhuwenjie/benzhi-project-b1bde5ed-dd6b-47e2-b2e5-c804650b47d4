package create_window_conflict

import (
	"context"
	"testing"
	"time"

	"cablewindow/internal/domain"
	"cablewindow/internal/store"
	"cablewindow/internal/workflow"
)

func TestCreateRejectsOverlapWithOpenCase(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	repo, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := workflow.New(repo, func() time.Time { return now })
	input := workflow.CreateInput{CableSegment: "SEG-1", WorkScope: "维护", WindowStart: now.Add(time.Hour), WindowEnd: now.Add(3 * time.Hour), Coordinator: "协调员", RiskBaseline: "基线"}
	first, err := service.Create(ctx, workflow.Envelope{RequestID: "create-first", Actor: "协调员", Role: "coordinator"}, input)
	if err != nil {
		t.Fatal(err)
	}
	input.WindowStart = now.Add(2 * time.Hour)
	input.WindowEnd = now.Add(4 * time.Hour)
	_, err = service.Create(ctx, workflow.Envelope{RequestID: "create-second", Actor: "协调员", Role: "coordinator"}, input)
	if !domain.IsCode(err, "window_conflict") {
		t.Fatalf("与未关闭个案 %s 交叠的建档应返回 window_conflict，实际返回: %v", first.Case.ID, err)
	}
}
