package domain

import (
	"context"
	"time"
)

type Mutation struct {
	Case             *MaintenanceCase
	ExpectedRevision int64
	RequestID        string
	Signature        string
	EventType        string
	Actor            string
}

type CommitResult struct {
	Case       *MaintenanceCase `json:"case"`
	Idempotent bool             `json:"idempotent"`
}

type Repository interface {
	Create(context.Context, Mutation) (CommitResult, error)
	Commit(context.Context, Mutation) (CommitResult, error)
	LookupRequest(context.Context, string, string) (CommitResult, bool, error)
	Get(context.Context, string) (*MaintenanceCase, error)
	List(context.Context) ([]*MaintenanceCase, error)
	Audit(context.Context, string, int, int) ([]AuditEvent, error)
	VerifyAudit(context.Context, string) (AuditVerification, error)
	AuditSummary(context.Context, string) (AuditSummary, error)
	Ready() bool
}

type WindowConflict struct {
	CaseID       string    `json:"case_id"`
	State        CaseState `json:"state"`
	OverlapStart time.Time `json:"overlap_start"`
	OverlapEnd   time.Time `json:"overlap_end"`
}

type AuditVerification struct {
	CaseID     string `json:"case_id"`
	Valid      bool   `json:"valid"`
	EventCount int    `json:"event_count"`
	FirstHash  string `json:"first_hash,omitempty"`
	LastHash   string `json:"last_hash,omitempty"`
	Revision   int64  `json:"revision"`
}
