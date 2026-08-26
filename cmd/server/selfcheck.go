package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"cablewindow/internal/domain"
)

type checkResponse struct {
	Case       *domain.MaintenanceCase `json:"case"`
	Idempotent bool                    `json:"idempotent"`
}

func runSelfCheck(ctx context.Context, cfg config, logger *slog.Logger) error {
	dir, err := os.MkdirTemp("", "cable-window-self-check-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	now := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	handler, err := buildHandler(dir, logger, func() time.Time { return now })
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", cfg.addr)
	if err != nil {
		return err
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 2 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second}
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	defer func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	base := "http://" + listener.Addr().String()
	client := &http.Client{Timeout: 4 * time.Second}
	if err := waitReady(ctx, client, base); err != nil {
		return err
	}
	created, err := checkPost(ctx, client, base+"/api/cases", map[string]any{"request_id": "self-create", "actor": "自检协调员", "role": "coordinator", "expected_revision": 0, "input": map[string]any{"cable_segment": "SCS-SELF/KP128", "work_scope": "海缆接续盒修复", "window_start": now.Add(-time.Hour), "window_end": now.Add(12 * time.Hour), "coordinator": "自检协调员", "risk_baseline": "中等海况基线"}})
	if err != nil {
		return err
	}
	id, revision := created.Case.ID, created.Case.Revision
	command := func(requestID, action, actor, role string, payload any) (*domain.MaintenanceCase, error) {
		result, err := checkPost(ctx, client, fmt.Sprintf("%s/api/cases/%s/commands/%s", base, id, action), map[string]any{"request_id": requestID, "actor": actor, "role": role, "expected_revision": revision, "payload": payload})
		if err == nil {
			revision = result.Case.Revision
		}
		return result.Case, err
	}
	for i, category := range domain.RequiredEvidence {
		if _, err := command(fmt.Sprintf("self-evidence-%d", i), "put_evidence", "自检协调员", "coordinator", map[string]any{"category": category, "subject": "核验证据-" + category, "reference": "REF-" + category, "valid_until": now.Add(30 * 24 * time.Hour)}); err != nil {
			return err
		}
	}
	if _, err = command("self-coordinate", "submit_coordination", "自检协调员", "coordinator", map[string]any{}); err != nil {
		return err
	}
	for i, role := range domain.RequiredParties {
		if _, err = command(fmt.Sprintf("self-ack-%d", i), "acknowledge", "自检席位", role, map[string]any{"party_role": role, "decision": "confirmed", "comment": "确认"}); err != nil {
			return err
		}
	}
	risks := make([]map[string]any, 0)
	for _, kind := range domain.RequiredRisks {
		risks = append(risks, map[string]any{"kind": kind, "rating": "medium", "control": "专项控制"})
	}
	if _, err = command("self-risk", "review_risk", "自检审查员", "safety_reviewer", map[string]any{"decision": "approve", "reason": "", "items": risks}); err != nil {
		return err
	}
	if _, err = command("self-gate", "activate", "自检现场负责人", "site_lead", map[string]any{"weather_reading": "风力4级，浪高1.2米", "exclusion_zone": true, "communication_test": true, "muster_pass": true}); err != nil {
		return err
	}
	if _, err = command("self-progress", "add_progress", "自检现场负责人", "site_lead", map[string]any{"text": "完成海缆定位"}); err != nil {
		return err
	}
	c, err := command("self-deviation", "add_deviation", "自检现场负责人", "site_lead", map[string]any{"severity": "major", "description": "定位短时漂移", "immediate_action": "暂停绞车"})
	if err != nil {
		return err
	}
	deviationID := c.Deviations[len(c.Deviations)-1].ID
	if _, err = command("self-resolve", "resolve_deviation", "自检审查员", "safety_reviewer", map[string]any{"id": deviationID, "corrective_action": "校准定位并完成复测", "review_verdict": "approved"}); err != nil {
		return err
	}
	for i, category := range domain.RequiredClosure {
		if _, err = command(fmt.Sprintf("self-close-evidence-%d", i), "submit_closure_evidence", "自检现场负责人", "site_lead", map[string]any{"category": category, "reference": "CLOSE-" + category}); err != nil {
			return err
		}
	}
	closed, err := command("self-close", "close", "自检现场负责人", "site_lead", map[string]any{})
	if err != nil {
		return err
	}
	if closed.State != domain.StateClosed || closed.CloseSummary == nil || closed.CloseSummary.Digest == "" {
		return errors.New("关闭摘要断言失败")
	}
	var audit struct {
		Events []domain.AuditEvent `json:"events"`
	}
	if err := checkGet(ctx, client, base+"/api/cases/"+id+"/audit?limit=100", &audit); err != nil {
		return err
	}
	if len(audit.Events) != int(closed.Revision) {
		return fmt.Errorf("审计事件数 %d 与修订 %d 不一致", len(audit.Events), closed.Revision)
	}
	var verification struct {
		Verification domain.AuditVerification `json:"verification"`
	}
	if err := checkGet(ctx, client, base+"/api/cases/"+id+"/audit/verify", &verification); err != nil {
		return err
	}
	if !verification.Verification.Valid || verification.Verification.EventCount != len(audit.Events) {
		return errors.New("审计链验证断言失败")
	}
	var auditSummary struct {
		AuditSummary domain.AuditSummary `json:"audit_summary"`
	}
	if err := checkGet(ctx, client, base+"/api/cases/"+id+"/audit/summary", &auditSummary); err != nil {
		return err
	}
	if auditSummary.AuditSummary.Digest == "" || auditSummary.AuditSummary.FinalRevision != closed.Revision {
		return errors.New("审计摘要断言失败")
	}
	var summary map[string]any
	if err := checkGet(ctx, client, base+"/api/cases/"+id+"/summary", &summary); err != nil {
		return err
	}
	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	default:
	}
	return nil
}

func waitReady(ctx context.Context, client *http.Client, base string) error {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		var out map[string]any
		if checkGet(ctx, client, base+"/readyz", &out) == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func checkPost(ctx context.Context, client *http.Client, url string, body any) (checkResponse, error) {
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return checkResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	var result checkResponse
	err = checkDo(client, req, &result)
	return result, err
}

func checkGet(ctx context.Context, client *http.Client, url string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	return checkDo(client, req, target)
}

func checkDo(client *http.Client, req *http.Request, target any) error {
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("%s %s 返回 %d: %s", req.Method, req.URL.Path, response.StatusCode, string(body))
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("解析自检响应: %w", err)
	}
	return nil
}
