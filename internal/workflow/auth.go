package workflow

import "cablewindow/internal/domain"

var permissions = map[string]map[string]bool{
	"coordinator": {
		"create": true, "put_evidence": true, "submit_coordination": true, "revise_draft": true, "revise": true, "revise_case": true, "submit_remediation": true, "submit_coordination_remediation": true,
	},
	"maritime_liaison": {"acknowledge": true, "review_remediation": true, "review_coordination_remediation": true},
	"cable_owner":      {"acknowledge": true, "review_remediation": true, "review_coordination_remediation": true},
	"vessel_party":     {"acknowledge": true, "review_remediation": true, "review_coordination_remediation": true},
	"safety_reviewer":  {"review_risk": true, "resolve_deviation": true, "verify_risk_control": true, "verify_control": true},
	"site_lead": {
		"activate": true, "add_progress": true, "add_deviation": true,
		"submit_closure_evidence": true, "close": true, "submit_risk_control": true, "submit_control_evidence": true,
	},
}

func authorize(role, action string) error {
	if permissions[role] == nil || !permissions[role][action] {
		return domain.NewError("forbidden", "当前角色无权执行该操作", "role")
	}
	return nil
}
