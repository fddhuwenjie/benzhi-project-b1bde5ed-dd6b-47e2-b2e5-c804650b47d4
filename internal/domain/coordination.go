package domain

import (
	"fmt"
	"strings"
	"time"
)

func (c *MaintenanceCase) Acknowledge(a CoordinationAcknowledgement, actor string, now time.Time) error {
	if err := c.expectState(StateCoordination); err != nil {
		return err
	}
	if !contains(RequiredParties, a.PartyRole) {
		return NewError("invalid_party", "未知的协调席位", "party_role")
	}
	if a.Decision != "confirmed" && a.Decision != "rejected" {
		return NewError("invalid_decision", "协调决定必须为 confirmed 或 rejected", "decision")
	}
	if a.Decision == "rejected" && strings.TrimSpace(a.Comment) == "" {
		return NewError("comment_required", "拒绝时必须填写意见", "comment")
	}
	a.CaseID, a.Actor, a.DecidedAt = c.ID, actor, now.UTC()
	round := 1
	for _, r := range c.CoordinationRounds {
		if r.PartyRole == a.PartyRole && r.Round >= round {
			round = r.Round + 1
		}
	}
	c.CoordinationRounds = append(c.CoordinationRounds, CoordinationRound{Round: round, PartyRole: a.PartyRole, Decision: a.Decision, Comment: strings.TrimSpace(a.Comment), Actor: actor, At: now.UTC()})
	if a.Decision == "rejected" {
		c.Remediations = append(c.Remediations, CoordinationRemediation{ID: "rem-" + a.PartyRole + "-" + fmt.Sprint(round), PartyRole: a.PartyRole, Round: round, Objection: strings.TrimSpace(a.Comment), Status: "pending_response", Actor: actor, At: now.UTC()})
	}
	for i := range c.Acknowledgements {
		if c.Acknowledgements[i].PartyRole == a.PartyRole {
			c.Acknowledgements = append(c.Acknowledgements, a)
			c.finishAcknowledgement(actor, now)
			return nil
		}
	}
	c.Acknowledgements = append(c.Acknowledgements, a)
	c.finishAcknowledgement(actor, now)
	return nil
}

func (c *MaintenanceCase) SubmitRemediation(party, response, reference, actor string, now time.Time) error {
	if err := c.expectState(StateCoordination); err != nil {
		return err
	}
	for i := range c.Remediations {
		r := &c.Remediations[i]
		if r.PartyRole == party && r.Status == "pending_response" {
			if strings.TrimSpace(response) == "" || strings.TrimSpace(reference) == "" {
				return NewError("remediation_required", "整改说明和证据引用不能为空", "response", "evidence_reference")
			}
			r.Response, r.EvidenceReference, r.Status, r.Actor, r.At = strings.TrimSpace(response), strings.TrimSpace(reference), "pending_review", actor, now.UTC()
			c.touch(actor, "coordination_remediation_submitted", party, now)
			return nil
		}
	}
	return NewError("remediation_not_found", "未找到待整改协调异议", "party_role")
}

func (c *MaintenanceCase) ReviewRemediation(party, decision, comment, actor string, now time.Time) error {
	if err := c.expectState(StateCoordination); err != nil {
		return err
	}
	for _, pending := range c.Remediations {
		if pending.Status == "pending_review" && pending.PartyRole != party {
			return NewError("party_role_mismatch", "仅提出异议的席位可复核整改", "party_role")
		}
	}
	for i := range c.Remediations {
		r := &c.Remediations[i]
		if r.PartyRole == party && r.Status == "pending_review" {
			if decision != "confirmed" && decision != "rejected" {
				return NewError("invalid_decision", "复核决定无效", "decision")
			}
			if decision == "rejected" && strings.TrimSpace(comment) == "" {
				return NewError("comment_required", "再次拒绝时必须填写意见", "comment")
			}
			r.Status, r.Actor, r.At = decision, actor, now.UTC()
			if decision == "rejected" {
				nextRound := r.Round + 1
				c.Remediations = append(c.Remediations, CoordinationRemediation{ID: "rem-" + party + "-" + fmt.Sprint(nextRound), PartyRole: party, Round: nextRound, Objection: strings.TrimSpace(comment), Status: "pending_response", Actor: actor, At: now.UTC()})
			}
			for j := range c.Acknowledgements {
				if c.Acknowledgements[j].PartyRole == party {
					c.Acknowledgements[j].Decision = decision
					c.Acknowledgements[j].Comment = comment
				}
			}
			c.touch(actor, "coordination_remediation_reviewed", party, now)
			c.finishAcknowledgement(actor, now)
			return nil
		}
	}
	return NewError("remediation_not_found", "未找到待复核整改", "party_role")
}

func (c *MaintenanceCase) finishAcknowledgement(actor string, now time.Time) {
	c.touch(actor, "coordination_recorded", "记录相关方协调意见", now)
	present := map[string]bool{}
	for _, a := range c.Acknowledgements {
		if a.Decision == "confirmed" {
			present[a.PartyRole] = true
		}
	}
	if len(missingFrom(RequiredParties, present)) == 0 {
		c.State = StateReview
		c.addTimeline("coordination_completed", actor, "全部必需席位确认", now)
	}
}

func (c *MaintenanceCase) CoordinationBlockers() []string {
	present := map[string]bool{}
	for _, a := range c.Acknowledgements {
		if a.Decision == "confirmed" {
			present[a.PartyRole] = true
		}
	}
	blockers := missingFrom(RequiredParties, present)
	for _, r := range c.Remediations {
		if r.Status == "pending_response" || r.Status == "pending_review" {
			blockers = append(blockers, r.PartyRole+":"+r.Status)
		}
	}
	return blockers
}
