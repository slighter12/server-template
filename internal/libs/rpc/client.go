package rpc

import (
	"context"
	"fmt"

	"server-template/config"

	"github.com/pkg/errors"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// clientKey 定義支持的 RPC 客戶端類型
type clientKey string

const (
	AuthClient clientKey = "auth"
)

// RPCClients 包含所有 RPC 客戶端
type RPCClients struct {
	clients map[clientKey]*grpc.ClientConn
}

type rpcClientsParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Config    *config.Config
}

func NewRPCClients(params rpcClientsParams) (*RPCClients, error) {
	rpcClients := &RPCClients{
		clients: make(map[clientKey]*grpc.ClientConn),
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	if params.Config.Observability.Otel.Enable {
		opts = append(opts, grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
	}

	// 遍歷配置創建客戶端
	for clientName, clientConfig := range params.Config.RPC.Clients {
		clientConn, err := grpc.NewClient(clientConfig.Target, opts...)

		if err != nil {
			return nil, errors.WithStack(err)
		}

		rpcClients.clients[clientKey(clientName)] = clientConn
	}

	// 註冊生命週期鉤子
	params.Lifecycle.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			for name, client := range rpcClients.clients {
				if err := client.Close(); err != nil {
					return errors.Wrapf(err, "failed to close RPC client: %s", name)
				}
			}

			return nil
		},
	})

	return rpcClients, nil
}

// GetClient 獲取指定的 RPC 客戶端
func (r *RPCClients) GetClient(key clientKey) (*grpc.ClientConn, error) {
	client, ok := r.clients[key]
	if !ok {
		return nil, fmt.Errorf("RPC client not found: %s", key)
	}

	return client, nil
}
