package usecase

import (
	"context"

	"server-template/internal/domain/entity"
)

//go:generate go run server-template/cmd/generator --source=./auth.go --output=../../usecase/auth.gen.go --interface=AuthUseCase --package=usecase --tracer=auth-usecase-tracer --template=otel --module-name=server-template
type AuthUseCase interface {
	Register(ctx context.Context, email string, password string) (*entity.User, error)
	Login(ctx context.Context, email string, password string) (*entity.User, error)
	GetUserByID(ctx context.Context, userID string) (*entity.User, error)
}
