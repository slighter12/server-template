package postgres_test

import (
	"testing"
	"time"

	"server-template/internal/libs/postgres"

	"go.uber.org/fx/fxtest"
)

func TestNewWithPgx(t *testing.T) {
	lc := fxtest.NewLifecycle(t)

	db, err := postgres.NewWithPgx(lc, &postgres.DBConn{
		Database: "test_db",
		Master: postgres.ConnectionConfig{
			Host:     "localhost",
			Port:     "5432",
			UserName: "test_user",
			Password: "test_pass",
		},
		Replicas: []postgres.ConnectionConfig{
			{
				Host:     "localhost",
				Port:     "5432",
				UserName: "test_reader",
				Password: "test_pass",
			},
		},
		MaxOpenConns:    50,
		MaxIdleConns:    10,
		ConnMaxLifetime: time.Hour,
		SearchPath:      "public",
		SSLMode:         "disable",
	})

	if err != nil {
		t.Fatalf("failed to create db connection: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get underlying DB: %v", err)
	}

	// 測試連接池配置
	if actual := sqlDB.Stats().MaxOpenConnections; actual != 50 {
		t.Errorf("expected MaxOpenConns=50, got %d", actual)
	}
}
