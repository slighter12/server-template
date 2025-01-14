package repository

import (
	"server-template/internal/domain/repository"
	"server-template/internal/libs/rpc"
	"server-template/proto/pb/authpb"

	"github.com/pkg/errors"
)

func NewAuthRPC(rpcClients *rpc.RPCClients) (repository.AuthRPCRepository, error) {
	client, err := rpcClients.GetClient(rpc.AuthClient)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return authpb.NewAuthClient(client), nil
}
