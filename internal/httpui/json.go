package httpui

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"cablewindow/internal/domain"
)

const maxBodyBytes = 1 << 20

type apiError struct {
	Error *domain.Error `json:"error"`
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return domain.NewError("unsupported_media_type", "Content-Type 必须为 application/json")
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return domain.NewError("body_too_large", "请求体超过 1 MiB 限制")
		}
		return domain.NewError("invalid_json", "JSON 请求格式错误")
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return domain.NewError("invalid_json", "请求体只能包含一个 JSON 对象")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, err error) {
	de := &domain.Error{Code: "internal_error", Message: "服务内部错误"}
	if candidate, ok := err.(*domain.Error); ok {
		de = candidate
	}
	status := http.StatusBadRequest
	switch de.Code {
	case "not_found":
		status = http.StatusNotFound
	case "forbidden":
		status = http.StatusForbidden
	case "revision_conflict", "idempotency_conflict", "case_exists", "window_conflict":
		status = http.StatusConflict
	case "unsupported_media_type":
		status = http.StatusUnsupportedMediaType
	case "body_too_large":
		status = http.StatusRequestEntityTooLarge
	case "internal_error":
		status = http.StatusInternalServerError
	}
	writeJSON(w, status, apiError{Error: de})
}
