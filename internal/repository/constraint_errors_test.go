package repository

import (
	"errors"
	"fmt"
	"testing"

	"server-template/internal/domain/entity"
	"server-template/internal/pkg/apperr"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestConstraintHelpers(t *testing.T) {
	uniq := fmt.Errorf("insert: %w", &pgconn.PgError{Code: "23505", ConstraintName: "udx_email"})

	assert.True(t, isUniqueConstraintViolation(uniq))
	assert.True(t, isUniqueConstraintViolation(gorm.ErrDuplicatedKey))
	assert.True(t, isUniqueConstraintViolation(errors.New(`ERROR: duplicate key value violates unique constraint "x"`)))
	assert.False(t, isUniqueConstraintViolation(errors.New("connection refused")))
	assert.False(t, isUniqueConstraintViolation(nil))

	assert.Equal(t, "udx_email", violatedConstraintName(uniq))
	assert.Empty(t, violatedConstraintName(errors.New("x")))

	assert.True(t, violatesEmail(uniq))
	assert.False(t, violatesEmail(&pgconn.PgError{Code: "23505", ConstraintName: "users_pkey"}))
	assert.True(t, violatesEmail(errors.New(`duplicate key value violates unique constraint "users_email_key"`)))

	assert.True(t, isForeignKeyViolation(&pgconn.PgError{Code: "23503"}))
	assert.True(t, isForeignKeyViolation(gorm.ErrForeignKeyViolated))
	assert.True(t, isNotNullViolation(&pgconn.PgError{Code: "23502"}))
	assert.False(t, isNotNullViolation(errors.New("x")))
}

func TestMapFindError(t *testing.T) {
	require.NoError(t, mapFindError(nil))

	err := mapFindError(gorm.ErrRecordNotFound)
	require.ErrorIs(t, err, entity.ErrUserNotFound)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)

	err = mapFindError(errors.New("db down"))
	require.ErrorIs(t, err, apperr.ErrInternal)
	assert.NotErrorIs(t, err, entity.ErrUserNotFound)
}
