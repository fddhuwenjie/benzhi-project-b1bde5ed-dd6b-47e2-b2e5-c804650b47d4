package workflow

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"cablewindow/internal/domain"
)

type WorkItem struct {
	CaseID           string           `json:"case_id"`
	CableSegment     string           `json:"cable_segment"`
	State            domain.CaseState `json:"state"`
	Revision         int64            `json:"revision"`
	NextAction       string           `json:"next_action,omitempty"`
	Blockers         []string         `json:"blockers,omitempty"`
	WindowStart      time.Time        `json:"window_start"`
	Urgency          string           `json:"urgency"`
	ExpiringEvidence int              `json:"expiring_evidence,omitempty"`
	EarliestExpiry   *time.Time       `json:"earliest_expiry,omitempty"`
}

type Dashboard struct {
	GeneratedAt time.Time                             `json:"generated_at"`
	StateCounts map[domain.CaseState]int              `json:"state_counts"`
	WorkItems   []WorkItem                            `json:"work_items"`
	Readiness   map[string]domain.ReadinessProjection `json:"readiness"`
}

const dashboardCacheTTL = 15 * time.Second

func (s *Service) Dashboard(ctx context.Context) (Dashboard, error) {
	select {
	case <-ctx.Done():
		return Dashboard{}, ctx.Err()
	default:
	}
	now := s.clock().UTC()
	s.dashboardMu.Lock()
	cached := append([]byte(nil), s.dashboardCache...)
	cachedAt := s.dashboardAt
	s.dashboardMu.Unlock()
	if len(cached) > 0 && !now.Before(cachedAt) && now.Sub(cachedAt) < dashboardCacheTTL {
		var dashboard Dashboard
		if err := json.Unmarshal(cached, &dashboard); err != nil {
			return Dashboard{}, err
		}
		return dashboard, nil
	}

	cases, err := s.repo.List(ctx)
	if err != nil {
		return Dashboard{}, err
	}
	d := Dashboard{GeneratedAt: now, StateCounts: map[domain.CaseState]int{}, Readiness: map[string]domain.ReadinessProjection{}}
	for _, c := range cases {
		d.StateCounts[c.State]++
		d.Readiness[c.ID] = c.Readiness(now)
		actions := c.Actions(now)
		next := ""
		if len(actions.Allowed) > 0 {
			next = actions.Allowed[0]
		}
		urgency := "normal"
		expiring := 0
		var earliest *time.Time
		for _, status := range c.EvidenceStatuses(now) {
			if status.Status == "expiring" {
				expiring++
				t := status.ValidUntil
				if earliest == nil || t.Before(*earliest) {
					earliest = &t
				}
			}
		}
		until := c.WindowStart.Sub(now)
		if c.State != domain.StateClosed && until < 0 {
			urgency = "overdue"
		} else if c.State != domain.StateClosed && until <= 24*time.Hour {
			urgency = "urgent"
		}
		d.WorkItems = append(d.WorkItems, WorkItem{CaseID: c.ID, CableSegment: c.CableSegment, State: c.State, Revision: c.Revision, NextAction: next, Blockers: actions.Blockers, WindowStart: c.WindowStart, Urgency: urgency, ExpiringEvidence: expiring, EarliestExpiry: earliest})
	}
	priority := map[string]int{"overdue": 0, "urgent": 1, "normal": 2}
	sort.SliceStable(d.WorkItems, func(i, j int) bool {
		if priority[d.WorkItems[i].Urgency] != priority[d.WorkItems[j].Urgency] {
			return priority[d.WorkItems[i].Urgency] < priority[d.WorkItems[j].Urgency]
		}
		return d.WorkItems[i].WindowStart.Before(d.WorkItems[j].WindowStart)
	})
	encoded, err := json.Marshal(d)
	if err != nil {
		return Dashboard{}, err
	}
	s.dashboardMu.Lock()
	s.dashboardCache = encoded
	s.dashboardAt = now
	s.dashboardMu.Unlock()
	return d, nil
}
