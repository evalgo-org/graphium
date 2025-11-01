package config

import (
	"os"
	"testing"
	"time"
)

// TestLoadDefaults tests that default configuration values are loaded correctly.
func TestLoadDefaults(t *testing.T) {
	// Load configuration without a config file
	cfg, err := Load("nonexistent.yaml")
	if err != nil {
		t.Fatalf("Failed to load defaults: %v", err)
	}

	// Test Server defaults
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Expected default server host '0.0.0.0', got '%s'", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Expected default server port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Server.ReadTimeout != 30*time.Second {
		t.Errorf("Expected default read timeout 30s, got %v", cfg.Server.ReadTimeout)
	}
	if cfg.Server.WriteTimeout != 30*time.Second {
		t.Errorf("Expected default write timeout 30s, got %v", cfg.Server.WriteTimeout)
	}
	if cfg.Server.ShutdownTimeout != 10*time.Second {
		t.Errorf("Expected default shutdown timeout 10s, got %v", cfg.Server.ShutdownTimeout)
	}
	if cfg.Server.Debug != false {
		t.Errorf("Expected default debug false, got %v", cfg.Server.Debug)
	}
	if cfg.Server.TLSEnabled != false {
		t.Errorf("Expected default tls_enabled false, got %v", cfg.Server.TLSEnabled)
	}

	// Test CouchDB defaults
	if cfg.CouchDB.URL != "http://localhost:5984" {
		t.Errorf("Expected default couchdb url 'http://localhost:5984', got '%s'", cfg.CouchDB.URL)
	}
	if cfg.CouchDB.Database != "graphium" {
		t.Errorf("Expected default database 'graphium', got '%s'", cfg.CouchDB.Database)
	}
	if cfg.CouchDB.Username != "admin" {
		t.Errorf("Expected default username 'admin', got '%s'", cfg.CouchDB.Username)
	}
	if cfg.CouchDB.Password != "password" {
		t.Errorf("Expected default password 'password', got '%s'", cfg.CouchDB.Password)
	}
	if cfg.CouchDB.MaxConnections != 10 {
		t.Errorf("Expected default max connections 10, got %d", cfg.CouchDB.MaxConnections)
	}
	if cfg.CouchDB.Timeout != 30 {
		t.Errorf("Expected default timeout 30, got %d", cfg.CouchDB.Timeout)
	}

	// Test Agent defaults
	if cfg.Agent.Enabled != false {
		t.Errorf("Expected default agent enabled false, got %v", cfg.Agent.Enabled)
	}
	if cfg.Agent.APIURL != "http://localhost:8080" {
		t.Errorf("Expected default api_url 'http://localhost:8080', got '%s'", cfg.Agent.APIURL)
	}
	if cfg.Agent.SyncInterval != 30*time.Second {
		t.Errorf("Expected default sync interval 30s, got %v", cfg.Agent.SyncInterval)
	}
	if cfg.Agent.DockerSocket != "/var/run/docker.sock" {
		t.Errorf("Expected default docker socket '/var/run/docker.sock', got '%s'", cfg.Agent.DockerSocket)
	}

	// Test Logging defaults
	if cfg.Logging.Level != "info" {
		t.Errorf("Expected default logging level 'info', got '%s'", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("Expected default logging format 'json', got '%s'", cfg.Logging.Format)
	}
	if cfg.Logging.Output != "stdout" {
		t.Errorf("Expected default logging output 'stdout', got '%s'", cfg.Logging.Output)
	}
	if cfg.Logging.MaxSize != 100 {
		t.Errorf("Expected default max size 100, got %d", cfg.Logging.MaxSize)
	}
	if cfg.Logging.MaxBackups != 3 {
		t.Errorf("Expected default max backups 3, got %d", cfg.Logging.MaxBackups)
	}
	if cfg.Logging.MaxAge != 7 {
		t.Errorf("Expected default max age 7, got %d", cfg.Logging.MaxAge)
	}

	// Test Security defaults
	if cfg.Security.RateLimit != 100 {
		t.Errorf("Expected default rate limit 100, got %d", cfg.Security.RateLimit)
	}
	if len(cfg.Security.AllowedOrigins) != 1 || cfg.Security.AllowedOrigins[0] != "*" {
		t.Errorf("Expected default allowed origins ['*'], got %v", cfg.Security.AllowedOrigins)
	}
	if cfg.Security.AuthEnabled != false {
		t.Errorf("Expected default auth_enabled false, got %v", cfg.Security.AuthEnabled)
	}
	if cfg.Security.JWTSecret != "change-me-in-production" {
		t.Errorf("Expected default jwt_secret 'change-me-in-production', got '%s'", cfg.Security.JWTSecret)
	}
	if cfg.Security.JWTExpiration != 24*time.Hour {
		t.Errorf("Expected default jwt expiration 24h, got %v", cfg.Security.JWTExpiration)
	}
	if cfg.Security.RefreshTokenExpiration != 168*time.Hour {
		t.Errorf("Expected default refresh token expiration 168h, got %v", cfg.Security.RefreshTokenExpiration)
	}
	if cfg.Security.AgentTokenSecret != "change-me-in-production" {
		t.Errorf("Expected default agent_token_secret 'change-me-in-production', got '%s'", cfg.Security.AgentTokenSecret)
	}
}

// TestValidation tests the configuration validation logic.
func TestValidation(t *testing.T) {
	tests := []struct {
		name      string
		cfg       *Config
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid configuration",
			cfg: &Config{
				Server: ServerConfig{
					Port: 8080,
				},
				CouchDB: CouchDBConfig{
					URL:      "http://localhost:5984",
					Database: "graphium",
				},
			},
			expectErr: false,
		},
		{
			name: "invalid port - too low",
			cfg: &Config{
				Server: ServerConfig{
					Port: 0,
				},
				CouchDB: CouchDBConfig{
					URL:      "http://localhost:5984",
					Database: "graphium",
				},
			},
			expectErr: true,
			errMsg:    "invalid server port",
		},
		{
			name: "invalid port - too high",
			cfg: &Config{
				Server: ServerConfig{
					Port: 70000,
				},
				CouchDB: CouchDBConfig{
					URL:      "http://localhost:5984",
					Database: "graphium",
				},
			},
			expectErr: true,
			errMsg:    "invalid server port",
		},
		{
			name: "missing couchdb url",
			cfg: &Config{
				Server: ServerConfig{
					Port: 8080,
				},
				CouchDB: CouchDBConfig{
					URL:      "",
					Database: "graphium",
				},
			},
			expectErr: true,
			errMsg:    "couchdb url is required",
		},
		{
			name: "missing couchdb database",
			cfg: &Config{
				Server: ServerConfig{
					Port: 8080,
				},
				CouchDB: CouchDBConfig{
					URL:      "http://localhost:5984",
					Database: "",
				},
			},
			expectErr: true,
			errMsg:    "couchdb database is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(tt.cfg)
			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tt.errMsg)
				} else if !contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

// TestBuildURL tests the BuildURL method of CouchDBConfig.
func TestBuildURL(t *testing.T) {
	tests := []struct {
		name     string
		config   CouchDBConfig
		expected string
	}{
		{
			name: "with credentials",
			config: CouchDBConfig{
				URL:      "http://localhost:5984",
				Username: "admin",
				Password: "secret",
			},
			expected: "http://admin:secret@localhost:5984",
		},
		{
			name: "https with credentials",
			config: CouchDBConfig{
				URL:      "https://db.example.com:5984",
				Username: "user",
				Password: "pass123",
			},
			expected: "https://user:pass123@db.example.com:5984",
		},
		{
			name: "without credentials",
			config: CouchDBConfig{
				URL:      "http://localhost:5984",
				Username: "",
				Password: "",
			},
			expected: "http://localhost:5984",
		},
		{
			name: "with username but no password",
			config: CouchDBConfig{
				URL:      "http://localhost:5984",
				Username: "admin",
				Password: "",
			},
			expected: "http://localhost:5984",
		},
		{
			name: "with password but no username",
			config: CouchDBConfig{
				URL:      "http://localhost:5984",
				Username: "",
				Password: "secret",
			},
			expected: "http://localhost:5984",
		},
		{
			name: "url already contains credentials",
			config: CouchDBConfig{
				URL:      "http://existing:creds@localhost:5984",
				Username: "admin",
				Password: "secret",
			},
			expected: "http://admin:secret@existing:creds@localhost:5984",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.BuildURL()
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// TestEnvironmentVariableOverride tests that environment variables override config values.
func TestEnvironmentVariableOverride(t *testing.T) {
	// Save original env vars
	originalPort := os.Getenv("CG_SERVER_PORT")
	originalHost := os.Getenv("CG_SERVER_HOST")
	originalDebug := os.Getenv("CG_SERVER_DEBUG")

	// Set test env vars
	os.Setenv("CG_SERVER_PORT", "9999")
	os.Setenv("CG_SERVER_HOST", "127.0.0.1")
	os.Setenv("CG_SERVER_DEBUG", "true")

	// Cleanup after test
	defer func() {
		if originalPort != "" {
			os.Setenv("CG_SERVER_PORT", originalPort)
		} else {
			os.Unsetenv("CG_SERVER_PORT")
		}
		if originalHost != "" {
			os.Setenv("CG_SERVER_HOST", originalHost)
		} else {
			os.Unsetenv("CG_SERVER_HOST")
		}
		if originalDebug != "" {
			os.Setenv("CG_SERVER_DEBUG", originalDebug)
		} else {
			os.Unsetenv("CG_SERVER_DEBUG")
		}
	}()

	cfg, err := Load("nonexistent.yaml")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Server.Port != 9999 {
		t.Errorf("Expected port 9999 from environment, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("Expected host '127.0.0.1' from environment, got '%s'", cfg.Server.Host)
	}
	if cfg.Server.Debug != true {
		t.Errorf("Expected debug true from environment, got %v", cfg.Server.Debug)
	}
}

// TestGet tests the global config getter.
func TestGet(t *testing.T) {
	// Load configuration first
	_, err := Load("nonexistent.yaml")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Get should return the loaded config
	retrieved := Get()
	if retrieved == nil {
		t.Error("Get() returned nil")
		return
	}

	// Verify it's the same instance
	if retrieved.Server.Port != 8080 {
		t.Errorf("Expected port 8080 from Get(), got %d", retrieved.Server.Port)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
