// Package apperr defines the single application error type shared by every
// layer. Errors carry a transport-agnostic Kind; delivery layers (HTTP, gRPC)
// own the mapping from Kind to their own status codes.
package apperr

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
)

// Kind classifies an error independently of any transport.
type Kind uint8

const (
	// Internal is the zero value so an unclassified error is never exposed as a client error.
	Internal Kind = iota
	InvalidArgument
	NotFound
	Conflict
	Unauthenticated
	PermissionDenied
	FailedPrecondition
	TooManyRequests
	Unavailable
)

var kindNames = [...]string{
	Internal:           "internal",
	InvalidArgument:    "invalid_argument",
	NotFound:           "not_found",
	Conflict:           "conflict",
	Unauthenticated:    "unauthenticated",
	PermissionDenied:   "permission_denied",
	FailedPrecondition: "failed_precondition",
	TooManyRequests:    "too_many_requests",
	Unavailable:        "unavailable",
}

func (k Kind) String() string {
	if int(k) < len(kindNames) {
		return kindNames[k]
	}

	return fmt.Sprintf("kind(%d)", k)
}

// IsServerFault reports whether the kind represents a server-side failure.
func (k Kind) IsServerFault() bool {
	return k == Internal || k == Unavailable
}

// FieldViolation describes one invalid input field.
type FieldViolation struct {
	Field       string `json:"field"`
	Description string `json:"description"`
}

// Error is the canonical application error.
//
// Code and Message are safe to show to clients. The wrapped cause and the
// stack are for logs and traces only and must never be sent to clients.
type Error struct {
	Kind    Kind
	Code    string
	Message string
	Fields  []FieldViolation

	cause error
	stack string
}

// Error implements the error interface. It includes the cause for logging.
func (e *Error) Error() string {
	if e.cause == nil {
		return e.Code + ": " + e.Message
	}

	return e.Code + ": " + e.Message + ": " + e.cause.Error()
}

// Unwrap exposes the cause so errors.Is / errors.AsType still see it.
func (e *Error) Unwrap() error {
	return e.cause
}

// Is matches by Code, so a wrapped copy still equals its sentinel.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)

	return ok && t.Code == e.Code
}

// Cause returns the wrapped internal cause, if any.
func (e *Error) Cause() error {
	return e.cause
}

// Stack returns the stack captured when the error was first wrapped.
func (e *Error) Stack() string {
	return e.stack
}

// WithFields returns a copy carrying field violations.
func (e *Error) WithFields(fields ...FieldViolation) *Error {
	c := *e
	c.Fields = append([]FieldViolation(nil), fields...)

	return &c
}

// WithMessage returns a copy with a different client-facing message.
func (e *Error) WithMessage(msg string) *Error {
	c := *e
	c.Message = msg

	return &c
}

// New defines a sentinel error. Use it for package-level variables.
func New(kind Kind, code, message string) *Error {
	return &Error{Kind: kind, Code: code, Message: message}
}

// Wrap attaches cause to a copy of sentinel and captures the caller's stack.
// If cause already carries a stack (it is an *Error), that stack is kept
// because it points closer to where the failure happened.
func Wrap(cause error, sentinel *Error) *Error {
	c := *sentinel
	c.cause = cause
	if inner, ok := errors.AsType[*Error](cause); ok && inner.stack != "" {
		c.stack = inner.stack
	} else {
		c.stack = captureStack(3)
	}

	return &c
}

// From converts any error into an *Error. Errors that are not application
// errors become ErrInternal with the original error as cause.
func From(err error) *Error {
	if err == nil {
		return nil
	}
	if e, ok := errors.AsType[*Error](err); ok {
		return e
	}

	return Wrap(err, ErrInternal)
}

// KindOf returns the Kind of err (Internal for non-application errors).
func KindOf(err error) Kind {
	if e, ok := errors.AsType[*Error](err); ok {
		return e.Kind
	}

	return Internal
}

// IsServerFault reports whether err should be treated as a server failure
// (5xx / span error). A nil error is not a fault.
func IsServerFault(err error) bool {
	return err != nil && KindOf(err).IsServerFault()
}

func captureStack(skip int) string {
	var pcs [32]uintptr
	n := runtime.Callers(skip, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])

	var b strings.Builder
	for {
		f, more := frames.Next()
		fmt.Fprintf(&b, "%s\n\t%s:%d\n", f.Function, f.File, f.Line)
		if !more {
			break
		}
	}

	return b.String()
}
