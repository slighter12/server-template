package config

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func validConfig() *Config {
	cfg := &Config{}
	cfg.Auth.JWTSecret = testSecret
	cfg.ApplyDefaults()

	return cfg
}

func TestApplyDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.ApplyDefaults()

	assert.Equal(t, 8080, cfg.HTTP.Port)
	assert.Equal(t, "4M", cfg.HTTP.MaxRequestBodySize)
	assert.False(t, cfg.HTTP3.Enable)
	assert.Equal(t, 8443, cfg.HTTP3.Port)
	assert.Equal(t, 9090, cfg.GRPC.Port)
	assert.False(t, cfg.DebugServer.Enable)
	assert.Equal(t, "127.0.0.1:6060", cfg.DebugServer.Addr)
	assert.Equal(t, 24*time.Hour, cfg.Auth.AccessTokenTTL)
	assert.Equal(t, Argon2Config{
		Memory: 64 * 1024, Iterations: 3, Parallelism: 2, SaltLength: 16, KeyLength: 32, MaxConcurrent: 4,
	}, cfg.Auth.Argon2)
	assert.Equal(t, "localhost:9090", cfg.RPC.Server.Target)
}

func TestApplyDefaultsKeepsExplicitValues(t *testing.T) {
	cfg := &Config{}
	cfg.HTTP.Port = 1234
	cfg.ApplyDefaults()

	assert.Equal(t, 1234, cfg.HTTP.Port)
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{"valid", func(*Config) {}, ""},
		{"short secret", func(c *Config) { c.Auth.JWTSecret = "short" }, "auth.jwtSecret"},
		{"empty secret", func(c *Config) { c.Auth.JWTSecret = "" }, "auth.jwtSecret"},
		{"bad http port", func(c *Config) { c.HTTP.Port = 70000 }, "http.port"},
		{"bad grpc port", func(c *Config) { c.GRPC.Port = -1 }, "grpc.port"},
		{"bad http3 port only when enabled", func(c *Config) { c.HTTP3.Enable = true; c.HTTP3.Port = 0 }, "http3.port"},
		{"empty rpc target", func(c *Config) { c.RPC.Server.Target = "" }, "rpc.server.target"},
		{"rate limit incomplete", func(c *Config) { c.HTTP.RateLimit.Enable = true }, "http.rateLimit"},
		{"argon2 zero", func(c *Config) { c.Auth.Argon2.MaxConcurrent = 0 }, "auth.argon2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.mutate(cfg)

			err := cfg.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)

				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}

	t.Run("http3 disabled ignores port", func(t *testing.T) {
		cfg := validConfig()
		cfg.HTTP3.Port = 0
		require.NoError(t, cfg.Validate())
	})
}

func TestNew_EnvOverridesYAML(t *testing.T) {
	dir := t.TempDir()
	yaml := strings.Join([]string{
		"auth:",
		"  jwtSecret: " + testSecret,
		"http:",
		"  port: 8080",
		"observability:",
		"  otel:",
		"    enable: false",
		"    port: 4317",
		"",
	}, "\n")
	require.NoError(t, os.WriteFile(dir+"/config.yaml", []byte(yaml), 0o600))
	t.Chdir(dir)

	cfg, err := New()
	require.NoError(t, err)
	assert.Equal(t, 8080, cfg.HTTP.Port)
	assert.False(t, cfg.Observability.Otel.Enable)

	t.Setenv("HTTP_PORT", "9999")
	t.Setenv("OBSERVABILITY_OTEL_ENABLE", "true")

	cfg, err = New()
	require.NoError(t, err)
	assert.Equal(t, 9999, cfg.HTTP.Port)
	assert.True(t, cfg.Observability.Otel.Enable)
	// defaults were applied for keys absent from the YAML
	assert.Equal(t, 9090, cfg.GRPC.Port)
}

func TestNew_InvalidConfig(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(dir+"/config.yaml", []byte("http:\n  port: 8080\n"), 0o600))
	t.Chdir(dir)

	_, err := New()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auth.jwtSecret")
}

func TestDemoConfigIsValid(t *testing.T) {
	data, err := os.ReadFile("config_demo.yaml")
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(dir+"/config.yaml", data, 0o600))
	t.Chdir(dir)

	cfg, err := New()
	require.NoError(t, err)
	assert.Equal(t, 8080, cfg.HTTP.Port)
	assert.Equal(t, 9090, cfg.GRPC.Port)
	assert.Equal(t, "otlp-grpc", cfg.Observability.Otel.Exporter)
	assert.Contains(t, cfg.Postgres, "main")
	assert.Empty(t, cfg.Mysql)
	assert.Nil(t, cfg.Redis)
	assert.Equal(t, "localhost:9090", cfg.RPC.Clients["auth"].Target)
}
