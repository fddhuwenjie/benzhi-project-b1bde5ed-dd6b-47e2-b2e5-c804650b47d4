package domain

import (
	"strings"
	"time"
)

var RequiredEvidence = []string{"vessel", "personnel", "positioning", "repair_equipment", "emergency_supplies"}
var RequiredParties = []string{"maritime_liaison", "cable_owner", "vessel_party"}
var RequiredRisks = []string{"anchor_drag", "weather", "communication_loss", "nearby_facility"}
var RequiredClosure = []string{"cable_restoration", "tool_inventory", "sea_clearance", "stakeholder_receipt"}

func NewCase(id, segment, scope string, start, end time.Time, coordinator, baseline string, now time.Time) (*MaintenanceCase, error) {
	missing := make([]string, 0)
	if strings.TrimSpace(segment) == "" {
		missing = append(missing, "cable_segment")
	}
	if strings.TrimSpace(scope) == "" {
		missing = append(missing, "work_scope")
	}
	if strings.TrimSpace(coordinator) == "" {
		missing = append(missing, "coordinator")
	}
	if strings.TrimSpace(baseline) == "" {
		missing = append(missing, "risk_baseline")
	}
	if start.IsZero() || end.IsZero() || !end.After(start) {
		missing = append(missing, "window")
	}
	if len(missing) > 0 {
		return nil, NewError("invalid_case", "维护个案字段不完整", missing...)
	}
	c := &MaintenanceCase{ID: id, CableSegment: strings.TrimSpace(segment), WorkScope: strings.TrimSpace(scope), WindowStart: start.UTC(), WindowEnd: end.UTC(), Coordinator: strings.TrimSpace(coordinator), RiskBaseline: strings.TrimSpace(baseline), State: StateDraft, Revision: 1, CreatedAt: now.UTC(), UpdatedAt: now.UTC()}
	c.addTimeline("case_created", coordinator, "创建维护个案草稿", now)
	return c, nil
}

func (c *MaintenanceCase) ensureMutable() error {
	if c.State == StateClosed {
		return NewError("case_frozen", "个案已关闭并冻结")
	}
	return nil
}

func (c *MaintenanceCase) ReviseDraft(segment, scope string, start, end time.Time, coordinator, baseline, actor string, now time.Time) error {
	if err := c.ensureMutable(); err != nil {
		return err
	}
	if c.State != StateDraft {
		return NewError("draft_immutable", "个案离开草稿后不可修订", "state")
	}
	segment, scope, coordinator, baseline = strings.TrimSpace(segment), strings.TrimSpace(scope), strings.TrimSpace(coordinator), strings.TrimSpace(baseline)
	var fields []string
	if segment == "" {
		fields = append(fields, "cable_segment")
	}
	if scope == "" {
		fields = append(fields, "work_scope")
	}
	if coordinator == "" {
		fields = append(fields, "coordinator")
	}
	if baseline == "" {
		fields = append(fields, "risk_baseline")
	}
	if start.IsZero() || end.IsZero() || !end.After(start) {
		fields = append(fields, "window")
	}
	if len(fields) > 0 {
		return NewError("invalid_case", "维护个案字段不完整", fields...)
	}
	changes := map[string]string{}
	if c.CableSegment != segment {
		changes["cable_segment"] = c.CableSegment + " -> " + segment
	}
	if c.WorkScope != scope {
		changes["work_scope"] = c.WorkScope + " -> " + scope
	}
	if !c.WindowStart.Equal(start.UTC()) || !c.WindowEnd.Equal(end.UTC()) {
		changes["window"] = c.WindowStart.Format(time.RFC3339) + "/" + c.WindowEnd.Format(time.RFC3339) + " -> " + start.UTC().Format(time.RFC3339) + "/" + end.UTC().Format(time.RFC3339)
	}
	if c.Coordinator != coordinator {
		changes["coordinator"] = c.Coordinator + " -> " + coordinator
	}
	if c.RiskBaseline != baseline {
		changes["risk_baseline"] = c.RiskBaseline + " -> " + baseline
	}
	c.CableSegment, c.WorkScope, c.WindowStart, c.WindowEnd, c.Coordinator, c.RiskBaseline = segment, scope, start.UTC(), end.UTC(), coordinator, baseline
	if len(changes) > 0 {
		c.touch(actor, "draft_revised", "修订草稿字段", now)
		c.DraftChanges = append(c.DraftChanges, DraftChange{Revision: c.Revision, Actor: actor, At: now.UTC(), Fields: changes})
	}
	return nil
}

func (c *MaintenanceCase) expectState(states ...CaseState) error {
	for _, state := range states {
		if c.State == state {
			return nil
		}
	}
	return NewError("invalid_state", "当前状态不允许执行该操作", string(c.State))
}

func (c *MaintenanceCase) touch(actor, kind, summary string, now time.Time) {
	c.Revision++
	c.UpdatedAt = now.UTC()
	c.addTimeline(kind, actor, summary, now)
}

func (c *MaintenanceCase) addTimeline(kind, actor, summary string, now time.Time) {
	c.Timeline = append(c.Timeline, TimelineEntry{Type: kind, Actor: actor, Summary: summary, At: now.UTC(), Revision: c.Revision})
}

func missingFrom(required []string, present map[string]bool) []string {
	var missing []string
	for _, item := range required {
		if !present[item] {
			missing = append(missing, item)
		}
	}
	return missing
}
