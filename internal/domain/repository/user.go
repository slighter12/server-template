package repository

import (
	"context"
	"server-template/internal/domain/entity"
)

//go:generate go run main.go --source=user.go --output=user.gen.go --interface=QuoteUseCase --package=usecase --tracer=quote-tracer --template=otel
type UserRepository interface {
	Create(ctx context.Context, user *entity.User) error
	FindByEmail(ctx context.Context, email string) (*entity.User, error)
}
