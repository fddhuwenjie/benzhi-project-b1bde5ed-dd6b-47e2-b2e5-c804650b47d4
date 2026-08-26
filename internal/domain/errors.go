package domain

import "fmt"

type Error struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Fields  []string       `json:"fields,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

func (e *Error) Error() string { return e.Message }

func NewError(code, message string, fields ...string) *Error {
	return &Error{Code: code, Message: message, Fields: fields}
}

func Blocked(reasons []string) *Error {
	return NewError("blocked", "当前条件不允许执行该操作", reasons...)
}

func IsCode(err error, code string) bool {
	e, ok := err.(*Error)
	return ok && e.Code == code
}

func Conflict(current int64) *Error {
	return NewError("revision_conflict", fmt.Sprintf("修订冲突，当前修订为 %d", current))
}
