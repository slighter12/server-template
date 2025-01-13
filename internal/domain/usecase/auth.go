package usecase

import (
	"context"
	"server-template/internal/domain/entity"
)

//go:generate go run main.go --source=quote_usecase.go --output=quote_usecase.gen.go --interface=QuoteUseCase --package=usecase --tracer=quote-tracer --template=otel
type AuthUseCase interface {
	Register(ctx context.Context, email string, hashedPassword string) (*entity.User, error)
	Login(ctx context.Context, email string, hashedPassword string) (*entity.User, error)
}
