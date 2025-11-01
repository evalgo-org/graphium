package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"evalgo.org/graphium/agent"
	"evalgo.org/graphium/internal/auth"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Start the Docker agent",
	Long:  `Start the agent that monitors Docker events and syncs with the API`,
	RunE:  runAgent,
}

func init() {
	agentCmd.Flags().String("api-url", "", "API server URL")
	agentCmd.Flags().String("host-id", "", "Unique host identifier")
	agentCmd.Flags().String("datacenter", "", "Datacenter name")
	agentCmd.Flags().String("docker-socket", "", "Docker socket path")
	agentCmd.Flags().Int("http-port", 0, "HTTP server port (0 = disabled)")

	// These should never fail as flags are defined above
	_ = viper.BindPFlag("agent.api_url", agentCmd.Flags().Lookup("api-url"))             //nolint:errcheck
	_ = viper.BindPFlag("agent.host_id", agentCmd.Flags().Lookup("host-id"))             //nolint:errcheck
	_ = viper.BindPFlag("agent.datacenter", agentCmd.Flags().Lookup("datacenter"))       //nolint:errcheck
	_ = viper.BindPFlag("agent.docker_socket", agentCmd.Flags().Lookup("docker-socket")) //nolint:errcheck
	_ = viper.BindPFlag("agent.http_port", agentCmd.Flags().Lookup("http-port"))         //nolint:errcheck
}

func runAgent(cmd *cobra.Command, args []string) error {
	// Get configuration values (command-line flags override config file)
	apiURL := viper.GetString("agent.api_url")
	hostID := viper.GetString("agent.host_id")
	datacenter := viper.GetString("agent.datacenter")
	dockerSocket := viper.GetString("agent.docker_socket")
	httpPort := viper.GetInt("agent.http_port")

	fmt.Println("🤖 Starting Graphium Agent")
	fmt.Printf("   Version: %s\n", rootCmd.Version)
	fmt.Printf("   Host ID: %s\n", hostID)
	fmt.Printf("   Datacenter: %s\n", datacenter)
	fmt.Printf("   API URL: %s\n", apiURL)
	if httpPort > 0 {
		fmt.Printf("   HTTP Port: %d\n", httpPort)
	}
	fmt.Println()

	// Get agent authentication token
	// Priority order:
	// 1. TOKEN environment variable (set by agent manager for managed agents)
	// 2. agent_token from config file (for standalone agents)
	// 3. Generate token if auth is enabled
	var agentToken string
	if envToken := os.Getenv("TOKEN"); envToken != "" {
		// Use token from environment (agent manager)
		agentToken = envToken
	} else if cfg.Agent.AgentToken != "" {
		// Use pre-configured token from config file
		agentToken = cfg.Agent.AgentToken
	} else if cfg.Security.AuthEnabled {
		// Use agent_token_secret if provided, otherwise fall back to jwt_secret
		secret := cfg.Security.AgentTokenSecret
		if secret == "" {
			secret = cfg.Security.JWTSecret
		}

		// Generate a long-lived token (7 days) if no token configured
		token, err := auth.GenerateAgentToken(
			secret,
			hostID,
			7*24*time.Hour,
		)
		if err != nil {
			return fmt.Errorf("failed to generate agent token: %w", err)
		}
		agentToken = token
	}

	a, err := agent.NewAgent(
		apiURL,
		hostID,
		datacenter,
		dockerSocket,
		agentToken,
		httpPort,
	)
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Println("✓ Agent started")
	fmt.Println("   Monitoring Docker events...")
	fmt.Println()

	go func() {
		if err := a.Start(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Agent error: %v\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	fmt.Println("\n🛑 Stopping agent...")
	cancel()

	fmt.Println("✓ Agent stopped")
	return nil
}
