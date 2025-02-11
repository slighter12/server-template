package grpc

import (
	"context"
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

type grpcServerParams struct {
	fx.In

	auth usecase.AuthUseCase
	cfg  *config.Config
}

type grpcServer struct {
	fx.In
	authpb.UnimplementedAuthServer
	auth usecase.AuthUseCase
	cfg  *config.Config
}

func NewGRPC(params grpcServerParams) delivery.Delivery {
	return &grpcServer{
		auth: params.auth,
		cfg:  params.cfg,
	}
}

func (s *grpcServer) Serve(lc fx.Lifecycle, ctx context.Context) {
	var opts []grpc.ServerOption
	if s.cfg.Observability.Otel.Enable {
		opts = append(opts, grpc.StatsHandler(otelgrpc.NewServerHandler()))
	}
	grpcServer := grpc.NewServer(opts...)
	registerAuthService(grpcServer, s)

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			lis, err := net.Listen("tcp", s.cfg.RPC.Server.Target)
			if err != nil {
				return errors.Wrap(err, "net.Listen")
			}

			return grpcServer.Serve(lis)
		},
		OnStop: func(ctx context.Context) error {
			grpcServer.GracefulStop()

			return nil
		},
	})
}

func registerAuthService(grpcServer *grpc.Server, s *grpcServer) {
	authpb.RegisterAuthServer(grpcServer, s)
}

func (s *grpcServer) Register(ctx context.Context, in *authpb.RegisterRequest) (*authpb.RegisterResponse, error) {
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
