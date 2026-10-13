package repository

import (
	"context"

	"server-template/internal/domain/entity"
)

//go:generate go run server-template/cmd/generator --source=./user.go --output=../../repository/user.gen.go --interface=UserRepository --package=repository --tracer=user-repo-tracer --template=otel --module-name=server-template
type UserRepository interface {
	Create(ctx context.Context, user *entity.User) error
	FindByEmail(ctx context.Context, email string) (*entity.User, error)
	FindByID(ctx context.Context, id string) (*entity.User, error)
}
