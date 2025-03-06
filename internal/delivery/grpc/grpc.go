package grpc

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"server-template/config"
	"server-template/internal/domain/delivery"
	"server-template/internal/domain/usecase"
	"server-template/proto/pb/authpb"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type gRPCServer struct {
	authpb.UnimplementedAuthServer
	auth       usecase.AuthUseCase
	cfg        *config.Config
	grpcServer *grpc.Server
	logger     *slog.Logger
	redis      *redis.Client
}

func NewGRPC(lc fx.Lifecycle, auth usecase.AuthUseCase, cfg *config.Config, logger *slog.Logger, redis *redis.Client) (delivery.Delivery, error) {
	var opts []grpc.ServerOption
	if cfg.Observability.Otel.Enable {
		opts = append(opts, grpc.StatsHandler(otelgrpc.NewServerHandler()))
	}
	grpcServer := grpc.NewServer(opts...)

	server := &gRPCServer{
		auth:       auth,
		cfg:        cfg,
		grpcServer: grpcServer,
		logger:     logger,
		redis:      redis,
	}

	authpb.RegisterAuthServer(grpcServer, server)

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			slog.Info("Stopping gRPC server")
			grpcServer.GracefulStop()

			return nil
		},
	})

	return server, nil
}

func (s *gRPCServer) Serve(ctx context.Context) error {
	lis, err := net.Listen("tcp", s.cfg.RPC.Server.Target)
	if err != nil {
		slog.Error("Failed to listen", slog.Any("error", err))

		return errors.Wrap(err, "failed to listen")
	}

	slog.Info("Starting gRPC server", slog.Any("target", s.cfg.RPC.Server.Target))
	if err := s.grpcServer.Serve(lis); err != nil {
		return errors.Wrap(err, "failed to serve gRPC")
	}

	return nil
}

func (s *gRPCServer) Register(ctx context.Context, in *authpb.RegisterRequest) (*authpb.RegisterResponse, error) {
	user, err := s.auth.Register(ctx, in.GetEmail(), in.GetPassword())
	if err != nil {
		return nil, errors.Wrap(err, "auth.Register")
	}

	resp := new(authpb.RegisterResponse)
	resp.GetStatus().SetCode(int32(codes.OK))
	resp.GetStatus().SetMessage("Register successful")
	resp.GetUser().SetId(user.ID)
	resp.GetUser().SetEmail(user.Email)
	resp.GetUser().SetCreatedAt(timestamppb.New(user.CreatedAt))

	return resp, nil
}

func (s *gRPCServer) Login(ctx context.Context, in *authpb.LoginRequest) (*authpb.LoginResponse, error) {
	user, err := s.auth.Login(ctx, in.GetEmail(), in.GetPassword())
	if err != nil {
		return nil, errors.Wrap(err, "auth.Login")
	}

	// 生成 token
	token, err := s.generateToken(ctx, user.ID, user.Email)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate token")
	}

	resp := new(authpb.LoginResponse)
	resp.GetStatus().SetCode(int32(codes.OK))
	resp.GetStatus().SetMessage("Login successful")
	resp.GetUser().SetId(user.ID)
	resp.GetUser().SetEmail(user.Email)
	resp.GetUser().SetCreatedAt(timestamppb.New(user.CreatedAt))
	resp.SetToken(token)

	return resp, nil
}

func (s *gRPCServer) Logout(ctx context.Context, in *authpb.LogoutRequest) (*authpb.LogoutResponse, error) {
	// 將 token 加入黑名單
	err := s.invalidateToken(ctx, in.GetToken())
	if err != nil {
		return nil, errors.Wrap(err, "failed to invalidate token")
	}

	resp := new(authpb.LogoutResponse)
	resp.GetStatus().SetCode(int32(codes.OK))
	resp.GetStatus().SetMessage("Logout successful")

	return resp, nil
}

func (s *gRPCServer) GenerateToken(ctx context.Context, in *authpb.GenerateTokenRequest) (*authpb.GenerateTokenResponse, error) {
	token, err := s.generateToken(ctx, in.GetUserId(), in.GetEmail())
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate token")
	}

	resp := new(authpb.GenerateTokenResponse)
	resp.GetStatus().SetCode(int32(codes.OK))
	resp.GetStatus().SetMessage("Token generated successfully")
	resp.SetToken(token)

	return resp, nil
}

func (s *gRPCServer) ValidateToken(ctx context.Context, in *authpb.ValidateTokenRequest) (*authpb.ValidateTokenResponse, error) {
	// 檢查 token 是否在黑名單中
	isInvalid, err := s.isTokenInvalid(ctx, in.GetToken())
	if err != nil {
		return nil, errors.Wrap(err, "failed to check token validity")
	}

	if isInvalid {
		resp := new(authpb.ValidateTokenResponse)
		resp.GetStatus().SetCode(int32(codes.Unauthenticated))
		resp.GetStatus().SetMessage("Token is invalid or has been revoked")

		return resp, nil
	}

	// 解析 token
	claims, err := s.parseToken(in.GetToken())
	if err != nil {
		return nil, errors.Wrap(err, "invalid token")
	}

	// 獲取用戶信息
	user, err := s.auth.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get user")
	}

	resp := new(authpb.ValidateTokenResponse)
	resp.GetStatus().SetCode(int32(codes.OK))
	resp.GetStatus().SetMessage("Token is valid")
	resp.GetUser().SetId(user.ID)
	resp.GetUser().SetEmail(user.Email)
	resp.GetUser().SetCreatedAt(timestamppb.New(user.CreatedAt))

	return resp, nil
}

// Claims 定義 JWT 的聲明
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// generateToken 生成 JWT token 並存儲在 Redis 中
func (s *gRPCServer) generateToken(ctx context.Context, userID, email string) (string, error) {
	// 生成 token
	expirationTime := time.Now().Add(24 * time.Hour) // Token 有效期為 24 小時
	claims := &Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.cfg.Auth.JWTSecret))
	if err != nil {
		return "", errors.Wrap(err, "failed to sign token")
	}

	// 將 token 存儲在 Redis 中
	key := fmt.Sprintf("token:%s", tokenString)
	err = s.redis.Set(ctx, key, userID, 24*time.Hour).Err()
	if err != nil {
		return "", errors.Wrap(err, "failed to store token in Redis")
	}

	return tokenString, nil
}

// invalidateToken 將 token 加入黑名單
func (s *gRPCServer) invalidateToken(ctx context.Context, tokenString string) error {
	// 解析 token 以獲取過期時間
	claims, err := s.parseToken(tokenString)
	if err != nil {
		return errors.Wrap(err, "failed to parse token")
	}

	// 計算 token 剩餘有效期
	expirationTime := claims.ExpiresAt.Time
	ttl := time.Until(expirationTime)
	if ttl <= 0 {
		// token 已過期，無需加入黑名單
		return nil
	}

	// 將 token 加入黑名單
	blacklistKey := fmt.Sprintf("blacklist:%s", tokenString)
	err = s.redis.Set(ctx, blacklistKey, "1", ttl).Err()
	if err != nil {
		return errors.Wrap(err, "failed to add token to blacklist")
	}

	// 刪除原有的 token 記錄
	tokenKey := fmt.Sprintf("token:%s", tokenString)
	err = s.redis.Del(ctx, tokenKey).Err()
	if err != nil {
		s.logger.Warn("Failed to delete token from Redis", slog.Any("error", err))
		// 繼續執行，不返回錯誤
	}

	return nil
}

// isTokenInvalid 檢查 token 是否在黑名單中
func (s *gRPCServer) isTokenInvalid(ctx context.Context, tokenString string) (bool, error) {
	blacklistKey := fmt.Sprintf("blacklist:%s", tokenString)
	exists, err := s.redis.Exists(ctx, blacklistKey).Result()
	if err != nil {
		return false, errors.Wrap(err, "failed to check token in blacklist")
	}

	return exists > 0, nil
}

// parseToken 解析 JWT token
func (s *gRPCServer) parseToken(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(s.cfg.Auth.JWTSecret), nil
	})

	if err != nil || !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}
