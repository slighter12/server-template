package mysql

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/fx"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const (
	_defaultMaxOpenConns     = 25
	_defaultMaxIdleConns     = 25
	_defaultMaxLifeTime      = 5 * time.Minute
	_defaultSlowSQLThreshold = 200 * time.Millisecond
)

type DBConn struct {
	Host            string        `json:"host" yaml:"host"`
	Port            string        `json:"port" yaml:"port"`
	UserName        string        `json:"username" yaml:"username"`
	Password        string        `json:"password" yaml:"password"`
	Loc             string        `json:"loc" yaml:"loc"`
	Timeout         time.Duration `json:"timeout" yaml:"timeout"`
	MaxIdleConns    int           `json:"maxIdleConns" yaml:"maxIdleConns"`
	MaxOpenConns    int           `json:"maxOpenConns" yaml:"maxOpenConns"`
	ConnMaxLifetime time.Duration `json:"connMaxLifetime" yaml:"connMaxLifetime"`
}

func New(lc fx.Lifecycle, conn *DBConn, database string) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8&parseTime=True&loc=%s&timeout=%s",
		conn.UserName,
		conn.Password,
		conn.Host,
		conn.Port,
		database,
		conn.Loc,
		conn.Timeout,
	)

	// https://gorm.io/docs/connecting_to_the_database.html
	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       dsn,
		DefaultStringSize:         256,   // default size for string fields
		DisableDatetimePrecision:  true,  // disable datetime precision, which not supported before MySQL 5.6
		DontSupportRenameIndex:    true,  // drop & create when rename index, rename index not supported before MySQL 5.7, MariaDB
		DontSupportRenameColumn:   true,  // `change` when rename column, rename column not supported before MySQL 8, MariaDB
		SkipInitializeWithVersion: false, // auto configure based on currently MySQL version
	}), &gorm.Config{})
	if err != nil {
		return nil, errors.Wrap(err, "gorm open failed")
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, errors.Wrap(err, "get connect pool failed")
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

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return sqlDB.PingContext(ctx)
		},
		OnStop: func(ctx context.Context) error {
			return sqlDB.Close()
		},
	})

	return db, nil
}
