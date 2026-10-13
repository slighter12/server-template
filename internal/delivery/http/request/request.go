// Package request binds and validates HTTP input. Every failure is an
// *apperr.Error: apperr.ErrInvalidInput for malformed input and
// apperr.ErrValidationFailed with one FieldViolation per failed field.
package request

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"uuid"

	validatorpkg "github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v5"

	"server-template/internal/pkg/apperr"
)

// Bind binds the request (path, query, body) into a new T and validates it.
func Bind[T any](c *echo.Context) (T, error) {
	var v T
	if err := c.Bind(&v); err != nil {
		return v, apperr.Wrap(err, apperr.ErrInvalidInput)
	}
	if err := Validate(c, &v); err != nil {
		var zero T

		return zero, err
	}

	return v, nil
}

// BindRequired binds a JSON body that must be present and validates it.
// An empty body or the literal null is a validation failure.
func BindRequired[T any](c *echo.Context) (*T, error) {
	var payload *T
	if err := c.Bind(&payload); err != nil {
		return nil, apperr.Wrap(err, apperr.ErrInvalidInput)
	}
	if payload == nil {
		return nil, apperr.ErrValidationFailed.WithFields(
			apperr.FieldViolation{Field: "body", Description: "request body is required"},
		)
	}
	if err := Validate(c, payload); err != nil {
		return nil, err
	}

	return payload, nil
}

// BindQuery binds query parameters into a copy of defaults and validates it,
// so absent parameters keep their default values.
func BindQuery[T any](c *echo.Context, defaults T) (T, error) {
	if err := echo.BindQueryParams(c, &defaults); err != nil {
		var zero T

		return zero, apperr.Wrap(err, apperr.ErrInvalidInput)
	}
	if err := Validate(c, &defaults); err != nil {
		var zero T

		return zero, err
	}

	return defaults, nil
}

// Validate runs the registered echo validator on req.
func Validate(c *echo.Context, req any) error {
	err := c.Validate(req)
	if err == nil {
		return nil
	}

	return validationError(err, req)
}

// PathUUID parses the named path parameter as a UUID.
func PathUUID(c *echo.Context, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param(name)))
	if err != nil {
		return uuid.Nil(), apperr.Wrap(err, apperr.ErrInvalidInput).WithFields(
			apperr.FieldViolation{Field: name, Description: name + " must be a valid UUID"},
		)
	}

	return id, nil
}

// Pagination holds page based query parameters.
type Pagination struct {
	Page     int `query:"page" validate:"gte=1"`
	PageSize int `query:"page_size" validate:"gte=1"`
}

// LimitOffset holds limit/offset based query parameters.
type LimitOffset struct {
	Limit  int `query:"limit" validate:"gte=1"`
	Offset int `query:"offset" validate:"gte=0"`
}

// NewPagination returns a Pagination with defaults, for BindQuery.
func NewPagination(page, pageSize int) Pagination {
	return Pagination{Page: page, PageSize: pageSize}
}

// NewLimitOffset returns a LimitOffset with defaults, for BindQuery.
func NewLimitOffset(limit, offset int) LimitOffset {
	return LimitOffset{Limit: limit, Offset: offset}
}

func validationError(err error, req any) error {
	verrs, ok := errors.AsType[validatorpkg.ValidationErrors](err)
	if !ok {
		return apperr.Wrap(err, apperr.ErrValidationFailed)
	}

	fields := make([]apperr.FieldViolation, 0, len(verrs))
	for _, fe := range verrs {
		name := requestFieldName(req, fe)
		fields = append(fields, apperr.FieldViolation{Field: name, Description: violationMessage(name, fe)})
	}

	return apperr.ErrValidationFailed.WithFields(fields...)
}

func violationMessage(name string, fe validatorpkg.FieldError) string {
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", name)
	case "oneof":
		return fmt.Sprintf("%s must be one of [%s]", name, fe.Param())
	case "min", "gte":
		return fmt.Sprintf("%s must be greater than or equal to %s", name, fe.Param())
	case "max", "lte":
		return fmt.Sprintf("%s must be less than or equal to %s", name, fe.Param())
	case "email":
		return fmt.Sprintf("%s must be a valid email address", name)
	case "uuid", "uuid4":
		return fmt.Sprintf("%s must be a valid UUID", name)
	}

	return fmt.Sprintf("%s is invalid", name)
}

// requestFieldName maps a failed field to its json/query/param/form tag name.
func requestFieldName(req any, fe validatorpkg.FieldError) string {
	field, ok := requestStructField(req, fe.StructNamespace())
	if !ok {
		return strings.ToLower(fe.Field())
	}
	for _, tagName := range []string{"json", "query", "param", "form"} {
		if name := tagValue(field, tagName); name != "" {
			return name
		}
	}

	return strings.ToLower(fe.Field())
}

func requestStructField(req any, namespace string) (reflect.StructField, bool) {
	t := reflect.TypeOf(req)
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct || namespace == "" {
		return reflect.StructField{}, false
	}

	path := strings.Split(namespace, ".")
	if path[0] == t.Name() {
		path = path[1:]
	}
	if len(path) == 0 {
		return reflect.StructField{}, false
	}

	var field reflect.StructField
	for _, segment := range path {
		if t.Kind() != reflect.Struct {
			return reflect.StructField{}, false
		}
		name, _, _ := strings.Cut(segment, "[")
		next, ok := t.FieldByName(name)
		if !ok {
			return reflect.StructField{}, false
		}
		field = next
		t = next.Type
		for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
			t = t.Elem()
		}
	}

	return field, true
}

func tagValue(field reflect.StructField, tagName string) string {
	name, _, _ := strings.Cut(field.Tag.Get(tagName), ",")
	if name == "-" {
		return ""
	}

	return name
}
