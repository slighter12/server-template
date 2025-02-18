package grpc

import (
	"context"
	"log/slog"
	"net"

	"server-template/config"
	"server-template/internal/domain/delivery"
	"server-template/internal/domain/usecase"
	"server-template/proto/pb/authpb"

	"github.com/pkg/errors"
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
}

func NewGRPC(lc fx.Lifecycle, auth usecase.AuthUseCase, cfg *config.Config) (delivery.Delivery, error) {
	var opts []grpc.ServerOption
	if cfg.Observability.Otel.Enable {
		opts = append(opts, grpc.StatsHandler(otelgrpc.NewServerHandler()))
	}
	grpcServer := grpc.NewServer(opts...)

	server := &gRPCServer{
		auth:       auth,
		cfg:        cfg,
		grpcServer: grpcServer,
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
