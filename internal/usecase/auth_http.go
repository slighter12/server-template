package usecase

import (
	"context"

	"server-template/internal/domain/entity"
	"server-template/internal/domain/repository"
	"server-template/internal/domain/usecase"
	"server-template/proto/pb/authpb"
)

type authHTTPUseCase struct {
	authRPC repository.AuthRPCRepository
}

func NewAuthHTTPUseCase(authRPC repository.AuthRPCRepository) usecase.AuthHTTPUseCase {
	return &authHTTPUseCase{
		authRPC: authRPC,
	}
}

// Errors returned by the RPC client are already *apperr.Error (restored by the
// client interceptor), so they are returned untouched.

func (uc *authHTTPUseCase) Register(ctx context.Context, email, password string) (string, *entity.User, error) {
	resp, err := uc.authRPC.Register(ctx, authpb.RegisterRequest_builder{Email: &email, Password: &password}.Build())
	if err != nil {
		return "", nil, err
	}

	tokenResp, err := uc.authRPC.GenerateToken(ctx, authpb.GenerateTokenRequest_builder{
		UserId: new(resp.GetUser().GetId()),
		Email:  new(resp.GetUser().GetEmail()),
	}.Build())
	if err != nil {
		return "", nil, err
	}

	return tokenResp.GetToken(), userFromPB(resp.GetUser()), nil
}

func (uc *authHTTPUseCase) Login(ctx context.Context, email, password string) (string, *entity.User, error) {
	resp, err := uc.authRPC.Login(ctx, authpb.LoginRequest_builder{Email: &email, Password: &password}.Build())
	if err != nil {
		return "", nil, err
	}

	return resp.GetToken(), userFromPB(resp.GetUser()), nil
}

func (uc *authHTTPUseCase) Logout(ctx context.Context, token string) error {
	_, err := uc.authRPC.Logout(ctx, authpb.LogoutRequest_builder{Token: &token}.Build())

	return err
}

func (uc *authHTTPUseCase) ValidateToken(ctx context.Context, token string) (*authpb.ValidateTokenResponse, error) {
	return uc.authRPC.ValidateToken(ctx, authpb.ValidateTokenRequest_builder{Token: &token}.Build())
}

func userFromPB(u *authpb.User) *entity.User {
	return &entity.User{
		ID:        u.GetId(),
		Email:     u.GetEmail(),
		CreatedAt: u.GetCreatedAt().AsTime(),
	}
}
