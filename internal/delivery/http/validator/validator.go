// Package validator adapts go-playground/validator to echo.Validator.
package validator

import (
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v5"
)

// CustomValidator implements echo.Validator.
type CustomValidator struct {
	validator *validator.Validate
}

var _ echo.Validator = (*CustomValidator)(nil)

// New returns a validator using the default go-playground rules.
func New() *CustomValidator {
	return &CustomValidator{validator: validator.New()}
}

// Validate validates a struct and returns validator.ValidationErrors on failure.
func (cv *CustomValidator) Validate(i any) error {
	return cv.validator.Struct(i)
}
