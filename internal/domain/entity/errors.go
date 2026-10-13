package entity

import "server-template/internal/pkg/apperr"

// Feature-specific sentinels for the user/auth example. Wrap them with
// apperr.Wrap(cause, ErrXxx) when a lower-level error is the cause.
var (
	ErrUserNotFound       = apperr.New(apperr.NotFound, "USER_NOT_FOUND", "user not found")
	ErrEmailTaken         = apperr.New(apperr.Conflict, "EMAIL_TAKEN", "email is already registered")
	ErrInvalidCredentials = apperr.New(apperr.Unauthenticated, "INVALID_CREDENTIALS", "invalid email or password")
	ErrWeakPassword       = apperr.New(apperr.InvalidArgument, "WEAK_PASSWORD",
		"password must be at least 8 characters and contain upper, lower case letters and a digit")
)
