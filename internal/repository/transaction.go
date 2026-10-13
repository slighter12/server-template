package repository

import (
	"context"
	"log/slog"

	"server-template/internal/domain/repository"
	"server-template/internal/pkg/apperr"

	"gorm.io/gorm"
)

type txKey struct{}

// DBFromContext returns the transaction stored in ctx by TransactionManager,
// or fallback when ctx carries none.
func DBFromContext(ctx context.Context, fallback *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok && tx != nil {
		return tx
	}

	return fallback
}

type gormTransactionManager struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewTransactionManager builds the GORM-backed TransactionManager.
func NewTransactionManager(db *gorm.DB, logger *slog.Logger) repository.TransactionManager {
	if logger == nil {
		logger = slog.Default()
	}

	return &gormTransactionManager{db: db, logger: logger}
}

// Execute implements repository.TransactionManager. A nested call reuses the
// outer transaction instead of opening a new one.
func (tm *gormTransactionManager) Execute(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return fn(ctx)
	}

	tx := tm.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return apperr.Wrap(tx.Error, apperr.ErrInternal)
	}

	// Roll back if fn panics, then let the panic continue.
	defer func() {
		if r := recover(); r != nil {
			if err := tx.Rollback().Error; err != nil {
				tm.logger.ErrorContext(ctx, "transaction rollback after panic failed", slog.Any("error", err))
			}
			panic(r)
		}
	}()

	if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		if rbErr := tx.Rollback().Error; rbErr != nil {
			tm.logger.ErrorContext(ctx, "transaction rollback failed", slog.Any("error", rbErr))
		}

		return err //nolint:wrapcheck // preserve the caller's error
	}

	if err := tx.Commit().Error; err != nil {
		return apperr.Wrap(err, apperr.ErrInternal)
	}

	return nil
}
