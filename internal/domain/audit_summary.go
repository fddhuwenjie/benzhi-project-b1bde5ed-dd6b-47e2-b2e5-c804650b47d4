package domain

import "time"

type EventTypeCount struct {
	EventType string `json:"event_type"`
	Count     int    `json:"count"`
}

type AuditSummary struct {
	CaseID          string           `json:"case_id"`
	EventCount      int              `json:"event_count"`
	FirstOccurredAt time.Time        `json:"first_occurred_at"`
	LastOccurredAt  time.Time        `json:"last_occurred_at"`
	FirstHash       string           `json:"first_hash"`
	LastHash        string           `json:"last_hash"`
	FinalRevision   int64            `json:"final_revision"`
	Actors          []string         `json:"actors"`
	EventTypes      []EventTypeCount `json:"event_types"`
	Digest          string           `json:"digest"`
}
