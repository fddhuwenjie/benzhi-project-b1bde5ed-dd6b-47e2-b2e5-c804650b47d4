package domain

import (
	"strings"
	"time"
)

func (c *MaintenanceCase) ReviewRisks(items []RiskItem, decision, reason, actor string, now time.Time) error {
	if err := c.expectState(StateReview); err != nil {
		return err
	}
	if decision == "return" {
		if strings.TrimSpace(reason) == "" {
			return NewError("reason_required", "退回时必须说明原因", "reason")
		}
		c.RiskReturns = append(c.RiskReturns, RiskReturn{Reason: reason, Reviewer: actor, At: now.UTC()})
		c.State = StateDraft
		c.touch(actor, "risk_returned", reason, now)
		return nil
	}
	if decision == "prepare" {
		if len(items) == 0 {
			return NewError("invalid_risk", "风险项不能为空", "items")
		}
		for _, item := range items {
			if !contains(RequiredRisks, item.Kind) || !contains([]string{"low", "medium", "high"}, item.Rating) || strings.TrimSpace(item.Control) == "" {
				return NewError("invalid_risk", "风险等级或控制措施无效", item.Kind)
			}
		}
		c.RiskItems = items
		c.touch(actor, "risk_assessed", "记录风险控制要求", now)
		return nil
	}
	if decision != "approve" {
		return NewError("invalid_decision", "风险审查决定无效", "decision")
	}
	present := map[string]bool{}
	clean := make([]RiskItem, 0, len(items))
	for _, item := range items {
		if !contains(RequiredRisks, item.Kind) {
			return NewError("invalid_risk", "未知风险项", item.Kind)
		}
		if !contains([]string{"low", "medium", "high"}, item.Rating) || strings.TrimSpace(item.Control) == "" {
			return NewError("invalid_risk", "风险等级或控制措施无效", item.Kind)
		}
		if item.Rating == "high" && (strings.TrimSpace(item.ControlOwner) == "" || item.DueAt.IsZero() || strings.TrimSpace(item.ControlEvidence) == "") {
			return NewError("risk_control_incomplete", "高风险控制措施缺少责任人、期限或证据", item.Kind)
		}
		item.Reviewer, item.ReviewedAt = actor, now.UTC()
		for _, prior := range c.RiskItems {
			if prior.Kind == item.Kind {
				if item.ControlEvidence == "" {
					item.ControlEvidence = prior.ControlEvidence
				}
				if item.ControlOwner == "" {
					item.ControlOwner = prior.ControlOwner
				}
				if item.DueAt.IsZero() {
					item.DueAt = prior.DueAt
				}
				if item.Verification == nil {
					item.Verification = prior.Verification
				}
				if item.ControlStatus == "" {
					item.ControlStatus = prior.ControlStatus
				}
			}
		}
		if item.Rating == "high" {
			if item.ControlStatus == "" {
				item.ControlStatus = "pending_verification"
			}
		} else {
			item.ControlStatus = "not_required"
		}
		present[item.Kind] = true
		clean = append(clean, item)
	}
	if missing := missingFrom(RequiredRisks, present); len(missing) > 0 {
		return Blocked(missing)
	}
	c.RiskItems = clean
	for _, item := range clean {
		if item.Rating == "high" && item.ControlStatus != "verified" {
			return Blocked([]string{"risk_control:" + item.Kind})
		}
	}
	c.State = StateAuthorization
	c.touch(actor, "risk_approved", "风险与控制措施审查通过", now)
	return nil
}

func (c *MaintenanceCase) SubmitRiskControl(kind, evidence, actor string, now time.Time) error {
	if err := c.expectState(StateReview); err != nil {
		return err
	}
	for i := range c.RiskItems {
		if c.RiskItems[i].Kind != kind {
			continue
		}
		if c.RiskItems[i].Rating != "high" {
			return NewError("risk_control_not_required", "该风险无需高风险控制核验", kind)
		}
		if strings.TrimSpace(c.RiskItems[i].ControlOwner) == "" || c.RiskItems[i].DueAt.IsZero() {
			return NewError("risk_control_incomplete", "高风险控制缺少责任人或期限", kind)
		}
		if now.After(c.RiskItems[i].DueAt) {
			return NewError("risk_control_overdue", "高风险控制已超过完成期限", kind)
		}
		if strings.TrimSpace(evidence) == "" {
			return NewError("evidence_required", "控制落实证据不能为空", "evidence_reference")
		}
		c.RiskItems[i].ControlEvidence, c.RiskItems[i].ControlStatus = strings.TrimSpace(evidence), "pending_verification"
		c.touch(actor, "risk_control_submitted", kind, now)
		return nil
	}
	return NewError("invalid_risk", "未找到风险项", kind)
}

func (c *MaintenanceCase) VerifyRiskControl(kind, verdict, reason, actor string, now time.Time) error {
	if err := c.expectState(StateReview); err != nil {
		return err
	}
	for i := range c.RiskItems {
		item := &c.RiskItems[i]
		if item.Kind != kind {
			continue
		}
		if item.Rating != "high" {
			return NewError("risk_control_not_required", "该风险无需高风险控制核验", kind)
		}
		if item.ControlOwner == actor {
			return NewError("role_separation", "控制落实人与核验人不得为同一人", kind)
		}
		if item.ControlStatus != "pending_verification" {
			return NewError("risk_control_pending", "风险控制尚未提交落实证据", kind)
		}
		if verdict != "approved" && verdict != "returned" {
			return NewError("invalid_verdict", "核验结论无效", "verdict")
		}
		if verdict == "returned" && strings.TrimSpace(reason) == "" {
			return NewError("reason_required", "退回必须填写原因", "reason")
		}
		if item.Verification != nil {
			item.VerificationHistory = append(item.VerificationHistory, *item.Verification)
		}
		item.Verification = &RiskVerification{Actor: actor, Verdict: verdict, Reason: strings.TrimSpace(reason), At: now.UTC()}
		if verdict == "approved" {
			item.ControlStatus = "verified"
		} else {
			item.ControlStatus = "returned"
		}
		c.touch(actor, "risk_control_verified", kind, now)
		return nil
	}
	return NewError("invalid_risk", "未找到风险项", kind)
}
