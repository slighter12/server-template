package middleware

import (
	"errors"
	"net/http"

	"server-template/internal/delivery/http/response"
	"server-template/internal/pkg/apperr"
	"server-template/internal/pkg/observability"

	"github.com/labstack/echo/v5"
)

// HTTP-level failures that have no dedicated apperr sentinel. Their Kind only
// decides redaction and logging; the real status (405/413/415/408) is carried
// separately from echo's original error, see handleError.
var (
	errMethodNotAllowed = apperr.New(apperr.InvalidArgument, "METHOD_NOT_ALLOWED", "method not allowed")
	errPayloadTooLarge  = apperr.New(apperr.InvalidArgument, "PAYLOAD_TOO_LARGE", "request body too large")
	errUnsupportedMedia = apperr.New(apperr.InvalidArgument, "UNSUPPORTED_MEDIA_TYPE", "unsupported media type")
	errRequestTimeout   = apperr.New(apperr.InvalidArgument, "REQUEST_TIMEOUT", "request timed out")
)

// HandleErrors converts every error returned by the rest of the chain into
// the response envelope, so outer middlewares (the request logger) see the
// final status. Handlers only return errors; they never write error bodies.
func HandleErrors() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			err := next(c)
			if err == nil {
				return nil
			}
			if handleError(c, err) {
				return nil
			}

			// Already committed: nothing more to render, keep the error visible to outer layers.
			return err
		}
	}
}

// HTTPErrorHandler is for echo.Echo.HTTPErrorHandler. It covers errors raised
// outside HandleErrors and funnels them through the same code path.
func HTTPErrorHandler(c *echo.Context, err error) {
	if err == nil {
		return
	}
	handleError(c, err)
}

// handleError records err in RequestInfo and renders it. It reports whether a
// response was written.
func handleError(c *echo.Context, err error) bool {
	ae := toAppError(err)
	if info := observability.RequestInfoFrom(c.Request().Context()); info != nil {
		info.Err = ae
	}
	if resp, _ := echo.UnwrapResponse(c.Response()); resp != nil && resp.Committed {
		return false
	}
	if renderErr := response.ErrorWithStatus(c, ae, transportStatus(err)); renderErr != nil {
		logger := observability.LoggerFromContextOrDefault(c.Request().Context(), c.Logger())
		logger.ErrorContext(c.Request().Context(), "render error response failed", "error", renderErr)
	}

	return true
}

// transportStatus returns the HTTP status of an echo-originated error (routing,
// body limit, binding, timeouts) so it survives the Kind-based mapping. It is 0
// for apperr errors, whose status always comes from their Kind.
func transportStatus(err error) int {
	if _, ok := errors.AsType[*apperr.Error](err); ok {
		return 0
	}

	return echo.StatusCode(err)
}

// toAppError returns the *apperr.Error for err, mapping Echo's own
// *echo.HTTPError (routing, body limit, binding) onto apperr sentinels.
func toAppError(err error) *apperr.Error {
	if _, ok := errors.AsType[*apperr.Error](err); ok {
		return apperr.From(err)
	}
	code := echo.StatusCode(err)
	if code == 0 {
		return apperr.From(err)
	}

	var sentinel *apperr.Error
	switch code {
	case http.StatusUnauthorized:
		sentinel = apperr.ErrUnauthenticated
	case http.StatusForbidden:
		sentinel = apperr.ErrForbidden
	case http.StatusNotFound:
		sentinel = apperr.ErrNotFound
	case http.StatusMethodNotAllowed:
		sentinel = errMethodNotAllowed
	case http.StatusRequestEntityTooLarge:
		sentinel = errPayloadTooLarge
	case http.StatusUnsupportedMediaType:
		sentinel = errUnsupportedMedia
	case http.StatusTooManyRequests:
		sentinel = apperr.ErrTooManyRequests
	case http.StatusRequestTimeout:
		sentinel = errRequestTimeout
	case http.StatusServiceUnavailable:
		sentinel = apperr.ErrUnavailable
	default:
		if code >= http.StatusInternalServerError {
			sentinel = apperr.ErrInternal
		} else {
			sentinel = apperr.ErrInvalidInput
		}
	}

	return apperr.Wrap(err, sentinel)
}
