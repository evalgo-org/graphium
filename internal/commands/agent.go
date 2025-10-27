package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"evalgo.org/graphium/agent"
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

	viper.BindPFlag("agent.api_url", agentCmd.Flags().Lookup("api-url"))
	viper.BindPFlag("agent.host_id", agentCmd.Flags().Lookup("host-id"))
	viper.BindPFlag("agent.datacenter", agentCmd.Flags().Lookup("datacenter"))
	viper.BindPFlag("agent.docker_socket", agentCmd.Flags().Lookup("docker-socket"))
}

func runAgent(cmd *cobra.Command, args []string) error {
	fmt.Println("🤖 Starting Graphium Agent")
	fmt.Printf("   Version: %s\n", rootCmd.Version)
	fmt.Printf("   Host ID: %s\n", cfg.Agent.HostID)
	fmt.Printf("   Datacenter: %s\n", cfg.Agent.Datacenter)
	fmt.Printf("   API URL: %s\n", cfg.Agent.APIURL)
	fmt.Println()

	a, err := agent.NewAgent(
		cfg.Agent.APIURL,
		cfg.Agent.HostID,
		cfg.Agent.Datacenter,
		cfg.Agent.DockerSocket,
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
