package usecase

import (
	"context"

	"server-template/internal/domain/entity"
	"server-template/proto/pb/authpb"
)

//go:generate go build -o generator ../../../cmd/generator/main.go
//go:generate ./generator --source=./auth_http.go --output=../../usecase/auth_http.gen.go --interface=AuthHTTPUseCase --package=usecase --tracer=auth-http-usecase-tracer --template=otel --module-name=server-template
//go:generate rm generator
type AuthHTTPUseCase interface {
	Register(ctx context.Context, email, password string) (string, *entity.User, error)
	Login(ctx context.Context, email, password string) (string, *entity.User, error)
	Logout(ctx context.Context, token string) error
	ValidateToken(ctx context.Context, token string) (*authpb.ValidateTokenResponse, error)
}
