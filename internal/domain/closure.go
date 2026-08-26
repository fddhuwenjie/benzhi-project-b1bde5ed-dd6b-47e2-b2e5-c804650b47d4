package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type ClosurePrecheck struct {
	Ready      bool         `json:"ready"`
	Missing    []string     `json:"missing"`
	Unresolved []string     `json:"unresolved_deviations"`
	Blockers   []string     `json:"blockers"`
	Revision   int64        `json:"revision"`
	Summary    CloseSummary `json:"summary"`
}

func (c *MaintenanceCase) ClosurePrecheck(now time.Time) ClosurePrecheck {
	p := ClosurePrecheck{Revision: c.Revision}
	p.Missing = missingFrom(RequiredClosure, func() map[string]bool {
		m := map[string]bool{}
		for _, e := range c.ClosureEvidence {
			m[e.Category] = true
		}
		return m
	}())
	p.Unresolved = c.UnresolvedDeviations()
	p.Blockers = append(append([]string{}, p.Missing...), p.Unresolved...)
	p.Ready = c.State == StatePendingClose && len(p.Blockers) == 0
	if c.CloseSummary != nil {
		p.Summary = *c.CloseSummary
	}
	return p
}

func (c *MaintenanceCase) SubmitClosureEvidence(e ClosureEvidence, actor string, now time.Time) error {
	if err := c.expectState(StateActive, StatePendingClose); err != nil {
		return err
	}
	if !contains(RequiredClosure, e.Category) || strings.TrimSpace(e.Reference) == "" {
		return NewError("invalid_closure_evidence", "关闭证据无效", "category", "reference")
	}
	e.Actor, e.SubmittedAt = actor, now.UTC()
	for i := range c.ClosureEvidence {
		if c.ClosureEvidence[i].Category == e.Category {
			c.ClosureEvidence[i] = e
			c.State = StatePendingClose
			c.touch(actor, "closure_evidence_updated", e.Category, now)
			return nil
		}
	}
	c.ClosureEvidence = append(c.ClosureEvidence, e)
	c.State = StatePendingClose
	c.touch(actor, "closure_evidence_added", e.Category, now)
	return nil
}

func (c *MaintenanceCase) Close(actor string, now time.Time) error {
	if err := c.expectState(StatePendingClose); err != nil {
		return err
	}
	present := map[string]bool{}
	for _, e := range c.ClosureEvidence {
		present[e.Category] = true
	}
	blockers := missingFrom(RequiredClosure, present)
	blockers = append(blockers, c.UnresolvedDeviations()...)
	if len(blockers) > 0 {
		return Blocked(blockers)
	}
	closed := now.UTC()
	c.State, c.ClosedAt = StateClosed, &closed
	c.touch(actor, "case_closed", "恢复证据齐备，个案冻结归档", now)
	summary := c.buildCloseSummary(now)
	c.CloseSummary = &summary
	return nil
}

func (c *MaintenanceCase) BuildArchive(audit AuditSummary) error {
	if c.State != StateClosed || c.CloseSummary == nil {
		return NewError("archive_unavailable", "仅已关闭个案可生成归档包")
	}
	copyCase := *c
	copyCase.Archive = nil
	payload := struct {
		Version         string            `json:"version"`
		Case            MaintenanceCase   `json:"case"`
		Timeline        []TimelineEntry   `json:"timeline"`
		ClosureEvidence []ClosureEvidence `json:"closure_evidence"`
		CloseSummary    CloseSummary      `json:"close_summary"`
		AuditSummary    AuditSummary      `json:"audit_summary"`
	}{"v1", copyCase, copyCase.Timeline, copyCase.ClosureEvidence, *c.CloseSummary, audit}
	b, _ := json.Marshal(payload)
	sum := sha256.Sum256(b)
	c.Archive = &ArchivePackage{Version: "v1", Case: copyCase, Timeline: copyCase.Timeline, ClosureEvidence: copyCase.ClosureEvidence, CloseSummary: *c.CloseSummary, AuditSummary: audit, Digest: hex.EncodeToString(sum[:])}
	return nil
}

func (c *MaintenanceCase) buildCloseSummary(now time.Time) CloseSummary {
	window := c.WindowStart.UTC().Format(time.RFC3339) + "/" + c.WindowEnd.UTC().Format(time.RFC3339)
	canonical := fmt.Sprintf("case=%s\nsegment=%s\nscope=%s\nwindow=%s\ncoordinator=%s\nevidence=%d\nprogress=%d\ndeviations=%d\nclosed=%s", c.ID, c.CableSegment, c.WorkScope, window, c.Coordinator, len(c.Evidence), len(c.Progress), len(c.Deviations), now.UTC().Format(time.RFC3339Nano))
	sum := sha256.Sum256([]byte(canonical))
	return CloseSummary{CaseID: c.ID, CableSegment: c.CableSegment, WorkScope: c.WorkScope, Window: window, Coordinator: c.Coordinator, EvidenceCount: len(c.Evidence), ProgressCount: len(c.Progress), DeviationCount: len(c.Deviations), ClosedAt: now.UTC(), Digest: hex.EncodeToString(sum[:])}
}

func (c *MaintenanceCase) Canonical() string {
	parts := []string{c.ID, c.CableSegment, c.WorkScope, c.WindowStart.UTC().Format(time.RFC3339Nano), c.WindowEnd.UTC().Format(time.RFC3339Nano), c.Coordinator, string(c.State), fmt.Sprint(c.Revision)}
	sort.Strings(parts[1:3])
	return strings.Join(parts, "|")
}
