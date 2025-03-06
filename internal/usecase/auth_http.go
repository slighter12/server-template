package usecase

import (
	"context"

	"server-template/internal/domain/entity"
	"server-template/internal/domain/repository"
	"server-template/internal/domain/usecase"
	"server-template/proto/pb/authpb"

	"github.com/pkg/errors"
)

type authHTTPUseCase struct {
	authRPC repository.AuthRPCRepository
}

func NewAuthHTTPUseCase(authRPC repository.AuthRPCRepository) usecase.AuthHTTPUseCase {
	return &authHTTPUseCase{
		authRPC: authRPC,
	}
}

func (uc *authHTTPUseCase) Register(ctx context.Context, email, password string) (string, *entity.User, error) {
	// 創建 gRPC 請求
	grpcReq := &authpb.RegisterRequest{}
	grpcReq.SetEmail(email)
	grpcReq.SetPassword(password)

	// 調用 gRPC 服務
	resp, err := uc.authRPC.Register(ctx, grpcReq)
	if err != nil {
		return "", nil, errors.Wrap(err, "failed to register user")
	}

	// 檢查 gRPC 響應狀態
	if resp.GetStatus().GetCode() != int32(0) {
		return "", nil, errors.New(resp.GetStatus().GetMessage())
	}

	// 通過 gRPC 生成 token
	tokenReq := &authpb.GenerateTokenRequest{}
	tokenReq.SetUserId(resp.GetUser().GetId())
	tokenReq.SetEmail(resp.GetUser().GetEmail())

	tokenResp, err := uc.authRPC.GenerateToken(ctx, tokenReq)
	if err != nil {
		return "", nil, errors.Wrap(err, "failed to generate token")
	}

	// 創建用戶實體
	user := &entity.User{
		ID:        resp.GetUser().GetId(),
		Email:     resp.GetUser().GetEmail(),
		CreatedAt: resp.GetUser().GetCreatedAt().AsTime(),
	}

	return tokenResp.GetToken(), user, nil
}

func (uc *authHTTPUseCase) Login(ctx context.Context, email, password string) (string, *entity.User, error) {
	// 創建 gRPC 請求
	grpcReq := &authpb.LoginRequest{}
	grpcReq.SetEmail(email)
	grpcReq.SetPassword(password)

	// 調用 gRPC 服務
	resp, err := uc.authRPC.Login(ctx, grpcReq)
	if err != nil {
		return "", nil, errors.Wrap(err, "failed to login user")
	}

	// 檢查 gRPC 響應狀態
	if resp.GetStatus().GetCode() != int32(0) {
		return "", nil, errors.New(resp.GetStatus().GetMessage())
	}

	// 創建用戶實體
	user := &entity.User{
		ID:        resp.GetUser().GetId(),
		Email:     resp.GetUser().GetEmail(),
		CreatedAt: resp.GetUser().GetCreatedAt().AsTime(),
	}

	return resp.GetToken(), user, nil
}

func (uc *authHTTPUseCase) Logout(ctx context.Context, token string) error {
	// 創建 gRPC 請求
	grpcReq := &authpb.LogoutRequest{}
	grpcReq.SetToken(token)

	// 調用 gRPC 服務
	resp, err := uc.authRPC.Logout(ctx, grpcReq)
	if err != nil {
		return errors.Wrap(err, "failed to logout user")
	}

	// 檢查 gRPC 響應狀態
	if resp.GetStatus().GetCode() != int32(0) {
		return errors.New(resp.GetStatus().GetMessage())
	}

	return nil
}

func (uc *authHTTPUseCase) ValidateToken(ctx context.Context, token string) (*authpb.ValidateTokenResponse, error) {
	// 創建 gRPC 請求
	grpcReq := &authpb.ValidateTokenRequest{}
	grpcReq.SetToken(token)

	// 調用 gRPC 服務
	resp, err := uc.authRPC.ValidateToken(ctx, grpcReq)
	if err != nil {
		return nil, errors.Wrap(err, "failed to validate token")
	}

	return resp, nil
}
