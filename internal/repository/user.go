package repository

import (
	"context"

	"server-template/config"
	"server-template/internal/domain/entity"
	"server-template/internal/domain/repository"

	"go.uber.org/fx"
	"gorm.io/gorm"
)

type userRepository struct {
	fx.In

	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) repository.UserRepository {
	return &userRepository{db: db}
}

func ProvideUserRepository(cfg *config.Config, db *gorm.DB) any {
	impl := NewUserRepository(db)

	// If tracing is disabled, return the implementation directly
	if !cfg.Observability.Otel.Enable {
		return impl
	}

	// If tracing is enabled, return the annotated version
	return fx.Annotate(
		impl,
		fx.ResultTags(`name:"original"`),
	)
}

func (r *userRepository) Create(ctx context.Context, user *entity.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*entity.User, error) {
	var user entity.User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}

	return &user, nil
}
