package repository

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

const (
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
	pgNotNullViolation    = "23502"
)

// isUniqueConstraintViolation reports whether err is a unique-key violation.
func isUniqueConstraintViolation(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == pgUniqueViolation {
		return true
	}

	msg := strings.ToLower(err.Error())

	return strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, pgUniqueViolation)
}

// isForeignKeyViolation reports whether err is a foreign-key violation.
func isForeignKeyViolation(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrForeignKeyViolated) {
		return true
	}
	pgErr, ok := errors.AsType[*pgconn.PgError](err)

	return ok && pgErr.Code == pgForeignKeyViolation
}

// isNotNullViolation reports whether err is a NOT NULL violation.
func isNotNullViolation(err error) bool {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)

	return ok && pgErr.Code == pgNotNullViolation
}

// violatedConstraintName returns the constraint name reported by PostgreSQL,
// or "" when it is not available.
func violatedConstraintName(err error) string {
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		return pgErr.ConstraintName
	}

	return ""
}

// violatesEmail reports whether err is a unique violation on a constraint
// whose name mentions the email column. When the driver does not report the constraint
// name, the error text is searched instead.
func violatesEmail(err error) bool {
	const column = "email"

	if !isUniqueConstraintViolation(err) {
		return false
	}
	if name := violatedConstraintName(err); name != "" {
		return strings.Contains(strings.ToLower(name), column)
	}

	return strings.Contains(strings.ToLower(err.Error()), column)
}
