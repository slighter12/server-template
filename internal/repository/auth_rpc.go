package repository

import (
	"server-template/internal/domain/repository"
	"server-template/internal/libs/rpc"
	pb "server-template/proto/pb/gen"

	"go.uber.org/fx"
)

type authRPCParams struct {
	fx.In
	*rpc.RPCClients
}

func NewAuthRPC(params authRPCParams) (repository.AuthRPCRepository, error) {
	client, err := params.RPCClients.GetClient(rpc.AuthClient)
	if err != nil {
		return nil, err
	}
	return pb.NewAuthClient(client.Conn()), nil
}
