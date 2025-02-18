package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"server-template/internal/libs/mysql"
	"server-template/internal/libs/redis"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

const defaultPath = "."

type Config struct {
	Env struct {
		Env         string `json:"env" yaml:"env"`
		ServiceName string `json:"serviceName" yaml:"serviceName"`
		Debug       bool   `json:"debug" yaml:"debug"`
		Log         Log    `json:"log" yaml:"log"`
	} `json:"env" yaml:"env"`

	HTTP struct {
		Port     int `json:"port" yaml:"port"`
		Timeouts struct {
			ReadTimeout       time.Duration `json:"readTimeout" yaml:"readTimeout"`
			ReadHeaderTimeout time.Duration `json:"readHeaderTimeout" yaml:"readHeaderTimeout"`
			WriteTimeout      time.Duration `json:"writeTimeout" yaml:"writeTimeout"`
			IdleTimeout       time.Duration `json:"idleTimeout" yaml:"idleTimeout"`
		} `json:"timeouts" yaml:"timeouts"`
	} `json:"http" yaml:"http"`

	Observability struct {
		Pyroscope struct {
			Enable bool   `json:"enable" yaml:"enable"`
			URL    string `json:"url" yaml:"url"`
		} `json:"pyroscope" yaml:"pyroscope"`
		Otel struct {
			Enable   bool   `json:"enable" yaml:"enable"`
			Host     string `json:"host" yaml:"host"`
			Port     int    `json:"port" yaml:"port"`
			IsSecure bool   `json:"isSecure" yaml:"isSecure"`
			Exporter string `json:"exporter" yaml:"exporter"` // 可選: "jaeger", "otlp-grpc", "otlp-http"
		} `json:"otel" yaml:"otel"`
		CloudProfiler struct {
			Enable         bool   `json:"enable" yaml:"enable"`
			ProjectID      string `json:"projectID" yaml:"projectID"`
			ServiceAccount string `json:"serviceAccount" yaml:"serviceAccount"`
		} `json:"cloudProfiler" yaml:"cloudProfiler"`
	} `json:"observability" yaml:"observability"`

	Mysql map[string]*mysql.DBConn `json:"mysql" yaml:"mysql" mapstructure:"mysql"`
	Redis *redis.Conn              `json:"redis" yaml:"redis"`

	RPC struct {
		Clients map[string]RPCClientConfig `mapstructure:"clients" json:"clients" yaml:"clients"`
		Server  struct {
			Target string `json:"target" yaml:"target"`
		} `json:"server" yaml:"server"`
	} `mapstructure:"rpc" json:"rpc" yaml:"rpc"`
}

type Log struct {
	Pretty       bool          `json:"pretty" yaml:"pretty"`
	Level        string        `json:"level" yaml:"level"`
	Path         string        `json:"path" yaml:"path"`
	MaxAge       time.Duration `json:"maxAge" yaml:"maxAge"`
	RotationTime time.Duration `json:"rotationTime" yaml:"rotationTime"`
}

type RPCClientConfig struct {
	Target string `mapstructure:"target" json:"target" yaml:"target"`
}

// LoadWithEnv is a loads .yaml files through viper.
func LoadWithEnv[T any](currEnv string, configPath ...string) (*T, error) {
	cfg := new(T)
	configCtl := viper.New()
	configCtl.SetConfigName(currEnv)
	configCtl.SetConfigType("yaml")
	configCtl.AddConfigPath(defaultPath) // For Ops to deploy, but recommend consistent with the local environment later.
	if len(configPath) != 0 {
		pwd, err := os.Getwd()
		if err != nil {
			return nil, errors.Wrap(err, "os.Getwd")
		}
		for _, path := range configPath {
			abs := filepath.Join(pwd, path)
			configCtl.AddConfigPath(abs)
		}
	}
	configCtl.AutomaticEnv()
	configCtl.SetEnvKeyReplacer(strings.NewReplacer(",", "_"))

	if err := configCtl.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read %s config failed: %w", currEnv, err)
	}

	if err := configCtl.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshal %s config failed: %w", currEnv, err)
	}

	return cfg, nil
}

func New() (*Config, error) {
	return LoadWithEnv[Config]("config", "config", "../connfig", "../../config")
}
