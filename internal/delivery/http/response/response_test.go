package response_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"server-template/internal/delivery/http/response"
	"server-template/internal/pkg/apperr"
	"server-template/internal/pkg/observability"
)

func newContext(t *testing.T) (*echo.Context, *httptest.ResponseRecorder) {
	t.Helper()

	e := echo.New()
	e.JSONSerializer = response.NewJSONSerializer()
	req := httptest.NewRequestWithContext(observability.WithCorrelationID(t.Context(), "rid-1"), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	return e.NewContext(req, rec), rec
}

func TestStatusFor(t *testing.T) {
	tests := map[apperr.Kind]int{
		apperr.InvalidArgument:    400,
		apperr.Unauthenticated:    401,
		apperr.PermissionDenied:   403,
		apperr.NotFound:           404,
		apperr.Conflict:           409,
		apperr.FailedPrecondition: 422,
		apperr.TooManyRequests:    429,
		apperr.Unavailable:        503,
		apperr.Internal:           500,
	}
	for kind, want := range tests {
		assert.Equal(t, want, response.StatusFor(kind), kind.String())
	}
}

func TestSuccess(t *testing.T) {
	c, rec := newContext(t)

	require.NoError(t, response.Success(c, http.StatusCreated, map[string]string{"id": "1"}))

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.JSONEq(t, `{"data":{"id":"1"},"meta":{"request_id":"rid-1"}}`, rec.Body.String())
}

func TestSuccessNoContent(t *testing.T) {
	c, rec := newContext(t)

	require.NoError(t, response.Success(c, http.StatusNoContent, nil))

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String())
}

func TestErrorClientFaultKeepsFields(t *testing.T) {
	c, rec := newContext(t)
	err := apperr.ErrValidationFailed.WithFields(apperr.FieldViolation{Field: "email", Description: "email is required"})

	require.NoError(t, response.Error(c, err))

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":{"code":"VALIDATION_FAILED","message":"validation failed","fields":[{"field":"email","description":"email is required"}]},"meta":{"request_id":"rid-1"}}`, rec.Body.String())
}

func TestErrorWithStatus(t *testing.T) {
	tests := []struct {
		name   string
		status int
		want   int
	}{
		{"override wins", http.StatusMethodNotAllowed, http.StatusMethodNotAllowed},
		{"zero falls back to kind", 0, http.StatusBadRequest},
		{"non-error status ignored", http.StatusOK, http.StatusBadRequest},
		{"out of range ignored", 700, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, rec := newContext(t)

			require.NoError(t, response.ErrorWithStatus(c, apperr.ErrValidationFailed, tt.status))

			assert.Equal(t, tt.want, rec.Code)
			assert.Contains(t, rec.Body.String(), `"code":"VALIDATION_FAILED"`)
		})
	}
}

func TestErrorRedaction(t *testing.T) {
	fields := []apperr.FieldViolation{{Field: "x", Description: "secret detail"}}
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		notWant    string
	}{
		{
			name:       "plain error becomes generic 500",
			err:        errors.New("dial tcp 10.0.0.1: connection refused"),
			wantStatus: 500, wantCode: "INTERNAL_ERROR", notWant: "10.0.0.1",
		},
		{
			name:       "internal with custom message and fields",
			err:        apperr.New(apperr.Internal, "DB_BROKEN", "table users corrupt").WithFields(fields...),
			wantStatus: 500, wantCode: "INTERNAL_ERROR", notWant: "corrupt",
		},
		{
			name:       "unavailable",
			err:        apperr.New(apperr.Unavailable, "REDIS_DOWN", "redis at 10.0.0.2 down").WithFields(fields...),
			wantStatus: 503, wantCode: "SERVICE_UNAVAILABLE", notWant: "10.0.0.2",
		},
		{
			name:       "unauthenticated drops fields",
			err:        apperr.ErrInvalidToken.WithFields(fields...),
			wantStatus: 401, wantCode: "INVALID_TOKEN", notWant: "secret detail",
		},
		{
			name:       "forbidden drops fields",
			err:        apperr.ErrForbidden.WithFields(fields...),
			wantStatus: 403, wantCode: "FORBIDDEN", notWant: "secret detail",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, rec := newContext(t)

			require.NoError(t, response.Error(c, tt.err))

			assert.Equal(t, tt.wantStatus, rec.Code)
			assert.Contains(t, rec.Body.String(), `"code":"`+tt.wantCode+`"`)
			assert.NotContains(t, rec.Body.String(), tt.notWant)
			assert.NotContains(t, rec.Body.String(), `"fields"`)
		})
	}
}

func TestJSONSerializerRoundTrip(t *testing.T) {
	type payload struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
		Opt  string   `json:"opt,omitzero"`
	}
	e := echo.New()
	e.JSONSerializer = response.NewJSONSerializer()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, c.JSON(http.StatusOK, payload{Name: "a"}))
	assert.JSONEq(t, `{"name":"a","tags":[]}`, rec.Body.String())

	var got payload
	req = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", stringsReader(`{"name":"b","tags":["x"]}`))
	c = e.NewContext(req, httptest.NewRecorder())
	require.NoError(t, response.NewJSONSerializer().Deserialize(c, &got))
	assert.Equal(t, payload{Name: "b", Tags: []string{"x"}}, got)

	req = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", stringsReader(`{"name":"b","name":"c"}`))
	c = e.NewContext(req, httptest.NewRecorder())
	err := response.NewJSONSerializer().Deserialize(c, &got)
	require.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, echo.StatusCode(err))
}

func stringsReader(s string) *strings.Reader { return strings.NewReader(s) }
