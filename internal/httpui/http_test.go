package httpui

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"cablewindow/internal/store"
	"cablewindow/internal/workflow"
)

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	repo, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return New(workflow.New(repo, func() time.Time { return time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC) }), slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestIndexAndSecurityHeaders(t *testing.T) {
	recorder := httptest.NewRecorder()
	testHandler(t).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "<body>") {
		t.Fatalf("工作台响应无效：%d", recorder.Code)
	}
	if recorder.Header().Get("Content-Security-Policy") == "" || recorder.Header().Get("X-Request-ID") == "" {
		t.Fatal("缺少安全头或请求关联标识")
	}
}

func TestStrictJSONAndContentType(t *testing.T) {
	h := testHandler(t)
	request := httptest.NewRequest(http.MethodPost, "/api/cases", bytes.NewBufferString(`{"unknown":true}`))
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("错误内容类型状态为 %d", recorder.Code)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/cases", bytes.NewBufferString(`{"unknown":true}`))
	request.Header.Set("Content-Type", "application/json")
	recorder = httptest.NewRecorder()
	h.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "invalid_json") {
		t.Fatalf("未知字段未被拒绝：%d %s", recorder.Code, recorder.Body.String())
	}
}
