package domain

import "time"

type CaseState string

const (
	StateDraft         CaseState = "draft"
	StateCoordination  CaseState = "coordination"
	StateReview        CaseState = "review"
	StateAuthorization CaseState = "authorization"
	StateActive        CaseState = "active"
	StatePaused        CaseState = "paused"
	StatePendingClose  CaseState = "pending_close"
	StateClosed        CaseState = "closed"
)

type MaintenanceCase struct {
	ID                 string                        `json:"id"`
	CableSegment       string                        `json:"cable_segment"`
	WorkScope          string                        `json:"work_scope"`
	WindowStart        time.Time                     `json:"window_start"`
	WindowEnd          time.Time                     `json:"window_end"`
	Coordinator        string                        `json:"coordinator"`
	State              CaseState                     `json:"state"`
	Revision           int64                         `json:"revision"`
	RiskBaseline       string                        `json:"risk_baseline"`
	CreatedAt          time.Time                     `json:"created_at"`
	UpdatedAt          time.Time                     `json:"updated_at"`
	ClosedAt           *time.Time                    `json:"closed_at,omitempty"`
	Evidence           []ReadinessEvidence           `json:"evidence"`
	EvidenceHistory    []ReadinessEvidence           `json:"evidence_history,omitempty"`
	Acknowledgements   []CoordinationAcknowledgement `json:"acknowledgements"`
	CoordinationRounds []CoordinationRound           `json:"coordination_rounds,omitempty"`
	Remediations       []CoordinationRemediation     `json:"remediations,omitempty"`
	RiskItems          []RiskItem                    `json:"risk_items"`
	RiskReturns        []RiskReturn                  `json:"risk_returns"`
	Gate               *StartGate                    `json:"start_gate,omitempty"`
	GateAttempts       []GateAttempt                 `json:"gate_attempts,omitempty"`
	Progress           []ProgressEntry               `json:"progress"`
	Deviations         []OperationalDeviation        `json:"deviations"`
	ClosureEvidence    []ClosureEvidence             `json:"closure_evidence"`
	Timeline           []TimelineEntry               `json:"timeline"`
	CloseSummary       *CloseSummary                 `json:"close_summary,omitempty"`
	Archive            *ArchivePackage               `json:"archive,omitempty"`
	DraftChanges       []DraftChange                 `json:"draft_changes,omitempty"`
}

type ReadinessEvidence struct {
	ID         string     `json:"id"`
	CaseID     string     `json:"case_id"`
	Category   string     `json:"category"`
	Subject    string     `json:"subject"`
	Reference  string     `json:"reference"`
	ValidUntil time.Time  `json:"valid_until"`
	VerifiedBy string     `json:"verified_by"`
	VerifiedAt *time.Time `json:"verified_at,omitempty"`
	Verdict    string     `json:"verdict"`
	Note       string     `json:"note,omitempty"`
	ReplacedAt *time.Time `json:"replaced_at,omitempty"`
	ReplacedBy string     `json:"replaced_by,omitempty"`
}

type EvidenceStatus struct {
	Category   string    `json:"category"`
	Status     string    `json:"status"`
	ValidUntil time.Time `json:"valid_until"`
	Reference  string    `json:"reference,omitempty"`
}

type CoordinationRound struct {
	Round     int       `json:"round"`
	PartyRole string    `json:"party_role"`
	Decision  string    `json:"decision"`
	Comment   string    `json:"comment,omitempty"`
	Actor     string    `json:"actor"`
	At        time.Time `json:"at"`
}

type CoordinationRemediation struct {
	ID                string    `json:"id"`
	PartyRole         string    `json:"party_role"`
	Round             int       `json:"round"`
	Objection         string    `json:"objection"`
	Response          string    `json:"response,omitempty"`
	EvidenceReference string    `json:"evidence_reference,omitempty"`
	Status            string    `json:"status"`
	Actor             string    `json:"actor"`
	At                time.Time `json:"at"`
}

type CoordinationAcknowledgement struct {
	ID        string    `json:"id"`
	CaseID    string    `json:"case_id"`
	PartyRole string    `json:"party_role"`
	Decision  string    `json:"decision"`
	Comment   string    `json:"comment"`
	Actor     string    `json:"actor"`
	DecidedAt time.Time `json:"decided_at"`
}

type RiskItem struct {
	Kind                string             `json:"kind"`
	Rating              string             `json:"rating"`
	Control             string             `json:"control"`
	Reviewer            string             `json:"reviewer"`
	ReviewedAt          time.Time          `json:"reviewed_at"`
	ControlOwner        string             `json:"control_owner,omitempty"`
	DueAt               time.Time          `json:"due_at,omitempty"`
	ControlStatus       string             `json:"control_status,omitempty"`
	ControlEvidence     string             `json:"control_evidence,omitempty"`
	Verification        *RiskVerification  `json:"verification,omitempty"`
	VerificationHistory []RiskVerification `json:"verification_history,omitempty"`
}

type RiskVerification struct {
	Actor   string    `json:"actor"`
	Verdict string    `json:"verdict"`
	Reason  string    `json:"reason,omitempty"`
	At      time.Time `json:"at"`
}

type RiskReturn struct {
	Reason   string    `json:"reason"`
	Reviewer string    `json:"reviewer"`
	At       time.Time `json:"at"`
}

type StartGate struct {
	WeatherReading    string      `json:"weather_reading"`
	Actor             string      `json:"actor"`
	ExclusionZone     bool        `json:"exclusion_zone"`
	CommunicationTest bool        `json:"communication_test"`
	MusterPass        bool        `json:"muster_pass"`
	CheckedAt         time.Time   `json:"checked_at"`
	Checks            []GateCheck `json:"checks,omitempty"`
}

type GateCheck struct {
	Kind      string    `json:"kind"`
	Passed    bool      `json:"passed"`
	CheckedAt time.Time `json:"checked_at"`
	Actor     string    `json:"actor"`
	Note      string    `json:"note,omitempty"`
}

type GateAttempt struct {
	At       time.Time   `json:"at"`
	Actor    string      `json:"actor"`
	Checks   []GateCheck `json:"checks"`
	Passed   bool        `json:"passed"`
	Failures []string    `json:"failures,omitempty"`
}

type ProgressEntry struct {
	ID    string    `json:"id"`
	Text  string    `json:"text"`
	Actor string    `json:"actor"`
	At    time.Time `json:"at"`
}

type OperationalDeviation struct {
	ID               string     `json:"id"`
	CaseID           string     `json:"case_id"`
	Severity         string     `json:"severity"`
	Description      string     `json:"description"`
	ImmediateAction  string     `json:"immediate_action"`
	ObservedAt       time.Time  `json:"observed_at"`
	CorrectiveAction string     `json:"corrective_action,omitempty"`
	ReviewVerdict    string     `json:"review_verdict,omitempty"`
	ResolvedAt       *time.Time `json:"resolved_at,omitempty"`
}

type ClosureEvidence struct {
	Category    string    `json:"category"`
	Reference   string    `json:"reference"`
	Actor       string    `json:"actor"`
	SubmittedAt time.Time `json:"submitted_at"`
}

type TimelineEntry struct {
	Type     string    `json:"type"`
	Actor    string    `json:"actor"`
	Summary  string    `json:"summary"`
	At       time.Time `json:"at"`
	Revision int64     `json:"revision"`
}

type CloseSummary struct {
	CaseID         string    `json:"case_id"`
	CableSegment   string    `json:"cable_segment"`
	WorkScope      string    `json:"work_scope"`
	Window         string    `json:"window"`
	Coordinator    string    `json:"coordinator"`
	EvidenceCount  int       `json:"evidence_count"`
	ProgressCount  int       `json:"progress_count"`
	DeviationCount int       `json:"deviation_count"`
	ClosedAt       time.Time `json:"closed_at"`
	Digest         string    `json:"digest"`
}

type DraftChange struct {
	Revision int64             `json:"revision"`
	Actor    string            `json:"actor"`
	At       time.Time         `json:"at"`
	Fields   map[string]string `json:"fields"`
}

type ArchivePackage struct {
	Version         string            `json:"version"`
	Case            MaintenanceCase   `json:"case"`
	Timeline        []TimelineEntry   `json:"timeline"`
	ClosureEvidence []ClosureEvidence `json:"closure_evidence"`
	CloseSummary    CloseSummary      `json:"close_summary"`
	AuditSummary    AuditSummary      `json:"audit_summary"`
	Digest          string            `json:"digest"`
}

type AuditEvent struct {
	Sequence      int64     `json:"sequence"`
	CaseID        string    `json:"case_id"`
	RequestID     string    `json:"request_id"`
	EventType     string    `json:"event_type"`
	Actor         string    `json:"actor"`
	OccurredAt    time.Time `json:"occurred_at"`
	FromRevision  int64     `json:"from_revision"`
	ToRevision    int64     `json:"to_revision"`
	PayloadDigest string    `json:"payload_digest"`
	PreviousHash  string    `json:"previous_hash"`
	EventHash     string    `json:"event_hash"`
}
