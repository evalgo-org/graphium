package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management",
}

var showConfigCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE:  runShowConfig,
}

var initConfigCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize configuration file",
	RunE:  runInitConfig,
}

func init() {
	configCmd.AddCommand(showConfigCmd)
	configCmd.AddCommand(initConfigCmd)
}

func runShowConfig(cmd *cobra.Command, args []string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}

func runInitConfig(cmd *cobra.Command, args []string) error {
	defaultConfig := `# Graphium Configuration

server:
  host: 0.0.0.0
  port: 8080
  read_timeout: 30s
  write_timeout: 30s
  shutdown_timeout: 10s
  debug: false

couchdb:
  url: http://localhost:5984
  database: graphium
  username: admin
  password: password
  max_connections: 10
  timeout: 30

agent:
  enabled: false
  api_url: http://localhost:8080
  sync_interval: 30s
  docker_socket: /var/run/docker.sock

logging:
  level: info
  format: json
  output: stdout

security:
  rate_limit: 100
  allowed_origins:
    - "*"
`

	if err := os.WriteFile("config.yaml", []byte(defaultConfig), 0644); err != nil {
		return err
	}

	fmt.Println("✓ Created config.yaml")
	return nil
}
