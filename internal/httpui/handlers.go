package httpui

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"cablewindow/internal/domain"
	"cablewindow/internal/workflow"
)

type App struct {
	service *workflow.Service
	logger  *slog.Logger
}

func New(service *workflow.Service, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	a := &App{service: service, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("/", a.HandleFallback)
	mux.HandleFunc("GET /assets/app.css", a.HandleCSS)
	mux.HandleFunc("GET /assets/app.js", a.HandleJS)
	mux.HandleFunc("GET /readyz", a.HandleReadiness)
	mux.HandleFunc("GET /api/cases", a.HandleListCases)
	mux.HandleFunc("GET /api/dashboard", a.HandleDashboard)
	mux.HandleFunc("POST /api/cases", a.HandleCreateCase)
	mux.HandleFunc("GET /api/cases/{id}", a.HandleGetCase)
	mux.HandleFunc("GET /api/cases/{id}/audit", a.HandleAudit)
	mux.HandleFunc("GET /api/cases/{id}/audit/verify", a.HandleAuditVerification)
	mux.HandleFunc("GET /api/cases/{id}/audit/summary", a.HandleAuditSummary)
	mux.HandleFunc("GET /api/cases/{id}/summary", a.HandleCloseSummary)
	mux.HandleFunc("GET /api/cases/{id}/closure-precheck", a.HandleClosurePrecheck)
	mux.HandleFunc("GET /api/cases/{id}/archive", a.HandleArchive)
	mux.HandleFunc("POST /api/cases/{id}/commands/{action}", a.HandleCommand)
	return Middleware(logger, mux)
}

func (a *App) HandleFallback(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" && r.Method == http.MethodGet {
		a.HandleIndex(w, r)
		return
	}
	if knownRoutePath(r.URL.Path) {
		w.Header().Set("Allow", allowedMethods(r.URL.Path))
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: domain.NewError("method_not_allowed", "该路由不支持此 HTTP 方法")})
		return
	}
	writeError(w, domain.NewError("not_found", "路由不存在"))
}

func knownRoutePath(path string) bool {
	if path == "/" || path == "/readyz" || path == "/api/cases" || path == "/api/dashboard" || path == "/assets/app.css" || path == "/assets/app.js" {
		return true
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 3 && parts[0] == "api" && parts[1] == "cases" {
		return true
	}
	if len(parts) == 4 && parts[0] == "api" && parts[1] == "cases" && (parts[3] == "audit" || parts[3] == "summary") {
		return true
	}
	if len(parts) == 4 && parts[0] == "api" && parts[1] == "cases" && (parts[3] == "closure-precheck" || parts[3] == "archive") {
		return true
	}
	if len(parts) == 5 && parts[0] == "api" && parts[1] == "cases" && parts[3] == "audit" && (parts[4] == "verify" || parts[4] == "summary") {
		return true
	}
	if len(parts) == 5 && parts[0] == "api" && parts[1] == "cases" && parts[3] == "commands" {
		return true
	}
	return false
}

func allowedMethods(path string) string {
	if path == "/api/cases" {
		return "GET, POST"
	}
	if strings.Contains(path, "/commands/") {
		return "POST"
	}
	return "GET"
}

func (a *App) HandleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		writeError(w, domain.NewError("not_found", "页面不存在"))
		return
	}
	b, err := assets.ReadFile("static/index.html")
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b)
}

func (a *App) HandleCSS(w http.ResponseWriter, _ *http.Request) {
	b, err := assets.ReadFile("static/app.css")
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	_, _ = w.Write(b)
}
func (a *App) HandleJS(w http.ResponseWriter, _ *http.Request) {
	b, err := assets.ReadFile("static/app.js")
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	_, _ = w.Write(b)
}

func (a *App) HandleReadiness(w http.ResponseWriter, _ *http.Request) {
	if !a.service.Ready() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ready": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ready": true})
}

func (a *App) HandleListCases(w http.ResponseWriter, r *http.Request) {
	result, err := a.service.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"cases": result})
}

func (a *App) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	dashboard, err := a.service.Dashboard(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dashboard)
}

type createRequest struct {
	workflow.Envelope
	Input workflow.CreateInput `json:"input"`
}

func (a *App) HandleCreateCase(w http.ResponseWriter, r *http.Request) {
	var request createRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, err)
		return
	}
	result, err := a.service.Create(r.Context(), request.Envelope, request.Input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (a *App) HandleGetCase(w http.ResponseWriter, r *http.Request) {
	view, err := a.service.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if category, status := r.URL.Query().Get("evidence_category"), r.URL.Query().Get("evidence_status"); category != "" || status != "" {
		filtered := view.Case.Evidence[:0]
		for _, e := range view.Case.Evidence {
			if category != "" && e.Category != category {
				continue
			}
			if status != "" {
				matched := false
				for _, s := range view.Case.EvidenceStatuses(a.service.Now()) {
					if s.Category == e.Category && s.Status == status {
						matched = true
						break
					}
				}
				if !matched {
					continue
				}
			}
			filtered = append(filtered, e)
		}
		view.Case.Evidence = filtered
	}
	writeJSON(w, http.StatusOK, view)
}

func (a *App) HandleCommand(w http.ResponseWriter, r *http.Request) {
	var env workflow.Envelope
	if err := decodeJSON(w, r, &env); err != nil {
		writeError(w, err)
		return
	}
	result, err := a.service.Execute(r.Context(), r.PathValue("id"), r.PathValue("action"), env)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) HandleAudit(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	events, err := a.service.Audit(r.Context(), r.PathValue("id"), offset, limit)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events, "offset": offset, "limit": limit})
}

func (a *App) HandleAuditVerification(w http.ResponseWriter, r *http.Request) {
	verification, err := a.service.VerifyAudit(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"verification": verification})
}

func (a *App) HandleAuditSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := a.service.AuditSummary(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"audit_summary": summary})
}

func (a *App) HandleCloseSummary(w http.ResponseWriter, r *http.Request) {
	view, err := a.service.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if view.Case.CloseSummary == nil {
		writeError(w, domain.NewError("summary_unavailable", "个案尚未关闭，暂无关闭摘要"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"summary": view.Case.CloseSummary, "audit_path": strings.TrimSuffix(r.URL.Path, "/summary") + "/audit"})
}

func (a *App) HandleClosurePrecheck(w http.ResponseWriter, r *http.Request) {
	result, err := a.service.ClosurePrecheck(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) HandleArchive(w http.ResponseWriter, r *http.Request) {
	archive, err := a.service.Archive(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, archive)
}
