package errors

import (
	stdErrors "errors"
	"fmt"
)

// Code identifies a machine-readable error class.
type Code string

const (
	CodeInvalidArgument Code = "invalid_argument"
	CodeUnauthenticated Code = "unauthenticated"
	CodeForbidden       Code = "forbidden"
	CodeNotFound        Code = "not_found"
	CodeConflict        Code = "conflict"
	CodeNotImplemented  Code = "not_implemented"
	CodeInternal        Code = "internal"
)

// Error is an application-level structured error.
type Error struct {
	code    Code
	message string
	cause   error
	meta    map[string]any
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.message != "" {
		return e.message
	}
	return string(e.code)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func (e *Error) Code() Code {
	if e == nil {
		return CodeInternal
	}
	if e.code == "" {
		return CodeInternal
	}
	return e.code
}

func (e *Error) Message() string {
	if e == nil {
		return ""
	}
	return e.message
}

func (e *Error) Meta() map[string]any {
	if e == nil || len(e.meta) == 0 {
		return nil
	}
	out := make(map[string]any, len(e.meta))
	for k, v := range e.meta {
		out[k] = v
	}
	return out
}

func New(code Code, message string) *Error {
	return &Error{code: code, message: message}
}

func Wrap(err error, code Code, message string) *Error {
	if err == nil {
		return nil
	}
	return &Error{code: code, message: message, cause: err}
}

func WithCode(err error, code Code) *Error {
	if err == nil {
		return nil
	}
	if appErr, ok := As(err); ok {
		return &Error{code: code, message: appErr.message, cause: appErr.cause, meta: appErr.Meta()}
	}
	return &Error{code: code, message: err.Error(), cause: err}
}

func WithMeta(err error, key string, value any) *Error {
	if err == nil {
		return nil
	}
	appErr, ok := As(err)
	if !ok {
		appErr = Wrap(err, CodeInternal, err.Error())
	}
	meta := appErr.Meta()
	if meta == nil {
		meta = map[string]any{}
	}
	meta[key] = value
	return &Error{code: appErr.code, message: appErr.message, cause: appErr.cause, meta: meta}
}

func NotImplemented(message string) *Error {
	if message == "" {
		message = "not implemented"
	}
	return New(CodeNotImplemented, message)
}

func CodeOf(err error) (Code, bool) {
	appErr, ok := As(err)
	if !ok {
		return "", false
	}
	return appErr.Code(), true
}

func IsCode(err error, code Code) bool {
	if err == nil {
		return false
	}
	found, ok := CodeOf(err)
	if !ok {
		return false
	}
	return found == code
}

func As(err error) (*Error, bool) {
	var appErr *Error
	if stdErrors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}

func Errorf(code Code, format string, args ...any) *Error {
	return New(code, fmt.Sprintf(format, args...))
}
