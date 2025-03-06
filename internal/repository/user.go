package repository

import (
	"context"

	"server-template/internal/domain/entity"
	"server-template/internal/domain/repository"
	"server-template/internal/repository/gen/query"

	"go.uber.org/fx"
	"gorm.io/gorm"
)

type userRepository struct {
	fx.In
	q *query.Query
}

func NewUserRepository(db *gorm.DB) repository.UserRepository {
	return &userRepository{q: query.Use(db)}
}

func (r *userRepository) Create(ctx context.Context, user *entity.User) error {
	return WrapNoValue(r.q.WithContext(ctx).User.Create(user), "Create")
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*entity.User, error) {
	user, err := r.q.WithContext(ctx).User.Where(r.q.User.Email.Eq(email)).First()

	return WrapResult(user, err, "FindByEmail")
}

func (r *userRepository) FindByID(ctx context.Context, id string) (*entity.User, error) {
	user, err := r.q.WithContext(ctx).User.Where(r.q.User.ID.Eq(id)).First()

	return WrapResult(user, err, "FindByID")
}
