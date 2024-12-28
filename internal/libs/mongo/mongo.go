package mongo

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"server-template/internal/domain/lifecycle"

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/fx"
)

const (
	_defaultMaxPoolSize     = 100
	_defaultMinPoolSize     = 10
	_defaultMaxConnIdleTime = 5 * time.Minute
	_defaultConnectTimeout  = 10 * time.Second
)

type DBConn struct {
	Hosts           []string          `json:"hosts" yaml:"hosts"`
	Username        string            `json:"username" yaml:"username"`
	Password        string            `json:"password" yaml:"password"`
	AuthDB          string            `json:"authDB" yaml:"authDB"`
	MaxPoolSize     uint64            `json:"maxPoolSize" yaml:"maxPoolSize"`
	MinPoolSize     uint64            `json:"minPoolSize" yaml:"minPoolSize"`
	MaxConnIdleTime time.Duration     `json:"maxConnIdleTime" yaml:"maxConnIdleTime"`
	ConnectTimeout  time.Duration     `json:"connectTimeout" yaml:"connectTimeout"`
	Options         map[string]string `json:"options" yaml:"options"`
}

func New(
	ctx context.Context,
	lc fx.Lifecycle,
	conn *DBConn,
) (*mongo.Client, error) {
	// 構建連線 URI
	var auth, optionsStr string
	if conn.Username != "" && conn.Password != "" {
		auth = fmt.Sprintf("%s:%s@", conn.Username, conn.Password)
	}

	hosts := strings.Join(conn.Hosts, ",")
	authDB := conn.AuthDB

	// 處理額外選項
	if len(conn.Options) > 0 {
		var params []string
		for key, value := range conn.Options {
			params = append(params, fmt.Sprintf("%s=%s", key, url.QueryEscape(value)))
		}
		optionsStr = "?" + strings.Join(params, "&")
	}

	uri := fmt.Sprintf("mongodb://%s%s/%s%s", auth, hosts, authDB, optionsStr)

	// 設置連線池參數
	maxPoolSize := conn.MaxPoolSize
	if maxPoolSize > 0 {
		maxPoolSize = _defaultMaxPoolSize
	}

	minPoolSize := conn.MinPoolSize
	if minPoolSize == 0 {
		minPoolSize = _defaultMinPoolSize
	}

	maxConnIdleTime := _defaultMaxConnIdleTime
	if conn.MaxConnIdleTime > 0 {
		maxConnIdleTime = conn.MaxConnIdleTime
	}

	// 設置客戶端選項
	clientOptions := options.Client().
		ApplyURI(uri).
		SetMaxPoolSize(maxPoolSize).
		SetMinPoolSize(minPoolSize).
		SetMaxConnIdleTime(maxConnIdleTime)

	// 建立連線
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, errors.Wrap(err, "mongo connect failed")
	}

	// 生命周期管理
	lc.Append(fx.Hook{
		OnStart: func(startCtx context.Context) error {
			ctx, cancel := context.WithTimeout(startCtx, lifecycle.DefaultTimeout)
			defer cancel()

			return client.Ping(ctx, nil)
		},
		OnStop: func(stopCtx context.Context) error {
			ctx, cancel := context.WithTimeout(stopCtx, lifecycle.DefaultTimeout)
			defer cancel()

			return client.Disconnect(ctx)
		},
	})

	return client, nil
}
