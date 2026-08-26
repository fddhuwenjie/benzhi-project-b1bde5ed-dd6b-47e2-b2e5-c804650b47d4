package workflow

import (
	"encoding/json"
	"time"

	"cablewindow/internal/domain"
)

type Envelope struct {
	RequestID        string          `json:"request_id"`
	Actor            string          `json:"actor"`
	Role             string          `json:"role"`
	ExpectedRevision int64           `json:"expected_revision"`
	Payload          json.RawMessage `json:"payload"`
}

type CreateInput struct {
	CableSegment string    `json:"cable_segment"`
	WorkScope    string    `json:"work_scope"`
	WindowStart  time.Time `json:"window_start"`
	WindowEnd    time.Time `json:"window_end"`
	Coordinator  string    `json:"coordinator"`
	RiskBaseline string    `json:"risk_baseline"`
}

type EvidenceInput struct {
	Category   string    `json:"category"`
	Subject    string    `json:"subject"`
	Reference  string    `json:"reference"`
	ValidUntil time.Time `json:"valid_until"`
	Note       string    `json:"note"`
}
type ReviseDraftInput struct {
	CableSegment string    `json:"cable_segment"`
	WorkScope    string    `json:"work_scope"`
	WindowStart  time.Time `json:"window_start"`
	WindowEnd    time.Time `json:"window_end"`
	Coordinator  string    `json:"coordinator"`
	RiskBaseline string    `json:"risk_baseline"`
}

type AcknowledgeInput struct {
	PartyRole string `json:"party_role"`
	Decision  string `json:"decision"`
	Comment   string `json:"comment"`
}
type RemediationInput struct {
	PartyRole         string `json:"party_role"`
	Response          string `json:"response"`
	EvidenceReference string `json:"evidence_reference"`
}
type RemediationReviewInput struct {
	PartyRole string `json:"party_role"`
	Decision  string `json:"decision"`
	Comment   string `json:"comment"`
}
type RiskReviewInput struct {
	Items    []domain.RiskItem `json:"items"`
	Decision string            `json:"decision"`
	Reason   string            `json:"reason"`
}
type RiskControlInput struct {
	Kind              string `json:"kind"`
	EvidenceReference string `json:"evidence_reference"`
}
type RiskControlVerificationInput struct {
	Kind    string `json:"kind"`
	Verdict string `json:"verdict"`
	Reason  string `json:"reason"`
}
type ActivateInput struct {
	WeatherReading    string             `json:"weather_reading"`
	ExclusionZone     bool               `json:"exclusion_zone"`
	CommunicationTest bool               `json:"communication_test"`
	MusterPass        bool               `json:"muster_pass"`
	Checks            []domain.GateCheck `json:"checks,omitempty"`
}
type ProgressInput struct {
	Text string `json:"text"`
}
type DeviationInput struct {
	Severity        string `json:"severity"`
	Description     string `json:"description"`
	ImmediateAction string `json:"immediate_action"`
}
type ResolveDeviationInput struct {
	ID               string `json:"id"`
	CorrectiveAction string `json:"corrective_action"`
	ReviewVerdict    string `json:"review_verdict"`
}
type ClosureEvidenceInput struct {
	Category  string `json:"category"`
	Reference string `json:"reference"`
}
