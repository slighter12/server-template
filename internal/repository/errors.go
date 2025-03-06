package repository

import (
	"fmt"

	"github.com/pkg/errors"
)

// Wrap wraps an error with a descriptive message and the operation name
func Wrap[T any](result T, err error, operation string) (T, error) {
	if err == nil {
		return result, nil
	}

	return result, errors.Wrapf(err, "repository.%s failed", operation)
}

// WrapNoValue wraps an error with a descriptive message and the operation name for operations that don't return a value
func WrapNoValue(err error, operation string) error {
	if err == nil {
		return nil
	}

	return errors.Wrapf(err, "repository.%s failed", operation)
}

// WrapWithValue wraps an error with a descriptive message, operation name, and additional context
func WrapWithValue[T any](result T, err error, operation string, format string, args ...interface{}) (T, error) {
	if err == nil {
		return result, nil
	}

	return result, errors.Wrapf(err, fmt.Sprintf("repository.%s failed: %s", operation, format), args...)
}

// WrapResult wraps a result and error with a descriptive message and the operation name
func WrapResult[T any](result T, err error, operation string) (T, error) {
	if err == nil {
		return result, nil
	}

	return result, errors.Wrapf(err, "repository.%s failed", operation)
}
