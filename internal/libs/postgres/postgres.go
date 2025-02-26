package postgres

import (
	"context"
	"fmt"
	"time"

	"server-template/internal/domain/lifecycle"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pkg/errors"
	"go.uber.org/fx"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
)

const (
	_defaultMaxOpenConns     = 25
	_defaultMaxIdleConns     = 25
	_defaultMaxLifeTime      = 5 * time.Minute
	_defaultSlowSQLThreshold = 200 * time.Millisecond
)

// DBConn 整合了主庫和從庫的配置
type DBConn struct {
	// 主庫配置
	Master ConnectionConfig `json:"master" yaml:"master"`

	// 從庫配置列表
	Replicas []ConnectionConfig `json:"replicas" yaml:"replicas"`

	// 連接池配置
	MaxIdleConns    int           `json:"maxIdleConns" yaml:"maxIdleConns"`
	MaxOpenConns    int           `json:"maxOpenConns" yaml:"maxOpenConns"`
	ConnMaxLifetime time.Duration `json:"connMaxLifetime" yaml:"connMaxLifetime"`

	// 數據庫名稱
	Database string `json:"database" yaml:"database"`

	// PostgreSQL 特有配置
	Schema     string `json:"schema" yaml:"schema"`
	SearchPath string `json:"searchPath" yaml:"searchPath"`
	SSLMode    string `json:"sslMode" yaml:"sslMode"`

	// pgx 特有配置
	ApplicationName   string            `json:"applicationName" yaml:"applicationName"`
	RuntimeParams     map[string]string `json:"runtimeParams" yaml:"runtimeParams"`
	ConnectTimeout    time.Duration     `json:"connectTimeout" yaml:"connectTimeout"`
	StatementCache    bool              `json:"statementCache" yaml:"statementCache"`
	HealthCheckPeriod time.Duration     `json:"healthCheckPeriod" yaml:"healthCheckPeriod"`
}

// ConnectionConfig 定義單個數據庫連接的配置
type ConnectionConfig struct {
	Host     string        `json:"host" yaml:"host"`
	Port     string        `json:"port" yaml:"port"`
	UserName string        `json:"username" yaml:"username"`
	Password string        `json:"password" yaml:"password"`
	Timeout  time.Duration `json:"timeout" yaml:"timeout"`
}

// DSN 生成PostgreSQL連接字符串
func (c *ConnectionConfig) DSN(cfg *DBConn) string {
	sslMode := "disable"
	if cfg.SSLMode != "" {
		sslMode = cfg.SSLMode
	}

	searchPath := "public"
	if cfg.SearchPath != "" {
		searchPath = cfg.SearchPath
	}

	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s search_path=%s",
		c.Host,
		c.Port,
		c.UserName,
		c.Password,
		cfg.Database,
		sslMode,
		searchPath,
	)
}

func setupConnPool(db *gorm.DB, conn *DBConn) error {
	sqlDB, err := db.DB()
	if err != nil {
		return errors.Wrap(err, "failed to get underlying DB")
	}

	maxIdleConns := _defaultMaxIdleConns
	if conn.MaxIdleConns > 0 {
		maxIdleConns = conn.MaxIdleConns
	}

	maxOpenConns := _defaultMaxOpenConns
	if conn.MaxOpenConns > 0 {
		maxOpenConns = conn.MaxOpenConns
	}

	maxLifeTime := _defaultMaxLifeTime
	if conn.ConnMaxLifetime > 0 {
		maxLifeTime = conn.ConnMaxLifetime
	}

	sqlDB.SetMaxIdleConns(maxIdleConns)
	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetConnMaxLifetime(maxLifeTime)

	return nil
}

// NewWithPgx 創建使用 pgx 驅動的數據庫連接，支持讀寫分離
func NewWithPgx(
	lc fx.Lifecycle,
	conn *DBConn,
) (*gorm.DB, error) {
	// 創建主庫連接
	masterConfig, err := pgxpool.ParseConfig(conn.Master.DSN(conn))
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse master config")
	}

	masterDB := stdlib.OpenDB(*masterConfig.ConnConfig)
	dbBase, err := gorm.Open(postgres.New(postgres.Config{
		Conn: masterDB,
	}), &gorm.Config{})

	if err != nil {
		return nil, errors.Wrap(err, "failed to create master connection")
	}

	// 如果有從庫配置，設置讀寫分離
	if len(conn.Replicas) > 0 {
		var replicas []gorm.Dialector
		for _, replica := range conn.Replicas {
			replicaConfig, err := pgxpool.ParseConfig(replica.DSN(conn))
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse replica config")
			}

			replicaDB := stdlib.OpenDB(*replicaConfig.ConnConfig)
			replicas = append(replicas, postgres.New(postgres.Config{
				Conn: replicaDB,
			}))
		}

		// 註冊 dbresolver 插件
		err = dbBase.Use(dbresolver.Register(dbresolver.Config{
			Replicas: replicas,
			Policy:   dbresolver.RandomPolicy{},
		}).SetConnMaxIdleTime(time.Hour))

		if err != nil {
			return nil, errors.Wrap(err, "failed to register dbresolver")
		}
	}

	if err := setupConnPool(dbBase, conn); err != nil {
		return nil, err
	}

	// 設置生命週期鉤子
	sqlDB, err := dbBase.DB()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get underlying DB")
	}

	lc.Append(fx.Hook{
		OnStart: func(startCtx context.Context) error {
			ctx, cancel := context.WithTimeout(startCtx, lifecycle.DefaultTimeout)
			defer cancel()

			return sqlDB.PingContext(ctx)
		},
		OnStop: func(_ context.Context) error {
			return sqlDB.Close()
		},
	})

	return dbBase, nil
}
