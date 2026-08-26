package domain

import "time"

type ActionProjection struct {
	Allowed  []string `json:"allowed"`
	Blockers []string `json:"blockers"`
}

func (c *MaintenanceCase) Actions(now time.Time) ActionProjection {
	p := ActionProjection{}
	switch c.State {
	case StateDraft:
		p.Allowed = []string{"put_evidence", "revise_draft"}
		p.Blockers = c.EvidenceBlockers(now)
		if len(p.Blockers) == 0 {
			p.Allowed = append(p.Allowed, "submit_coordination")
		}
	case StateCoordination:
		p.Allowed = []string{"acknowledge", "submit_remediation", "review_remediation"}
		p.Blockers = c.CoordinationBlockers()
	case StateReview:
		p.Allowed = []string{"review_risk", "submit_risk_control", "verify_risk_control"}
	case StateAuthorization:
		p.Allowed = []string{"activate"}
	case StateActive:
		p.Allowed = []string{"add_progress", "add_deviation", "submit_closure_evidence"}
	case StatePaused:
		p.Allowed = []string{"resolve_deviation"}
		p.Blockers = c.UnresolvedDeviations()
	case StatePendingClose:
		p.Allowed = []string{"submit_closure_evidence"}
		present := map[string]bool{}
		for _, e := range c.ClosureEvidence {
			present[e.Category] = true
		}
		p.Blockers = append(missingFrom(RequiredClosure, present), c.UnresolvedDeviations()...)
		if len(p.Blockers) == 0 {
			p.Allowed = append(p.Allowed, "close")
		}
	}
	return p
}
