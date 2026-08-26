package domain

import "time"

type RequirementStatus struct {
	Key      string `json:"key"`
	Complete bool   `json:"complete"`
	Detail   string `json:"detail,omitempty"`
}

type ReadinessProjection struct {
	State          CaseState           `json:"state"`
	Evidence       []RequirementStatus `json:"evidence"`
	Coordination   []RequirementStatus `json:"coordination"`
	Risks          []RequirementStatus `json:"risks"`
	Gate           []RequirementStatus `json:"gate"`
	Closure        []RequirementStatus `json:"closure"`
	Unresolved     []string            `json:"unresolved_deviations"`
	Percent        int                 `json:"percent"`
	EvidenceStatus []EvidenceStatus    `json:"evidence_status,omitempty"`
}

func (c *MaintenanceCase) Readiness(now time.Time) ReadinessProjection {
	p := ReadinessProjection{State: c.State, Unresolved: c.UnresolvedDeviations()}
	p.EvidenceStatus = c.EvidenceStatuses(now)
	evidence := map[string]ReadinessEvidence{}
	for _, item := range c.Evidence {
		evidence[item.Category] = item
	}
	for _, key := range RequiredEvidence {
		item, ok := evidence[key]
		complete := ok && item.Verdict == "accepted" && !item.ValidUntil.Before(now)
		detail := "缺少有效证据"
		if ok && item.ValidUntil.Before(now) {
			detail = "证据已过期"
		} else if complete {
			detail = item.Reference
		}
		p.Evidence = append(p.Evidence, RequirementStatus{Key: key, Complete: complete, Detail: detail})
	}
	acks := map[string]CoordinationAcknowledgement{}
	for _, item := range c.Acknowledgements {
		acks[item.PartyRole] = item
	}
	for _, key := range RequiredParties {
		item, ok := acks[key]
		complete := ok && item.Decision == "confirmed"
		detail := "待确认"
		if ok {
			detail = item.Decision
		}
		p.Coordination = append(p.Coordination, RequirementStatus{Key: key, Complete: complete, Detail: detail})
	}
	risks := map[string]RiskItem{}
	for _, item := range c.RiskItems {
		risks[item.Kind] = item
	}
	for _, key := range RequiredRisks {
		item, ok := risks[key]
		detail := "待评定"
		if ok {
			detail = item.Rating + " / " + item.Control
		}
		p.Risks = append(p.Risks, RequirementStatus{Key: key, Complete: ok && item.Control != "", Detail: detail})
	}
	gate := map[string]bool{"weather_reading": false, "exclusion_zone": false, "communication_test": false, "muster": false}
	if c.Gate != nil {
		gate["weather_reading"] = c.Gate.WeatherReading != ""
		gate["exclusion_zone"] = c.Gate.ExclusionZone
		gate["communication_test"] = c.Gate.CommunicationTest
		gate["muster"] = c.Gate.MusterPass
	}
	for _, key := range []string{"weather_reading", "exclusion_zone", "communication_test", "muster"} {
		p.Gate = append(p.Gate, RequirementStatus{Key: key, Complete: gate[key]})
	}
	closure := map[string]ClosureEvidence{}
	for _, item := range c.ClosureEvidence {
		closure[item.Category] = item
	}
	for _, key := range RequiredClosure {
		item, ok := closure[key]
		p.Closure = append(p.Closure, RequirementStatus{Key: key, Complete: ok, Detail: item.Reference})
	}
	completed, total := 0, 0
	groups := [][]RequirementStatus{p.Evidence, p.Coordination, p.Risks, p.Gate, p.Closure}
	for _, group := range groups {
		for _, item := range group {
			total++
			if item.Complete {
				completed++
			}
		}
	}
	if total > 0 {
		p.Percent = completed * 100 / total
	}
	return p
}
