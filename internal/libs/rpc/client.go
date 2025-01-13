package rpc

import (
	"context"
	"fmt"
	"server-template/config"

	"strings"

	"github.com/pkg/errors"
	"github.com/zeromicro/go-zero/zrpc"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// clientKey 定義支持的 RPC 客戶端類型
type clientKey string

const (
	AuthClient clientKey = "auth"
	// 可以添加更多客戶端類型
)

// RPCClients 包含所有 RPC 客戶端
type RPCClients struct {
	clients map[clientKey]zrpc.Client

	// Auth pb.AuthClient
}

type rpcClientsParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Config    *config.Config
}

func NewRPCClients(params rpcClientsParams) (*RPCClients, error) {
	rpcClients := &RPCClients{
		clients: make(map[clientKey]zrpc.Client),
	}

	// 遍歷配置創建客戶端
	for clientName, clientConfig := range params.Config.RPC {
		client, err := createClient(clientKey(clientName),
			params.Config.Observability.Otel.Enable,
			clientConfig,
		)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create RPC client: %s", clientName)
		}
		rpcClients.clients[clientKey(clientName)] = client

		// 特殊處理 Auth 客戶端
		// if clientKey(clientName) == AuthClient {
		// 	rpcClients.Auth = pb.NewAuthClient(client.Conn())
		// }
	}

	// 註冊生命週期鉤子
	params.Lifecycle.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			for name, client := range rpcClients.clients {
				if err := client.Conn().Close(); err != nil {
					return errors.Wrapf(err, "failed to close RPC client: %s", name)
				}
			}
			return nil
		},
	})

	return rpcClients, nil
}

func createClient(key clientKey, isOtel bool, cfg config.RPCClientConfig) (zrpc.Client, error) {
	var target string
	if cfg.Target != "" {
		target = cfg.Target
	} else if len(cfg.Endpoints) > 0 {
		endpoints := strings.Join(cfg.Endpoints, ",")
		target = fmt.Sprintf("direct://%s", endpoints)
	} else {
		return nil, fmt.Errorf("no target or endpoints configured for %s", key)
	}

	opts := []zrpc.ClientOption{
		zrpc.WithTimeout(cfg.Timeout),
		zrpc.WithNonBlock(),
		zrpc.WithDialOption(
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		),
	}

	if isOtel {
		opts = append(opts, zrpc.WithDialOption(
			grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		))
	}

	return zrpc.NewClient(zrpc.RpcClientConf{
		Target: target,
	}, opts...)
}

// GetClient 獲取指定的 RPC 客戶端
func (r *RPCClients) GetClient(key clientKey) (zrpc.Client, error) {
	client, ok := r.clients[key]
	if !ok {
		return nil, fmt.Errorf("RPC client not found: %s", key)
	}
	return client, nil
}
