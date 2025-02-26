package usecase

import (
	"context"
	"server-template/internal/domain/entity"
	"server-template/proto/pb/authpb"
)

type AuthHTTPUseCase interface {
	Register(ctx context.Context, email, password string) (string, *entity.User, error)
	Login(ctx context.Context, email, password string) (string, *entity.User, error)
	Logout(ctx context.Context, token string) error
	ValidateToken(ctx context.Context, token string) (*authpb.ValidateTokenResponse, error)
}
