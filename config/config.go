package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/slighter12/go-lib/database/mongo"
	"github.com/slighter12/go-lib/database/mysql"
	"github.com/slighter12/go-lib/database/postgres"
	"github.com/slighter12/go-lib/database/redis/cluster"
	"github.com/spf13/viper"
)

const (
	defaultPath = "."

	// MinJWTSecretLength is the minimum accepted length (bytes) of Auth.JWTSecret.
	MinJWTSecretLength = 32

	defaultHTTPPort           = 8080
	defaultHTTP3Port          = 8443
	defaultGRPCPort           = 9090
	defaultMaxRequestBodySize = "4M"
	defaultDebugServerAddr    = "127.0.0.1:6060"
	defaultAccessTokenTTL     = 24 * time.Hour
	defaultLogLevel           = "info"
)

type Config struct {
	Env struct {
		Env         string `json:"env" yaml:"env" mapstructure:"env"`
		ServiceName string `json:"serviceName" yaml:"serviceName" mapstructure:"serviceName"`
		Debug       bool   `json:"debug" yaml:"debug" mapstructure:"debug"`
		Log         Log    `json:"log" yaml:"log" mapstructure:"log"`
	} `json:"env" yaml:"env" mapstructure:"env"`

	HTTP struct {
		Port int `json:"port" yaml:"port" mapstructure:"port"`
		// MaxRequestBodySize accepts echo body-limit notation such as "4M" or "512K".
		MaxRequestBodySize string `json:"maxRequestBodySize" yaml:"maxRequestBodySize" mapstructure:"maxRequestBodySize"`
		// MaxHeaderValueCount limits repeated header values; 0 keeps the net/http default.
		MaxHeaderValueCount int      `json:"maxHeaderValueCount" yaml:"maxHeaderValueCount" mapstructure:"maxHeaderValueCount"`
		CORSAllowedOrigins  []string `json:"corsAllowedOrigins" yaml:"corsAllowedOrigins" mapstructure:"corsAllowedOrigins"`
		RateLimit           struct {
			Enable            bool    `json:"enable" yaml:"enable" mapstructure:"enable"`
			RequestsPerSecond float64 `json:"requestsPerSecond" yaml:"requestsPerSecond" mapstructure:"requestsPerSecond"`
			Burst             int     `json:"burst" yaml:"burst" mapstructure:"burst"`
		} `json:"rateLimit" yaml:"rateLimit" mapstructure:"rateLimit"`
		CloudflareSecret string `json:"cloudflareSecret" yaml:"cloudflareSecret" mapstructure:"cloudflareSecret"`
		Timeouts         struct {
			ReadTimeout       time.Duration `json:"readTimeout" yaml:"readTimeout" mapstructure:"readTimeout"`
			ReadHeaderTimeout time.Duration `json:"readHeaderTimeout" yaml:"readHeaderTimeout" mapstructure:"readHeaderTimeout"`
			WriteTimeout      time.Duration `json:"writeTimeout" yaml:"writeTimeout" mapstructure:"writeTimeout"`
			IdleTimeout       time.Duration `json:"idleTimeout" yaml:"idleTimeout" mapstructure:"idleTimeout"`
		} `json:"timeouts" yaml:"timeouts" mapstructure:"timeouts"`
	} `json:"http" yaml:"http" mapstructure:"http"`

	HTTP3 struct {
		Enable bool `json:"enable" yaml:"enable" mapstructure:"enable"`
		Port   int  `json:"port" yaml:"port" mapstructure:"port"`
	} `json:"http3" yaml:"http3" mapstructure:"http3"`

	GRPC struct {
		Port       int  `json:"port" yaml:"port" mapstructure:"port"`
		Reflection bool `json:"reflection" yaml:"reflection" mapstructure:"reflection"`
	} `json:"grpc" yaml:"grpc" mapstructure:"grpc"`

	DebugServer struct {
		Enable bool   `json:"enable" yaml:"enable" mapstructure:"enable"`
		Addr   string `json:"addr" yaml:"addr" mapstructure:"addr"`
	} `json:"debugServer" yaml:"debugServer" mapstructure:"debugServer"`

	Observability struct {
		Pyroscope struct {
			Enable bool   `json:"enable" yaml:"enable" mapstructure:"enable"`
			URL    string `json:"url" yaml:"url" mapstructure:"url"`
		} `json:"pyroscope" yaml:"pyroscope" mapstructure:"pyroscope"`
		Otel struct {
			Enable   bool   `json:"enable" yaml:"enable" mapstructure:"enable"`
			Host     string `json:"host" yaml:"host" mapstructure:"host"`
			Port     int    `json:"port" yaml:"port" mapstructure:"port"`
			IsSecure bool   `json:"isSecure" yaml:"isSecure" mapstructure:"isSecure"`
			Exporter string `json:"exporter" yaml:"exporter" mapstructure:"exporter"` // "otlp-grpc", "otlp-http" or "jaeger"
		} `json:"otel" yaml:"otel" mapstructure:"otel"`
		CloudProfiler struct {
			Enable         bool   `json:"enable" yaml:"enable" mapstructure:"enable"`
			ProjectID      string `json:"projectID" yaml:"projectID" mapstructure:"projectID"`
			ServiceAccount string `json:"serviceAccount" yaml:"serviceAccount" mapstructure:"serviceAccount"`
		} `json:"cloudProfiler" yaml:"cloudProfiler" mapstructure:"cloudProfiler"`
	} `json:"observability" yaml:"observability" mapstructure:"observability"`

	// Database sections are optional: an empty/missing section disables that connection.
	Mysql    map[string]*mysql.DBConn    `json:"mysql" yaml:"mysql" mapstructure:"mysql"`
	Postgres map[string]*postgres.DBConn `json:"postgres" yaml:"postgres" mapstructure:"postgres"`
	Redis    *cluster.Conn               `json:"redis" yaml:"redis" mapstructure:"redis"`
	Mongo    map[string]*mongo.DBConn    `json:"mongo" yaml:"mongo" mapstructure:"mongo"`

	RPC struct {
		Clients map[string]RPCClientConfig `mapstructure:"clients" json:"clients" yaml:"clients"`
		Server  struct {
			Target string `json:"target" yaml:"target" mapstructure:"target"`
		} `json:"server" yaml:"server" mapstructure:"server"`
	} `mapstructure:"rpc" json:"rpc" yaml:"rpc"`

	Auth struct {
		JWTSecret      string        `json:"jwtSecret" yaml:"jwtSecret" mapstructure:"jwtSecret"`
		AccessTokenTTL time.Duration `json:"accessTokenTTL" yaml:"accessTokenTTL" mapstructure:"accessTokenTTL"`
		Argon2         Argon2Config  `json:"argon2" yaml:"argon2" mapstructure:"argon2"`
	} `json:"auth" yaml:"auth" mapstructure:"auth"`
}

// Argon2Config holds argon2id password hashing parameters.
type Argon2Config struct {
	Memory        uint32 `json:"memory" yaml:"memory" mapstructure:"memory"` // KiB
	Iterations    uint32 `json:"iterations" yaml:"iterations" mapstructure:"iterations"`
	Parallelism   uint8  `json:"parallelism" yaml:"parallelism" mapstructure:"parallelism"`
	SaltLength    uint32 `json:"saltLength" yaml:"saltLength" mapstructure:"saltLength"`
	KeyLength     uint32 `json:"keyLength" yaml:"keyLength" mapstructure:"keyLength"`
	MaxConcurrent int    `json:"maxConcurrent" yaml:"maxConcurrent" mapstructure:"maxConcurrent"`
}

type Log struct {
	Pretty bool   `json:"pretty" yaml:"pretty" mapstructure:"pretty"`
	Level  string `json:"level" yaml:"level" mapstructure:"level"`
}

type RPCClientConfig struct {
	Target string `mapstructure:"target" json:"target" yaml:"target"`
}

// ApplyDefaults fills zero-valued fields with their defaults.
func (c *Config) ApplyDefaults() {
	if c.Env.Log.Level == "" {
		c.Env.Log.Level = defaultLogLevel
	}
	if c.HTTP.Port == 0 {
		c.HTTP.Port = defaultHTTPPort
	}
	if c.HTTP.MaxRequestBodySize == "" {
		c.HTTP.MaxRequestBodySize = defaultMaxRequestBodySize
	}
	if c.HTTP3.Port == 0 {
		c.HTTP3.Port = defaultHTTP3Port
	}
	if c.GRPC.Port == 0 {
		c.GRPC.Port = defaultGRPCPort
	}
	if c.DebugServer.Addr == "" {
		c.DebugServer.Addr = defaultDebugServerAddr
	}
	if c.RPC.Server.Target == "" {
		c.RPC.Server.Target = "localhost:" + strconv.Itoa(c.GRPC.Port)
	}
	if c.Auth.AccessTokenTTL == 0 {
		c.Auth.AccessTokenTTL = defaultAccessTokenTTL
	}

	c.Auth.Argon2.applyDefaults()
}

func (a *Argon2Config) applyDefaults() {
	if a.Memory == 0 {
		a.Memory = 64 * 1024
	}
	if a.Iterations == 0 {
		a.Iterations = 3
	}
	if a.Parallelism == 0 {
		a.Parallelism = 2
	}
	if a.SaltLength == 0 {
		a.SaltLength = 16
	}
	if a.KeyLength == 0 {
		a.KeyLength = 32
	}
	if a.MaxConcurrent == 0 {
		a.MaxConcurrent = 4
	}
}

// Validate reports every configuration problem it finds, joined into one error.
func (c *Config) Validate() error {
	return errors.Join(
		c.validateAuth(),
		c.validateHTTP(),
		c.validateOther(),
	)
}

func (c *Config) validateAuth() error {
	var errs []error

	if len(c.Auth.JWTSecret) < MinJWTSecretLength {
		errs = append(errs, fmt.Errorf("auth.jwtSecret must be at least %d bytes", MinJWTSecretLength))
	}
	if c.Auth.AccessTokenTTL <= 0 {
		errs = append(errs, errors.New("auth.accessTokenTTL must be positive"))
	}
	if a := c.Auth.Argon2; a.Memory == 0 || a.Iterations == 0 || a.Parallelism == 0 ||
		a.SaltLength == 0 || a.KeyLength == 0 || a.MaxConcurrent <= 0 {
		errs = append(errs, errors.New("auth.argon2 parameters must all be positive"))
	}

	return errors.Join(errs...)
}

func (c *Config) validateHTTP() error {
	errs := []error{validPort("http.port", c.HTTP.Port)}

	if c.HTTP3.Enable {
		errs = append(errs, validPort("http3.port", c.HTTP3.Port))
	}
	if c.HTTP.MaxHeaderValueCount < 0 {
		errs = append(errs, errors.New("http.maxHeaderValueCount must not be negative"))
	}
	if c.HTTP.RateLimit.Enable && (c.HTTP.RateLimit.RequestsPerSecond <= 0 || c.HTTP.RateLimit.Burst <= 0) {
		errs = append(errs, errors.New("http.rateLimit requestsPerSecond and burst must be positive when enabled"))
	}

	return errors.Join(errs...)
}

// validateOther covers gRPC, RPC clients, the debug server and OTel.
func (c *Config) validateOther() error {
	errs := []error{validPort("grpc.port", c.GRPC.Port)}

	if c.DebugServer.Enable && c.DebugServer.Addr == "" {
		errs = append(errs, errors.New("debugServer.addr is required when debugServer.enable is true"))
	}
	if c.Observability.Otel.Enable {
		errs = append(errs, validPort("observability.otel.port", c.Observability.Otel.Port))
	}
	if c.RPC.Server.Target == "" {
		errs = append(errs, errors.New("rpc.server.target is required"))
	}
	for name, client := range c.RPC.Clients {
		if client.Target == "" {
			errs = append(errs, fmt.Errorf("rpc.clients.%s.target is required", name))
		}
	}

	return errors.Join(errs...)
}

func validPort(name string, port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("%s must be between 1 and 65535, got %d", name, port)
	}

	return nil
}

// LoadWithEnv loads <currEnv>.yaml through viper.
//
// Environment variables override YAML values: the key path is upper-cased and
// joined with "_" (http.port -> HTTP_PORT, observability.otel.enable ->
// OBSERVABILITY_OTEL_ENABLE). Viper only unmarshals keys that exist in the
// YAML file, so a key must be present in the file (even with a default value)
// for its environment override to apply.
func LoadWithEnv[T any](currEnv string, configPath ...string) (*T, error) {
	cfg := new(T)
	configCtl := viper.New()
	configCtl.SetConfigName(currEnv)
	configCtl.SetConfigType("yaml")
	configCtl.AddConfigPath(defaultPath) // For Ops to deploy, but recommend consistent with the local environment later.
	if len(configPath) != 0 {
		pwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("os.Getwd: %w", err)
		}
		for _, path := range configPath {
			configCtl.AddConfigPath(filepath.Join(pwd, path))
		}
	}
	configCtl.AutomaticEnv()
	configCtl.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := configCtl.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read %s config failed: %w", currEnv, err)
	}

	if err := configCtl.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshal %s config failed: %w", currEnv, err)
	}

	return cfg, nil
}

// New loads config.yaml, applies defaults and validates the result.
func New() (*Config, error) {
	cfg, err := LoadWithEnv[Config]("config", "config", "../config", "../../config")
	if err != nil {
		return nil, err
	}

	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, nil
}
