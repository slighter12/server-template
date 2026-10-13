package request_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"server-template/internal/delivery/http/request"
	"server-template/internal/delivery/http/response"
	"server-template/internal/delivery/http/validator"
	"server-template/internal/pkg/apperr"
)

type address struct {
	City string `json:"city" validate:"required"`
}

type payload struct {
	Name   string    `json:"name" validate:"required"`
	Email  string    `json:"email" validate:"required,email"`
	Role   string    `json:"role" validate:"omitempty,oneof=admin user"`
	Age    int       `json:"age" validate:"gte=18"`
	Addr   address   `json:"address" validate:"required"`
	Others []address `json:"others" validate:"dive"`
}

func newContext(t *testing.T, method, target, body string) *echo.Context {
	t.Helper()

	e := echo.New()
	e.Validator = validator.New()
	e.JSONSerializer = response.NewJSONSerializer()
	req := httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	return e.NewContext(req, httptest.NewRecorder())
}

func violations(t *testing.T, err error) map[string]string {
	t.Helper()

	ae := apperr.From(err)
	require.ErrorIs(t, err, apperr.ErrValidationFailed)
	out := make(map[string]string, len(ae.Fields))
	for _, f := range ae.Fields {
		out[f.Field] = f.Description
	}

	return out
}

func TestBind_ReportsOneViolationPerFieldWithJSONNames(t *testing.T) {
	c := newContext(t, http.MethodPost, "/", `{"email":"nope","role":"root","age":3,"others":[{}]}`)

	_, err := request.Bind[payload](c)

	got := violations(t, err)
	assert.Equal(t, "name is required", got["name"])
	assert.Equal(t, "email must be a valid email address", got["email"])
	assert.Equal(t, "role must be one of [admin user]", got["role"])
	assert.Equal(t, "age must be greater than or equal to 18", got["age"])
	assert.Equal(t, "city is required", got["city"])
	assert.Len(t, got, 5)
}

func TestBind_Success(t *testing.T) {
	c := newContext(t, http.MethodPost, "/", `{"name":"a","email":"a@b.co","age":20,"address":{"city":"x"}}`)

	got, err := request.Bind[payload](c)

	require.NoError(t, err)
	assert.Equal(t, "a", got.Name)
}

func TestBind_MalformedBodyIsInvalidInput(t *testing.T) {
	c := newContext(t, http.MethodPost, "/", `{"name":`)

	_, err := request.Bind[payload](c)

	require.ErrorIs(t, err, apperr.ErrInvalidInput)
	assert.Equal(t, apperr.InvalidArgument, apperr.From(err).Kind)
	assert.NotNil(t, apperr.From(err).Cause())
}

func TestBindRequired(t *testing.T) {
	t.Run("null body", func(t *testing.T) {
		c := newContext(t, http.MethodPost, "/", `null`)

		got, err := request.BindRequired[payload](c)

		assert.Nil(t, got)
		assert.Equal(t, "request body is required", violations(t, err)["body"])
	})
	t.Run("validates", func(t *testing.T) {
		c := newContext(t, http.MethodPost, "/", `{}`)

		_, err := request.BindRequired[payload](c)

		assert.Equal(t, "name is required", violations(t, err)["name"])
	})
}

func TestBindQuery_KeepsDefaults(t *testing.T) {
	c := newContext(t, http.MethodGet, "/?page_size=50", "")

	got, err := request.BindQuery(c, request.NewPagination(1, 20))

	require.NoError(t, err)
	assert.Equal(t, request.Pagination{Page: 1, PageSize: 50}, got)
}

func TestBindQuery_Validation(t *testing.T) {
	c := newContext(t, http.MethodGet, "/?page=0", "")

	_, err := request.BindQuery(c, request.NewPagination(1, 20))

	assert.Equal(t, "page must be greater than or equal to 1", violations(t, err)["page"])
}

func TestBindQuery_BadType(t *testing.T) {
	c := newContext(t, http.MethodGet, "/?limit=abc", "")

	_, err := request.BindQuery(c, request.NewLimitOffset(10, 0))

	require.ErrorIs(t, err, apperr.ErrInvalidInput)
}

func TestPathUUID(t *testing.T) {
	id := uuid.NewV7()
	c := newContext(t, http.MethodGet, "/", "")
	c.SetPathValues(echo.PathValues{{Name: "id", Value: id.String()}})

	got, err := request.PathUUID(c, "id")
	require.NoError(t, err)
	assert.Equal(t, id, got)

	c.SetPathValues(echo.PathValues{{Name: "id", Value: "bad"}})
	_, err = request.PathUUID(c, "id")
	require.ErrorIs(t, err, apperr.ErrInvalidInput)
	assert.Equal(t, "id", apperr.From(err).Fields[0].Field)
}
