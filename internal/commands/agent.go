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

	// These should never fail as flags are defined above
	_ = viper.BindPFlag("agent.api_url", agentCmd.Flags().Lookup("api-url"))             //nolint:errcheck
	_ = viper.BindPFlag("agent.host_id", agentCmd.Flags().Lookup("host-id"))             //nolint:errcheck
	_ = viper.BindPFlag("agent.datacenter", agentCmd.Flags().Lookup("datacenter"))       //nolint:errcheck
	_ = viper.BindPFlag("agent.docker_socket", agentCmd.Flags().Lookup("docker-socket")) //nolint:errcheck
}

func runAgent(cmd *cobra.Command, args []string) error {
	fmt.Println("🤖 Starting Graphium Agent")
	fmt.Printf("   Version: %s\n", rootCmd.Version)
	fmt.Printf("   Host ID: %s\n", cfg.Agent.HostID)
	fmt.Printf("   Datacenter: %s\n", cfg.Agent.Datacenter)
	fmt.Printf("   API URL: %s\n", cfg.Agent.APIURL)
	fmt.Println()

	// Generate agent authentication token if agent secret is configured
	var agentToken string
	if cfg.Security.AgentTokenSecret != "" && cfg.Security.AuthEnabled {
		// Generate a long-lived token (7 days)
		token, err := auth.GenerateAgentToken(
			cfg.Security.AgentTokenSecret,
			cfg.Agent.HostID,
			7*24*time.Hour,
		)
		if err != nil {
			return fmt.Errorf("failed to generate agent token: %w", err)
		}
		agentToken = token
	}

	a, err := agent.NewAgent(
		cfg.Agent.APIURL,
		cfg.Agent.HostID,
		cfg.Agent.Datacenter,
		cfg.Agent.DockerSocket,
		agentToken,
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
