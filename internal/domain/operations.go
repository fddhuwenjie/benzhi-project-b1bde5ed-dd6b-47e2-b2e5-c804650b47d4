package domain

import (
	"strings"
	"time"
)

func (c *MaintenanceCase) Activate(g StartGate, actor string, now time.Time) error {
	if err := c.expectState(StateAuthorization); err != nil {
		return err
	}
	var blockers []string
	if strings.TrimSpace(g.WeatherReading) == "" {
		blockers = append(blockers, "weather_reading")
	}
	if !g.ExclusionZone {
		blockers = append(blockers, "exclusion_zone")
	}
	if !g.CommunicationTest {
		blockers = append(blockers, "communication_test")
	}
	if !g.MusterPass {
		blockers = append(blockers, "muster")
	}
	if now.Before(c.WindowStart.Add(-24*time.Hour)) || now.After(c.WindowEnd) {
		blockers = append(blockers, "window_boundary")
	}
	if len(g.Checks) > 0 {
		canonical := func(kind string) string {
			switch kind {
			case "weather_reading":
				return "weather"
			case "communication_test":
				return "communication"
			case "personnel_muster", "muster_pass":
				return "muster"
			default:
				return kind
			}
		}
		merged := map[string]GateCheck{}
		if c.Gate != nil {
			for _, old := range c.Gate.Checks {
				old.Kind = canonical(old.Kind)
				merged[old.Kind] = old
			}
		}
		for _, check := range g.Checks {
			check.Kind = canonical(check.Kind)
			merged[check.Kind] = check
		}
		g.Checks = g.Checks[:0]
		for _, kind := range []string{"weather", "exclusion_zone", "communication", "muster"} {
			if check, ok := merged[kind]; ok {
				g.Checks = append(g.Checks, check)
			}
		}
		seen := map[string]bool{}
		for i := range g.Checks {
			check := &g.Checks[i]
			if check.CheckedAt.IsZero() {
				check.CheckedAt = now
			}
			if check.CheckedAt.After(now) {
				blockers = append(blockers, check.Kind+":future")
			}
			if now.Sub(check.CheckedAt) > 2*time.Hour {
				blockers = append(blockers, check.Kind+":stale")
			}
			seen[check.Kind] = true
			if !check.Passed {
				blockers = append(blockers, check.Kind)
			}
		}
		for _, kind := range []string{"weather", "exclusion_zone", "communication", "muster"} {
			if !seen[kind] {
				blockers = append(blockers, kind)
			}
		}
	}
	attempt := GateAttempt{At: now.UTC(), Actor: actor, Checks: append([]GateCheck(nil), g.Checks...), Passed: len(blockers) == 0, Failures: append([]string(nil), blockers...)}
	c.GateAttempts = append(c.GateAttempts, attempt)
	if len(blockers) > 0 {
		g.Actor, g.CheckedAt = actor, now.UTC()
		c.Gate = &g
		c.touch(actor, "gate_attempt_failed", strings.Join(blockers, ","), now)
		return Blocked(blockers)
	}
	g.Actor, g.CheckedAt = actor, now.UTC()
	c.Gate = &g
	c.State = StateActive
	c.touch(actor, "window_activated", "开工门禁全部通过", now)
	return nil
}

func (c *MaintenanceCase) AddProgress(id, text, actor string, now time.Time) error {
	if err := c.expectState(StateActive); err != nil {
		return err
	}
	if strings.TrimSpace(text) == "" {
		return NewError("text_required", "进展内容不能为空", "text")
	}
	c.Progress = append(c.Progress, ProgressEntry{ID: id, Text: strings.TrimSpace(text), Actor: actor, At: now.UTC()})
	c.touch(actor, "progress_added", text, now)
	return nil
}

func (c *MaintenanceCase) AddDeviation(d OperationalDeviation, actor string, now time.Time) error {
	if err := c.expectState(StateActive); err != nil {
		return err
	}
	if !contains([]string{"minor", "major", "critical"}, d.Severity) {
		return NewError("invalid_severity", "偏差严重度无效", "severity")
	}
	if strings.TrimSpace(d.Description) == "" || strings.TrimSpace(d.ImmediateAction) == "" {
		return NewError("invalid_deviation", "偏差描述与立即措施不能为空", "description", "immediate_action")
	}
	d.CaseID, d.ObservedAt = c.ID, now.UTC()
	c.Deviations = append(c.Deviations, d)
	if d.Severity == "major" || d.Severity == "critical" {
		c.State = StatePaused
	}
	c.touch(actor, "deviation_added", d.Description, now)
	return nil
}

func (c *MaintenanceCase) ResolveDeviation(id, corrective, verdict, actor string, now time.Time) error {
	if err := c.expectState(StatePaused); err != nil {
		return err
	}
	if strings.TrimSpace(corrective) == "" || verdict != "approved" {
		return NewError("review_required", "必须提交纠正措施并复核通过", "corrective_action", "review_verdict")
	}
	for i := range c.Deviations {
		if c.Deviations[i].ID == id && c.Deviations[i].ResolvedAt == nil {
			resolved := now.UTC()
			c.Deviations[i].CorrectiveAction, c.Deviations[i].ReviewVerdict, c.Deviations[i].ResolvedAt = corrective, verdict, &resolved
			c.touch(actor, "deviation_resolved", id, now)
			if len(c.UnresolvedDeviations()) == 0 {
				c.State = StateActive
				c.addTimeline("work_resumed", actor, "全部严重偏差复核完成", now)
			}
			return nil
		}
	}
	return NewError("deviation_not_found", "未找到待复核偏差")
}

func (c *MaintenanceCase) UnresolvedDeviations() []string {
	var ids []string
	for _, d := range c.Deviations {
		if (d.Severity == "major" || d.Severity == "critical") && d.ResolvedAt == nil {
			ids = append(ids, d.ID)
		}
	}
	return ids
}
