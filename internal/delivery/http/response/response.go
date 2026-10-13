// Package response renders the JSON envelope used by every HTTP endpoint:
//
//	{"data": ..., "meta": {"request_id": "..."}}
//	{"error": {"code": "...", "message": "...", "fields": [...]}, "meta": {...}}
//
// Handlers return errors and call Success; they never write error bodies.
package response

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"server-template/internal/pkg/apperr"
	"server-template/internal/pkg/observability"
)

// Meta is request metadata attached to every envelope.
type Meta struct {
	RequestID string `json:"request_id,omitzero"`
}

// ErrorBody is the client-facing error payload.
type ErrorBody struct {
	Code    string                  `json:"code"`
	Message string                  `json:"message"`
	Fields  []apperr.FieldViolation `json:"fields,omitzero"`
}

// Envelope is the top-level response document.
type Envelope struct {
	Data  any        `json:"data,omitzero"`
	Error *ErrorBody `json:"error,omitzero"`
	Meta  Meta       `json:"meta,omitzero"`
}

// StatusFor is the single Kind to HTTP status table.
//
// FailedPrecondition maps to 422 rather than 412: 412 is reserved for failed
// HTTP conditional headers (If-Match and friends), while this kind means the
// request is well formed but the resource state forbids the operation.
func StatusFor(kind apperr.Kind) int {
	switch kind {
	case apperr.InvalidArgument:
		return http.StatusBadRequest
	case apperr.Unauthenticated:
		return http.StatusUnauthorized
	case apperr.PermissionDenied:
		return http.StatusForbidden
	case apperr.NotFound:
		return http.StatusNotFound
	case apperr.Conflict:
		return http.StatusConflict
	case apperr.FailedPrecondition:
		return http.StatusUnprocessableEntity
	case apperr.TooManyRequests:
		return http.StatusTooManyRequests
	case apperr.Unavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// Success writes a data envelope. A nil data or a 204 status writes no body.
func Success(c *echo.Context, status int, data any) error {
	if status == http.StatusNoContent {
		return c.NoContent(status)
	}

	return c.JSON(status, Envelope{Data: data, Meta: metaFrom(c)})
}

// Error writes the error envelope for err. Server faults never expose their
// message or fields, and 401/403 never expose fields.
func Error(c *echo.Context, err error) error {
	return ErrorWithStatus(c, err, 0)
}

// ErrorWithStatus is Error with a transport-level status override. apperr
// kinds are transport agnostic and cannot express statuses such as 405, 413,
// 415 or 408, so the HTTP error middleware passes the status echo originally
// chose and it wins over StatusFor. Overrides outside 400-599 are ignored.
// The body is still derived from err by Kind, so redaction rules are unchanged.
func ErrorWithStatus(c *echo.Context, err error, status int) error {
	ae := apperr.From(err)
	if status < http.StatusBadRequest || status > 599 {
		status = StatusFor(ae.Kind)
	}

	return c.JSON(status, Envelope{Error: bodyFor(ae), Meta: metaFrom(c)})
}

func bodyFor(ae *apperr.Error) *ErrorBody {
	switch ae.Kind {
	case apperr.Unavailable:
		return &ErrorBody{Code: apperr.ErrUnavailable.Code, Message: apperr.ErrUnavailable.Message}
	case apperr.Internal:
		return &ErrorBody{Code: apperr.ErrInternal.Code, Message: apperr.ErrInternal.Message}
	case apperr.Unauthenticated, apperr.PermissionDenied:
		return &ErrorBody{Code: ae.Code, Message: ae.Message}
	default:
		return &ErrorBody{Code: ae.Code, Message: ae.Message, Fields: ae.Fields}
	}
}

func metaFrom(c *echo.Context) Meta {
	return Meta{RequestID: observability.CorrelationIDFromContext(c.Request().Context())}
}
