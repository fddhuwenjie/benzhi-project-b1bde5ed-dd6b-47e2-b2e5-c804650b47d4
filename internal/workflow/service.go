package workflow

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"cablewindow/internal/domain"
)

type Clock func() time.Time

type Service struct {
	repo  domain.Repository
	clock Clock
}

func New(repo domain.Repository, clock Clock) *Service {
	if clock == nil {
		clock = time.Now
	}
	return &Service{repo: repo, clock: clock}
}

func (s *Service) findWindowConflicts(ctx context.Context, segment string, start, end time.Time, excludeID string) []domain.WindowConflict {
	finder, ok := s.repo.(interface {
		FindOverlaps(context.Context, string, time.Time, time.Time, string) ([]domain.WindowConflict, error)
	})
	if !ok {
		return nil
	}
	conflicts, err := finder.FindOverlaps(ctx, segment, start, end, excludeID)
	if err != nil {
		return nil
	}
	return conflicts
}

func (s *Service) Create(ctx context.Context, env Envelope, input CreateInput) (domain.CommitResult, error) {
	if err := validateEnvelope(env, true); err != nil {
		return domain.CommitResult{}, err
	}
	if err := authorize(env.Role, "create"); err != nil {
		return domain.CommitResult{}, err
	}
	payload, _ := json.Marshal(input)
	signature := commandSignature("create", env, payload)
	if prior, ok, err := s.repo.LookupRequest(ctx, env.RequestID, signature); ok || err != nil {
		return prior, err
	}
	now := s.clock().UTC()
	c, err := domain.NewCase(newID("case"), input.CableSegment, input.WorkScope, input.WindowStart, input.WindowEnd, input.Coordinator, input.RiskBaseline, now)
	if err != nil {
		return domain.CommitResult{}, err
	}
	if conflicts := s.findWindowConflicts(ctx, strings.TrimSpace(input.CableSegment), input.WindowStart, input.WindowEnd, ""); conflicts != nil {
		e := domain.NewError("window_conflict", "计划时间窗与未关闭个案交叠")
		e.Details = map[string]any{"conflicts": conflicts}
		return domain.CommitResult{}, e
	}
	return s.repo.Create(ctx, domain.Mutation{Case: c, ExpectedRevision: 0, RequestID: env.RequestID, Signature: signature, EventType: "case_created", Actor: env.Actor})
}

func (s *Service) Execute(ctx context.Context, caseID, action string, env Envelope) (domain.CommitResult, error) {
	if err := validateEnvelope(env, false); err != nil {
		return domain.CommitResult{}, err
	}
	if err := authorize(env.Role, action); err != nil {
		return domain.CommitResult{}, err
	}
	signature := commandSignature(caseID+"|"+action, env, env.Payload)
	if prior, ok, err := s.repo.LookupRequest(ctx, env.RequestID, signature); ok || err != nil {
		return prior, err
	}
	c, err := s.repo.Get(ctx, caseID)
	if err != nil {
		return domain.CommitResult{}, err
	}
	if c.Revision != env.ExpectedRevision {
		return domain.CommitResult{}, domain.Conflict(c.Revision)
	}
	now := s.clock().UTC()
	if (action == "review_risk" || action == "activate") && len(c.CoverageBlockers(now)) > 0 {
		return domain.CommitResult{}, domain.Blocked(c.CoverageBlockers(now))
	}
	if action == "revise_draft" || action == "revise" || action == "revise_case" {
		var in ReviseDraftInput
		if err := decodePayload(env.Payload, &in); err != nil {
			return domain.CommitResult{}, err
		}
		if conflicts := s.findWindowConflicts(ctx, strings.TrimSpace(in.CableSegment), in.WindowStart, in.WindowEnd, caseID); conflicts != nil {
			e := domain.NewError("window_conflict", "计划时间窗与未关闭个案交叠")
			e.Details = map[string]any{"conflicts": conflicts}
			return domain.CommitResult{}, e
		}
	}
	if err := s.apply(c, action, env, now); err != nil {
		if action == "activate" && c.Revision != env.ExpectedRevision {
			_, _ = s.repo.Commit(ctx, domain.Mutation{Case: c, ExpectedRevision: env.ExpectedRevision, RequestID: env.RequestID + "#failed", Signature: signature + ":failed", EventType: action + "_failed", Actor: env.Actor})
		}
		return domain.CommitResult{}, err
	}
	result, err := s.repo.Commit(ctx, domain.Mutation{Case: c, ExpectedRevision: env.ExpectedRevision, RequestID: env.RequestID, Signature: signature, EventType: action, Actor: env.Actor})
	if err == nil && action == "close" {
		if audit, auditErr := s.repo.AuditSummary(ctx, caseID); auditErr == nil {
			_ = result.Case.BuildArchive(audit)
		}
	}
	return result, err
}

func (s *Service) apply(c *domain.MaintenanceCase, action string, env Envelope, now time.Time) error {
	switch action {
	case "revise_draft", "revise", "revise_case":
		var in ReviseDraftInput
		if err := decodePayload(env.Payload, &in); err != nil {
			return err
		}
		return c.ReviseDraft(in.CableSegment, in.WorkScope, in.WindowStart, in.WindowEnd, in.Coordinator, in.RiskBaseline, env.Actor, now)
	case "put_evidence":
		var in EvidenceInput
		if err := decodePayload(env.Payload, &in); err != nil {
			return err
		}
		return c.PutEvidence(domain.ReadinessEvidence{ID: newID("evidence"), Category: in.Category, Subject: in.Subject, Reference: in.Reference, ValidUntil: in.ValidUntil, Note: in.Note}, env.Actor, now)
	case "submit_coordination":
		if err := ensureEmptyPayload(env.Payload); err != nil {
			return err
		}
		if blockers := c.CoverageBlockers(now); len(blockers) > 0 {
			return domain.Blocked(blockers)
		}
		return c.SubmitForCoordination(env.Actor, now)
	case "acknowledge":
		var in AcknowledgeInput
		if err := decodePayload(env.Payload, &in); err != nil {
			return err
		}
		if in.PartyRole != env.Role {
			return domain.NewError("party_role_mismatch", "协调席位必须与角色一致", "party_role")
		}
		return c.Acknowledge(domain.CoordinationAcknowledgement{ID: newID("ack"), PartyRole: in.PartyRole, Decision: in.Decision, Comment: in.Comment}, env.Actor, now)
	case "submit_remediation", "submit_coordination_remediation":
		var in RemediationInput
		if err := decodePayload(env.Payload, &in); err != nil {
			return err
		}
		return c.SubmitRemediation(in.PartyRole, in.Response, in.EvidenceReference, env.Actor, now)
	case "review_remediation", "review_coordination_remediation":
		var in RemediationReviewInput
		if err := decodePayload(env.Payload, &in); err != nil {
			return err
		}
		if in.PartyRole != env.Role {
			return domain.NewError("party_role_mismatch", "复核席位必须与角色一致", "party_role")
		}
		return c.ReviewRemediation(in.PartyRole, in.Decision, in.Comment, env.Actor, now)
	case "review_risk":
		var in RiskReviewInput
		if err := decodePayload(env.Payload, &in); err != nil {
			return err
		}
		return c.ReviewRisks(in.Items, in.Decision, in.Reason, env.Actor, now)
	case "submit_risk_control", "submit_control_evidence":
		var in RiskControlInput
		if err := decodePayload(env.Payload, &in); err != nil {
			return err
		}
		return c.SubmitRiskControl(in.Kind, in.EvidenceReference, env.Actor, now)
	case "verify_risk_control", "verify_control":
		var in RiskControlVerificationInput
		if err := decodePayload(env.Payload, &in); err != nil {
			return err
		}
		return c.VerifyRiskControl(in.Kind, in.Verdict, in.Reason, env.Actor, now)
	case "activate":
		var in ActivateInput
		if err := decodePayload(env.Payload, &in); err != nil {
			return err
		}
		return c.Activate(domain.StartGate{WeatherReading: in.WeatherReading, ExclusionZone: in.ExclusionZone, CommunicationTest: in.CommunicationTest, MusterPass: in.MusterPass, Checks: in.Checks}, env.Actor, now)
	case "add_progress":
		var in ProgressInput
		if err := decodePayload(env.Payload, &in); err != nil {
			return err
		}
		return c.AddProgress(newID("progress"), in.Text, env.Actor, now)
	case "add_deviation":
		var in DeviationInput
		if err := decodePayload(env.Payload, &in); err != nil {
			return err
		}
		return c.AddDeviation(domain.OperationalDeviation{ID: newID("deviation"), Severity: in.Severity, Description: in.Description, ImmediateAction: in.ImmediateAction}, env.Actor, now)
	case "resolve_deviation":
		var in ResolveDeviationInput
		if err := decodePayload(env.Payload, &in); err != nil {
			return err
		}
		return c.ResolveDeviation(in.ID, in.CorrectiveAction, in.ReviewVerdict, env.Actor, now)
	case "submit_closure_evidence":
		var in ClosureEvidenceInput
		if err := decodePayload(env.Payload, &in); err != nil {
			return err
		}
		return c.SubmitClosureEvidence(domain.ClosureEvidence{Category: in.Category, Reference: in.Reference}, env.Actor, now)
	case "close":
		if err := ensureEmptyPayload(env.Payload); err != nil {
			return err
		}
		return c.Close(env.Actor, now)
	default:
		return domain.NewError("unknown_action", "未知状态命令")
	}
}

func validateEnvelope(env Envelope, creating bool) error {
	var fields []string
	if strings.TrimSpace(env.RequestID) == "" {
		fields = append(fields, "request_id")
	}
	if strings.TrimSpace(env.Actor) == "" {
		fields = append(fields, "actor")
	}
	if strings.TrimSpace(env.Role) == "" {
		fields = append(fields, "role")
	}
	if creating && env.ExpectedRevision != 0 {
		fields = append(fields, "expected_revision")
	}
	if !creating && env.ExpectedRevision < 1 {
		fields = append(fields, "expected_revision")
	}
	if len(fields) > 0 {
		return domain.NewError("invalid_command", "命令信封字段不完整", fields...)
	}
	return nil
}

func decodePayload(raw json.RawMessage, target any) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return domain.NewError("payload_required", "命令载荷不能为空", "payload")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return domain.NewError("invalid_payload", "命令载荷格式错误", "payload")
	}
	if dec.Decode(&struct{}{}) == nil {
		return domain.NewError("invalid_payload", "命令载荷只能包含一个 JSON 值", "payload")
	}
	return nil
}

func ensureEmptyPayload(raw json.RawMessage) error {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "{}" || trimmed == "null" {
		return nil
	}
	return domain.NewError("unexpected_payload", "此命令不接受载荷", "payload")
}

func commandSignature(action string, env Envelope, payload []byte) string {
	canonical := action + "\n" + env.Actor + "\n" + env.Role + "\n" + strings.TrimSpace(string(payload))
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}
