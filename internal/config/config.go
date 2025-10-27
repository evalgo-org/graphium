package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	CouchDB  CouchDBConfig  `mapstructure:"couchdb"`
	Agent    AgentConfig    `mapstructure:"agent"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	Security SecurityConfig `mapstructure:"security"`
}

type ServerConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
	Debug           bool          `mapstructure:"debug"`
	TLSEnabled      bool          `mapstructure:"tls_enabled"`
	TLSCert         string        `mapstructure:"tls_cert"`
	TLSKey          string        `mapstructure:"tls_key"`
}

type CouchDBConfig struct {
	URL            string `mapstructure:"url"`
	Database       string `mapstructure:"database"`
	Username       string `mapstructure:"username"`
	Password       string `mapstructure:"password"`
	MaxConnections int    `mapstructure:"max_connections"`
	Timeout        int    `mapstructure:"timeout"`
}

type AgentConfig struct {
	Enabled      bool          `mapstructure:"enabled"`
	APIURL       string        `mapstructure:"api_url"`
	HostID       string        `mapstructure:"host_id"`
	Datacenter   string        `mapstructure:"datacenter"`
	SyncInterval time.Duration `mapstructure:"sync_interval"`
	DockerSocket string        `mapstructure:"docker_socket"`
}

type LoggingConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	Output     string `mapstructure:"output"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
}

type SecurityConfig struct {
	RateLimit      int      `mapstructure:"rate_limit"`
	AllowedOrigins []string `mapstructure:"allowed_origins"`
	APIKeys        []string `mapstructure:"api_keys"`
}

var cfg *Config

func Load(cfgFile string) (*Config, error) {
	v := viper.New()

	setDefaults(v)

	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./configs")
		v.AddConfigPath("$HOME/.graphium")
		v.AddConfigPath("/etc/graphium")
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	v.SetConfigFile(".env")
	v.SetConfigType("env")
	v.MergeInConfig()

	v.SetEnvPrefix("CG")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	cfg = &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unable to decode config: %w", err)
	}

	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", "30s")
	v.SetDefault("server.write_timeout", "30s")
	v.SetDefault("server.shutdown_timeout", "10s")
	v.SetDefault("server.debug", false)
	v.SetDefault("server.tls_enabled", false)

	v.SetDefault("couchdb.url", "http://localhost:5984")
	v.SetDefault("couchdb.database", "graphium")
	v.SetDefault("couchdb.username", "admin")
	v.SetDefault("couchdb.password", "password")
	v.SetDefault("couchdb.max_connections", 10)
	v.SetDefault("couchdb.timeout", 30)

	v.SetDefault("agent.enabled", false)
	v.SetDefault("agent.api_url", "http://localhost:8080")
	v.SetDefault("agent.sync_interval", "30s")
	v.SetDefault("agent.docker_socket", "/var/run/docker.sock")

	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
	v.SetDefault("logging.output", "stdout")
	v.SetDefault("logging.max_size", 100)
	v.SetDefault("logging.max_backups", 3)
	v.SetDefault("logging.max_age", 7)

	v.SetDefault("security.rate_limit", 100)
	v.SetDefault("security.allowed_origins", []string{"*"})
}

func validate(cfg *Config) error {
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", cfg.Server.Port)
	}

	if cfg.CouchDB.URL == "" {
		return fmt.Errorf("couchdb url is required")
	}

	if cfg.CouchDB.Database == "" {
		return fmt.Errorf("couchdb database is required")
	}

	return nil
}

func Get() *Config {
	return cfg
}

func (c *CouchDBConfig) BuildURL() string {
	if c.Username != "" && c.Password != "" {
		url := strings.Replace(c.URL, "://", "://"+c.Username+":"+c.Password+"@", 1)
		return url
	}
	return c.URL
}
