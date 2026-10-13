package apperr

// Common sentinels shared by every service. Feature-specific sentinels live
// next to the feature (for example in internal/domain).
var (
	ErrInternal         = New(Internal, "INTERNAL_ERROR", "internal server error")
	ErrUnavailable      = New(Unavailable, "SERVICE_UNAVAILABLE", "service temporarily unavailable")
	ErrInvalidInput     = New(InvalidArgument, "INVALID_INPUT", "invalid input")
	ErrValidationFailed = New(InvalidArgument, "VALIDATION_FAILED", "validation failed")
	ErrNotFound         = New(NotFound, "NOT_FOUND", "resource not found")
	ErrConflict         = New(Conflict, "CONFLICT", "resource conflict")
	ErrUnauthenticated  = New(Unauthenticated, "UNAUTHENTICATED", "authentication required")
	ErrInvalidToken     = New(Unauthenticated, "INVALID_TOKEN", "invalid or expired token")
	ErrForbidden        = New(PermissionDenied, "FORBIDDEN", "access denied")
	ErrTooManyRequests  = New(TooManyRequests, "TOO_MANY_REQUESTS", "too many requests")
)
