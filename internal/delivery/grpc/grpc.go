package grpc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"server-template/config"
	"server-template/internal/domain/delivery"
	"server-template/internal/domain/usecase"
	"server-template/internal/pkg/apperr"
	"server-template/proto/pb/authpb"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const gracefulStopTimeout = 10 * time.Second

type gRPCServer struct {
	authpb.UnimplementedAuthServer
	auth       usecase.AuthUseCase
	cfg        *config.Config
	grpcServer *grpc.Server
	health     *health.Server
	logger     *slog.Logger
	tokens     *tokenManager
}

// NewGRPC builds the gRPC delivery. Redis (token store and revocation list) is mandatory.
func NewGRPC(lc fx.Lifecycle, auth usecase.AuthUseCase, cfg *config.Config, logger *slog.Logger, rdb redis.Cmdable) (delivery.Delivery, error) {
	server := newServer(auth, cfg, logger, rdb)

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			server.stop(ctx)

			return nil
		},
	})

	return server, nil
}

func newServer(auth usecase.AuthUseCase, cfg *config.Config, logger *slog.Logger, rdb redis.Cmdable) *gRPCServer {
	opts := []grpc.ServerOption{grpc.ChainUnaryInterceptor(ErrorInterceptor(logger))}
	if cfg.Observability.Otel.Enable {
		opts = append(opts, grpc.StatsHandler(otelgrpc.NewServerHandler()))
	}
	grpcServer := grpc.NewServer(opts...)

	server := &gRPCServer{
		auth:       auth,
		cfg:        cfg,
		grpcServer: grpcServer,
		health:     health.NewServer(),
		logger:     logger,
		tokens:     newTokenManager(cfg.Auth.JWTSecret, cfg.Auth.AccessTokenTTL, rdb, logger),
	}

	authpb.RegisterAuthServer(grpcServer, server)
	healthpb.RegisterHealthServer(grpcServer, server.health)
	if cfg.GRPC.Reflection && cfg.Env.Env != "production" {
		reflection.Register(grpcServer)
	}

	return server
}

// Serve listens on the configured gRPC port and blocks until the server stops.
func (s *gRPCServer) Serve(ctx context.Context) error {
	var listenConfig net.ListenConfig

	addr := fmt.Sprintf(":%d", s.cfg.GRPC.Port)
	lis, err := listenConfig.Listen(ctx, "tcp", addr)
	if err != nil {
		return apperr.Wrap(fmt.Errorf("listen %s: %w", addr, err), apperr.ErrInternal)
	}

	s.logger.InfoContext(ctx, "Starting gRPC server", slog.String("addr", addr))

	return s.serve(lis)
}

func (s *gRPCServer) serve(lis net.Listener) error {
	s.health.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	if err := s.grpcServer.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		return fmt.Errorf("serve gRPC: %w", err)
	}

	return nil
}

// stop drains in-flight RPCs, falling back to a hard Stop on timeout.
func (s *gRPCServer) stop(ctx context.Context) {
	s.logger.InfoContext(ctx, "Stopping gRPC server")
	s.health.Shutdown() // sets every service NOT_SERVING

	done := make(chan struct{})
	go func() {
		s.grpcServer.GracefulStop()
		close(done)
	}()

	timer := time.NewTimer(gracefulStopTimeout)
	defer timer.Stop()

	select {
	case <-done:
	case <-timer.C:
		s.grpcServer.Stop()
	case <-ctx.Done():
		s.grpcServer.Stop()
	}
}

func okStatus(msg string) *authpb.Status {
	return authpb.Status_builder{Code: new(int32(codes.OK)), Message: &msg}.Build()
}

func userPB(id, email string, createdAt time.Time) *authpb.User {
	return authpb.User_builder{Id: &id, Email: &email, CreatedAt: timestamppb.New(createdAt)}.Build()
}

func (s *gRPCServer) Register(ctx context.Context, in *authpb.RegisterRequest) (*authpb.RegisterResponse, error) {
	user, err := s.auth.Register(ctx, in.GetEmail(), in.GetPassword())
	if err != nil {
		return nil, err
	}

	return authpb.RegisterResponse_builder{
		Status: okStatus("Register successful"),
		User:   userPB(user.ID, user.Email, user.CreatedAt),
	}.Build(), nil
}

func (s *gRPCServer) Login(ctx context.Context, in *authpb.LoginRequest) (*authpb.LoginResponse, error) {
	user, err := s.auth.Login(ctx, in.GetEmail(), in.GetPassword())
	if err != nil {
		return nil, err
	}

	token, err := s.tokens.issue(ctx, user.ID, user.Email)
	if err != nil {
		return nil, err
	}

	return authpb.LoginResponse_builder{
		Status: okStatus("Login successful"),
		User:   userPB(user.ID, user.Email, user.CreatedAt),
		Token:  &token,
	}.Build(), nil
}

func (s *gRPCServer) Logout(ctx context.Context, in *authpb.LogoutRequest) (*authpb.LogoutResponse, error) {
	if err := s.tokens.revoke(ctx, in.GetToken()); err != nil {
		return nil, err
	}

	return authpb.LogoutResponse_builder{Status: okStatus("Logout successful")}.Build(), nil
}

func (s *gRPCServer) GenerateToken(ctx context.Context, in *authpb.GenerateTokenRequest) (*authpb.GenerateTokenResponse, error) {
	token, err := s.tokens.issue(ctx, in.GetUserId(), in.GetEmail())
	if err != nil {
		return nil, err
	}

	return authpb.GenerateTokenResponse_builder{
		Status: okStatus("Token generated successfully"),
		Token:  &token,
	}.Build(), nil
}

func (s *gRPCServer) ValidateToken(ctx context.Context, in *authpb.ValidateTokenRequest) (*authpb.ValidateTokenResponse, error) {
	revoked, err := s.tokens.revoked(ctx, in.GetToken())
	if err != nil {
		return nil, err
	}
	if revoked {
		return nil, apperr.ErrInvalidToken
	}

	claims, err := s.tokens.parse(in.GetToken())
	if err != nil {
		return nil, err
	}

	user, err := s.auth.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}

	return authpb.ValidateTokenResponse_builder{
		Status: okStatus("Token is valid"),
		User:   userPB(user.ID, user.Email, user.CreatedAt),
	}.Build(), nil
}
