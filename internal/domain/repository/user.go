package repository

import (
	"context"

	"server-template/internal/domain/entity"
)

//go:generate go build -o generator ../../../cmd/generator/main.go
//go:generate ./generator --source=./user.go --output=../../repository/user.gen.go --interface=UserRepository --package=repository --tracer=user-repo-tracer --template=otel
//go:generate rm generator
type UserRepository interface {
	Create(ctx context.Context, user *entity.User) error
	FindByEmail(ctx context.Context, email string) (*entity.User, error)
}
