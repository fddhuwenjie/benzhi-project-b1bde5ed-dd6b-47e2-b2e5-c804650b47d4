package domain

import (
	"strings"
	"time"
)

func (c *MaintenanceCase) PutEvidence(e ReadinessEvidence, actor string, now time.Time) error {
	if err := c.ensureMutable(); err != nil {
		return err
	}
	if err := c.expectState(StateDraft); err != nil {
		return err
	}
	if !contains(RequiredEvidence, e.Category) {
		return NewError("invalid_evidence_category", "未知的证据类别", "category")
	}
	if strings.TrimSpace(e.Subject) == "" || strings.TrimSpace(e.Reference) == "" {
		return NewError("invalid_evidence", "证据主题和引用不能为空", "subject", "reference")
	}
	if e.ValidUntil.IsZero() || e.ValidUntil.Before(now) {
		return NewError("evidence_expired", "证据已经过期", "valid_until")
	}
	e.CaseID, e.VerifiedBy, e.Verdict = c.ID, actor, "accepted"
	verified := now.UTC()
	e.VerifiedAt = &verified
	for i := range c.Evidence {
		if c.Evidence[i].Category == e.Category {
			old := c.Evidence[i]
			replaced := now.UTC()
			old.ReplacedAt, old.ReplacedBy = &replaced, e.Reference
			c.EvidenceHistory = append(c.EvidenceHistory, old)
			c.Evidence[i] = e
			c.touch(actor, "evidence_updated", e.Category, now)
			return nil
		}
	}
	c.Evidence = append(c.Evidence, e)
	c.touch(actor, "evidence_added", e.Category, now)
	return nil
}

func (c *MaintenanceCase) EvidenceStatuses(now time.Time) []EvidenceStatus {
	current := map[string]ReadinessEvidence{}
	for _, e := range c.Evidence {
		current[e.Category] = e
	}
	result := make([]EvidenceStatus, 0, len(RequiredEvidence))
	for _, category := range RequiredEvidence {
		e, ok := current[category]
		status := "missing"
		if ok {
			switch {
			case e.ValidUntil.Before(now):
				status = "expired"
			case e.ValidUntil.Before(c.WindowEnd):
				status = "cannot_cover_window"
			case e.ValidUntil.Sub(now) <= 7*24*time.Hour:
				status = "expiring"
			default:
				status = "valid"
			}
		}
		result = append(result, EvidenceStatus{Category: category, Status: status, ValidUntil: e.ValidUntil, Reference: e.Reference})
	}
	return result
}

func (c *MaintenanceCase) EvidenceBlockers(now time.Time) []string {
	present := map[string]bool{}
	for _, e := range c.Evidence {
		if e.Verdict == "accepted" && !e.ValidUntil.Before(now) {
			present[e.Category] = true
		}
	}
	return missingFrom(RequiredEvidence, present)
}

func (c *MaintenanceCase) CoverageBlockers(now time.Time) []string {
	var blockers []string
	for _, st := range c.EvidenceStatuses(now) {
		if st.Status == "missing" || st.Status == "expired" || st.Status == "cannot_cover_window" {
			blockers = append(blockers, st.Category)
		}
	}
	return blockers
}

func (c *MaintenanceCase) SubmitForCoordination(actor string, now time.Time) error {
	if err := c.expectState(StateDraft); err != nil {
		return err
	}
	if blockers := c.EvidenceBlockers(now); len(blockers) > 0 {
		return Blocked(blockers)
	}
	c.State = StateCoordination
	c.touch(actor, "coordination_started", "准入证据齐备，进入协调", now)
	return nil
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
