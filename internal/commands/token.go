package commands

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"evalgo.org/graphium/internal/auth"
)

var tokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Manage authentication tokens",
	Long:  `Generate and manage authentication tokens for agents and users`,
}

var generateAgentTokenCmd = &cobra.Command{
	Use:   "agent [host-id]",
	Short: "Generate an agent authentication token",
	Long: `Generate a JWT token for agent authentication.

The token is signed with the agent_token_secret from the configuration file
and includes the host ID in the claims. By default, tokens expire after 1 year.

Examples:
  # Generate token for localhost-docker
  graphium token agent localhost-docker

  # Generate token with custom expiration (in hours)
  graphium token agent localhost-docker --expiration 8760

  # Use custom secret (overrides config)
  graphium token agent localhost-docker --secret "my-custom-secret"`,
	Args: cobra.ExactArgs(1),
	RunE: runGenerateAgentToken,
}

var (
	tokenExpiration int64
	tokenSecret     string
)

func init() {
	// Add flags to generate command
	generateAgentTokenCmd.Flags().Int64Var(&tokenExpiration, "expiration", 8760, "Token expiration in hours (default: 8760 = 1 year)")
	generateAgentTokenCmd.Flags().StringVar(&tokenSecret, "secret", "", "Agent token secret (default: from config file)")

	// Add subcommands
	tokenCmd.AddCommand(generateAgentTokenCmd)
}

func runGenerateAgentToken(cmd *cobra.Command, args []string) error {
	hostID := args[0]

	// Get secret from flag or config
	secret := tokenSecret
	if secret == "" {
		// Get from loaded config struct
		if cfg != nil {
			// Use agent_token_secret if provided, otherwise fall back to jwt_secret
			secret = cfg.Security.AgentTokenSecret
			if secret == "" {
				secret = cfg.Security.JWTSecret
			}
		}

		if secret == "" {
			return fmt.Errorf(`jwt_secret not found in config file and --secret not provided

Please either:
  1. Add to your config.yaml:
     security:
       jwt_secret: your-secret-here

  2. Or use the --secret flag:
     graphium token agent %s --secret "your-secret-here"`, hostID)
		}
	}

	// Convert expiration from hours to duration
	expiration := time.Duration(tokenExpiration) * time.Hour

	// Generate token
	token, err := auth.GenerateAgentToken(secret, hostID, expiration)
	if err != nil {
		return fmt.Errorf("failed to generate token: %w", err)
	}

	// Print token information
	fmt.Printf("Agent Token Generated Successfully\n")
	fmt.Printf("==================================\n\n")
	fmt.Printf("Host ID:    %s\n", hostID)
	fmt.Printf("Expiration: %s (%d hours)\n", expiration, tokenExpiration)
	fmt.Printf("\nToken:\n%s\n\n", token)
	fmt.Printf("Add this to your agent configuration:\n")
	fmt.Printf("  agent:\n")
	fmt.Printf("    agent_token: %s\n\n", token)
	fmt.Printf("⚠️  Keep this token secure! It grants full agent access to your Graphium instance.\n")

	return nil
}
