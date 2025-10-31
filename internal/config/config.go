// Package config provides configuration management for Graphium.
//
// This package handles loading configuration from multiple sources:
//   - YAML configuration files
//   - Environment variables (with CG_ prefix)
//   - .env files
//   - Default values
//
// # Configuration Sources Priority
//
// Configuration is loaded in the following order (later sources override earlier ones):
//  1. Default values (hardcoded)
//  2. Configuration files (./configs/config.yaml, ~/.graphium/config.yaml, /etc/graphium/config.yaml)
//  3. .env files
//  4. Environment variables (CG_ prefix)
//
// # Usage Example
//
//	cfg, err := config.Load("configs/config.yaml")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Server: %s:%d\n", cfg.Server.Host, cfg.Server.Port)
//
// # Environment Variables
//
// Environment variables override all other configuration sources.
// Use CG_ prefix and underscores for nested keys:
//   - CG_SERVER_PORT=8095
//   - CG_COUCHDB_URL=http://localhost:5984
//   - CG_AGENT_ENABLED=true
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config is the root configuration structure for Graphium.
// It contains all configuration sections for server, database, agent manager, logging, and security.
type Config struct {
	// Server contains HTTP server configuration
	Server ServerConfig `mapstructure:"server"`

	// CouchDB contains database connection settings
	CouchDB CouchDBConfig `mapstructure:"couchdb"`

	// Agent contains Docker agent configuration (deprecated - kept for backward compatibility)
	Agent AgentConfig `mapstructure:"agent"`

	// Agents contains agent manager configuration for managing remote agents
	Agents AgentsManagerConfig `mapstructure:"agents"`

	// Logging contains logging and observability settings
	Logging LoggingConfig `mapstructure:"logging"`

	// Security contains security and rate limiting settings
	Security SecurityConfig `mapstructure:"security"`
}

// ServerConfig contains HTTP server configuration.
type ServerConfig struct {
	// Host is the server bind address (default: localhost)
	Host string `mapstructure:"host"`

	// Port is the server listen port (default: 8095)
	Port int `mapstructure:"port"`

	// ReadTimeout is the maximum duration for reading requests
	ReadTimeout time.Duration `mapstructure:"read_timeout"`

	// WriteTimeout is the maximum duration for writing responses
	WriteTimeout time.Duration `mapstructure:"write_timeout"`

	// ShutdownTimeout is the maximum duration for graceful shutdown
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`

	// Debug enables debug logging and additional endpoints
	Debug bool `mapstructure:"debug"`

	// TLSEnabled enables HTTPS
	TLSEnabled bool `mapstructure:"tls_enabled"`

	// TLSCert is the path to the TLS certificate file
	TLSCert string `mapstructure:"tls_cert"`

	// TLSKey is the path to the TLS private key file
	TLSKey string `mapstructure:"tls_key"`
}

// CouchDBConfig contains CouchDB connection settings.
type CouchDBConfig struct {
	// URL is the CouchDB server URL (e.g., http://localhost:5984)
	URL string `mapstructure:"url"`

	// Database is the database name to use
	Database string `mapstructure:"database"`

	// Username for CouchDB authentication
	Username string `mapstructure:"username"`

	// Password for CouchDB authentication
	Password string `mapstructure:"password"`

	// MaxConnections is the maximum number of concurrent connections
	MaxConnections int `mapstructure:"max_connections"`

	// Timeout in seconds for database operations
	Timeout int `mapstructure:"timeout"`
}

// AgentConfig contains Docker agent configuration (deprecated - kept for backward compatibility).
type AgentConfig struct {
	// Enabled determines if the agent should run
	Enabled bool `mapstructure:"enabled"`

	// APIURL is the URL of the Graphium API server
	APIURL string `mapstructure:"api_url"`

	// HostID is the unique identifier for this host
	HostID string `mapstructure:"host_id"`

	// Datacenter is the datacenter/location identifier
	Datacenter string `mapstructure:"datacenter"`

	// SyncInterval is the duration between container syncs
	SyncInterval time.Duration `mapstructure:"sync_interval"`

	// DockerSocket is the path to the Docker socket
	DockerSocket string `mapstructure:"docker_socket"`

	// AgentToken is the JWT token for agent authentication
	AgentToken string `mapstructure:"agent_token"`
}

// AgentsManagerConfig contains configuration for the agent manager.
type AgentsManagerConfig struct {
	// LogsPath is the directory where agent logs will be stored
	// Each agent will have its own log file: {LogsPath}/{host-id}.log
	LogsPath string `mapstructure:"logs_path"`
}

// LoggingConfig contains logging configuration.
type LoggingConfig struct {
	// Level is the log level (debug, info, warn, error)
	Level string `mapstructure:"level"`

	// Format is the log format (json, text)
	Format string `mapstructure:"format"`

	// Output is the log output destination (stdout, file)
	Output string `mapstructure:"output"`

	// MaxSize is the maximum log file size in megabytes
	MaxSize int `mapstructure:"max_size"`

	// MaxBackups is the maximum number of old log files to keep
	MaxBackups int `mapstructure:"max_backups"`

	// MaxAge is the maximum number of days to keep old log files
	MaxAge int `mapstructure:"max_age"`
}

// SecurityConfig contains security and rate limiting settings.
type SecurityConfig struct {
	// RateLimit is the maximum requests per second per client
	RateLimit int `mapstructure:"rate_limit"`

	// AllowedOrigins are the CORS allowed origins
	AllowedOrigins []string `mapstructure:"allowed_origins"`

	// APIKeys are valid API keys for authentication (optional)
	APIKeys []string `mapstructure:"api_keys"`

	// AuthEnabled enables JWT authentication (default: false for backwards compatibility)
	AuthEnabled bool `mapstructure:"auth_enabled"`

	// JWTSecret is the secret key for signing JWT tokens
	JWTSecret string `mapstructure:"jwt_secret"`

	// JWTExpiration is the JWT token expiration duration (default: 24h)
	JWTExpiration time.Duration `mapstructure:"jwt_expiration"`

	// RefreshTokenExpiration is the refresh token expiration duration (default: 7 days)
	RefreshTokenExpiration time.Duration `mapstructure:"refresh_token_expiration"`

	// AgentTokenSecret is the secret key for agent authentication tokens
	AgentTokenSecret string `mapstructure:"agent_token_secret"`
}

var cfg *Config

// Load reads configuration from a file and environment variables.
// If cfgFile is empty, it searches for config.yaml in standard locations.
//
// Configuration precedence (highest to lowest):
//  1. Environment variables (CG_ prefix)
//  2. .env file
//  3. Configuration file
//  4. Default values
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
		// If config file was explicitly specified, fail on any error
		// If searching multiple paths, only fail on errors other than ConfigFileNotFoundError
		if cfgFile != "" {
			// For explicit file path, check if it's a "file not found" type error
			// In this case, we want to proceed with defaults
			if !isFileNotFoundError(err) {
				return nil, fmt.Errorf("error reading config file: %w", err)
			}
		} else {
			// For auto-discovery, only fail on non-NotFound errors
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return nil, fmt.Errorf("error reading config file: %w", err)
			}
		}
	}

	v.SetConfigFile(".env")
	v.SetConfigType("env")
	_ = v.MergeInConfig() // Ignore error if .env file doesn't exist

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

	v.SetDefault("agents.logs_path", "./logs")

	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
	v.SetDefault("logging.output", "stdout")
	v.SetDefault("logging.max_size", 100)
	v.SetDefault("logging.max_backups", 3)
	v.SetDefault("logging.max_age", 7)

	v.SetDefault("security.rate_limit", 100)
	v.SetDefault("security.allowed_origins", []string{"*"})
	v.SetDefault("security.auth_enabled", false)
	v.SetDefault("security.jwt_secret", "change-me-in-production")
	v.SetDefault("security.jwt_expiration", "24h")
	v.SetDefault("security.refresh_token_expiration", "168h") // 7 days
	v.SetDefault("security.agent_token_secret", "change-me-in-production")
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

// isFileNotFoundError checks if an error is a file not found error.
func isFileNotFoundError(err error) bool {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return errors.Is(pathErr, os.ErrNotExist)
	}
	return false
}
