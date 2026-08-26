package domain

import (
	"testing"
	"time"
)

func TestLifecycleBlocksMissingEvidenceAndFreezesClose(t *testing.T) {
	now := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	c, err := NewCase("case-test", "SCS-04", "接续修复", now.Add(-time.Hour), now.Add(10*time.Hour), "协调员", "中风险", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.SubmitForCoordination("协调员", now); !IsCode(err, "blocked") {
		t.Fatalf("缺少证据应被阻断，得到 %v", err)
	}
	for _, category := range RequiredEvidence {
		err := c.PutEvidence(ReadinessEvidence{ID: category, Category: category, Subject: category, Reference: "REF", ValidUntil: now.Add(time.Hour)}, "协调员", now)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := c.SubmitForCoordination("协调员", now); err != nil {
		t.Fatal(err)
	}
	for _, party := range RequiredParties {
		if err := c.Acknowledge(CoordinationAcknowledgement{ID: party, PartyRole: party, Decision: "confirmed"}, party, now); err != nil {
			t.Fatal(err)
		}
	}
	items := make([]RiskItem, 0, len(RequiredRisks))
	for _, kind := range RequiredRisks {
		items = append(items, RiskItem{Kind: kind, Rating: "medium", Control: "控制"})
	}
	if err := c.ReviewRisks(items, "approve", "", "审查员", now); err != nil {
		t.Fatal(err)
	}
	if err := c.Activate(StartGate{WeatherReading: "良好", ExclusionZone: true, CommunicationTest: true, MusterPass: true}, "现场负责人", now); err != nil {
		t.Fatal(err)
	}
	if err := c.AddDeviation(OperationalDeviation{ID: "dev-1", Severity: "critical", Description: "漂移", ImmediateAction: "暂停"}, "现场负责人", now); err != nil {
		t.Fatal(err)
	}
	if c.State != StatePaused {
		t.Fatalf("严重偏差后状态为 %s", c.State)
	}
	if err := c.ResolveDeviation("dev-1", "重新校准", "approved", "审查员", now); err != nil {
		t.Fatal(err)
	}
	for _, category := range RequiredClosure {
		if err := c.SubmitClosureEvidence(ClosureEvidence{Category: category, Reference: "CLOSE"}, "现场负责人", now); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.Close("现场负责人", now); err != nil {
		t.Fatal(err)
	}
	if c.CloseSummary == nil || len(c.CloseSummary.Digest) != 64 {
		t.Fatal("关闭摘要缺少确定性摘要")
	}
	if err := c.AddProgress("late", "归档后修改", "现场负责人", now); !IsCode(err, "invalid_state") {
		t.Fatalf("归档后修改未拒绝：%v", err)
	}
}

func TestRiskReturnKeepsReason(t *testing.T) {
	now := time.Now().UTC()
	c, _ := NewCase("case-risk", "S", "范围", now, now.Add(time.Hour), "协调", "风险", now)
	c.State = StateReview
	if err := c.ReviewRisks(nil, "return", "补充拖锚控制", "审查员", now); err != nil {
		t.Fatal(err)
	}
	if c.State != StateDraft || len(c.RiskReturns) != 1 || c.RiskReturns[0].Reason == "" {
		t.Fatal("退回原因或状态未保留")
	}
}
