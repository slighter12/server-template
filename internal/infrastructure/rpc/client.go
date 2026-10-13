package rpc

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"server-template/config"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ClientKey 定義支持的 RPC 客戶端類型
type ClientKey string

const (
	AuthClient ClientKey = "auth"
)

// Clients 包含所有 RPC 客戶端
type Clients struct {
	clients map[ClientKey]*grpc.ClientConn
}

type Params struct {
	fx.In
	fx.Lifecycle

	Config *config.Config
}

// New 創建 RPC 客戶端管理器
func New(params Params) (*Clients, error) {
	rpcClients := &Clients{
		clients: make(map[ClientKey]*grpc.ClientConn),
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(ErrorClientInterceptor()),
	}
	if params.Config.Observability.Otel.Enable {
		opts = append(opts, grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
	}

	// Nothing configured: no connections and no lifecycle hooks.
	if len(params.Config.RPC.Clients) == 0 {
		return rpcClients, nil
	}

	// Sorted iteration keeps startup and error behaviour deterministic.
	for _, clientName := range slices.Sorted(maps.Keys(params.Config.RPC.Clients)) {
		clientConn, err := grpc.NewClient(params.Config.RPC.Clients[clientName].Target, opts...)
		if err != nil {
			_ = rpcClients.close()

			return nil, fmt.Errorf("create RPC client %q: %w", clientName, err)
		}

		rpcClients.clients[ClientKey(clientName)] = clientConn
	}

	params.Append(fx.Hook{
		OnStop: func(context.Context) error {
			return rpcClients.close()
		},
	})

	return rpcClients, nil
}

// GetClient 獲取指定的 RPC 客戶端
func (r *Clients) GetClient(key ClientKey) (*grpc.ClientConn, error) {
	client, ok := r.clients[key]
	if !ok {
		return nil, fmt.Errorf("RPC client not found: %s", key)
	}

	return client, nil
}

func (r *Clients) close() error {
	var errs []error
	for _, name := range slices.Sorted(maps.Keys(r.clients)) {
		if err := r.clients[name].Close(); err != nil {
			errs = append(errs, fmt.Errorf("close RPC client %q: %w", name, err))
		}
	}

	return errors.Join(errs...)
}
