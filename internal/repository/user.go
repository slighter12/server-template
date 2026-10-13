package repository

import (
	"context"
	"errors"

	"server-template/internal/domain/entity"
	"server-template/internal/domain/repository"
	"server-template/internal/pkg/apperr"
	"server-template/internal/repository/gen/query"

	"gorm.io/gorm"
)

type userRepository struct {
	db *gorm.DB
}

// NewUserRepository builds the GORM user repository. Methods use the
// transaction carried by ctx (see TransactionManager) when there is one,
// otherwise db. A nil db is an error.
func NewUserRepository(db *gorm.DB) (repository.UserRepository, error) {
	if db == nil {
		return nil, errors.New("user repository: database connection is required")
	}

	return &userRepository{db: db}, nil
}

func (r *userRepository) Create(ctx context.Context, user *entity.User) error {
	q := query.Use(DBFromContext(ctx, r.db))
	err := q.WithContext(ctx).User.Create(user)
	switch {
	case err == nil:
		return nil
	case violatesEmail(err):
		return apperr.Wrap(err, entity.ErrEmailTaken)
	case isUniqueConstraintViolation(err):
		return apperr.Wrap(err, apperr.ErrConflict)
	default:
		return apperr.Wrap(err, apperr.ErrInternal)
	}
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*entity.User, error) {
	q := query.Use(DBFromContext(ctx, r.db))
	user, err := q.WithContext(ctx).User.Where(q.User.Email.Eq(email)).First()

	return user, mapFindError(err)
}

func (r *userRepository) FindByID(ctx context.Context, id string) (*entity.User, error) {
	q := query.Use(DBFromContext(ctx, r.db))
	user, err := q.WithContext(ctx).User.Where(q.User.ID.Eq(id)).First()

	return user, mapFindError(err)
}

func mapFindError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return apperr.Wrap(err, entity.ErrUserNotFound)
	default:
		return apperr.Wrap(err, apperr.ErrInternal)
	}
}
