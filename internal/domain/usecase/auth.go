package usecase

import (
	"context"

	"server-template/internal/domain/entity"
)

//go:generate go build -o generator ../../../cmd/generator/main.go
//go:generate ./generator --source=./auth.go --output=../../usecase/auth.gen.go --interface=AuthUseCase --package=usecase --tracer=auth-usecase-tracer --template=otel
//go:generate rm generator
type AuthUseCase interface {
	Register(ctx context.Context, email string, hashedPassword string) (*entity.User, error)
	Login(ctx context.Context, email string, hashedPassword string) (*entity.User, error)
	GetUserByID(ctx context.Context, userID string) (*entity.User, error)
}
