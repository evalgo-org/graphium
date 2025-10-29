package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/docker/docker/client"
	"github.com/spf13/cobra"
	"evalgo.org/graphium/internal/orchestration"
	"evalgo.org/graphium/internal/storage"
	"evalgo.org/graphium/models"
	"eve.evalgo.org/containers/stacks"
)

var (
	stackName         string
	stackDefinition   string
	placementStrategy string
	datacenter        string
	targetHosts       []string
	removeVolumes     bool
	outputFormat      string
)

var stackCmd = &cobra.Command{
	Use:   "stack",
	Short: "Manage distributed container stacks",
	Long: `Manage multi-container stacks deployed across multiple hosts.

Graphium supports distributed stack deployment with multiple placement strategies:
  - auto: Automatic resource-based placement
  - manual: User-defined host assignments
  - spread: Even distribution across hosts
  - datacenter: Keep containers in same datacenter`,
}

var stackDeployCmd = &cobra.Command{
	Use:   "deploy <definition-file>",
	Short: "Deploy a stack from a definition file",
	Long: `Deploy a multi-container stack across multiple Docker hosts.

The stack definition file must be in JSON-LD format following the schema.org ItemList structure.

Examples:
  # Deploy with automatic placement
  graphium stack deploy my-app-stack.json --strategy auto

  # Deploy to specific hosts
  graphium stack deploy my-app-stack.json --strategy manual --hosts host1,host2

  # Deploy to a specific datacenter
  graphium stack deploy my-app-stack.json --strategy datacenter --datacenter us-west-2`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		definitionPath := args[0]

		// Initialize config and storage
		store, err := storage.New(cfg)
		if err != nil {
			return fmt.Errorf("failed to initialize storage: %w", err)
		}
		defer store.Close()

		// Load stack definition
		definition, err := stacks.LoadStackFromFile(definitionPath)
		if err != nil {
			return fmt.Errorf("failed to load stack definition: %w", err)
		}

		// Determine stack name
		name := stackName
		if name == "" {
			name = definition.Name
		}

		// Create stack model
		stack := &models.Stack{
			Context:     "https://schema.org",
			Type:        "ItemList",
			ID:          fmt.Sprintf("stack-%s-%d", name, time.Now().Unix()),
			Name:        name,
			Description: fmt.Sprintf("Deployed from %s", definitionPath),
			Status:      "pending",
			Datacenter:  datacenter,
			Deployment: models.DeploymentConfig{
				Mode:              "multi-host",
				PlacementStrategy: placementStrategy,
				NetworkMode:       "host-port",
			},
			DefinitionPath: definitionPath,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
			Owner:          "cli",
		}

		// Save stack
		if err := store.SaveStack(stack); err != nil {
			return fmt.Errorf("failed to save stack: %w", err)
		}

		fmt.Printf("Stack %s created (ID: %s)\n", stack.Name, stack.ID)

		// Get target hosts
		hosts, err := getTargetHostsForDeploy(cmd.Context(), store, targetHosts, datacenter)
		if err != nil {
			stack.Status = "error"
			stack.ErrorMessage = fmt.Sprintf("Failed to get target hosts: %v", err)
			store.UpdateStack(stack)
			return fmt.Errorf("failed to get target hosts: %w", err)
		}

		fmt.Printf("Deploying to %d host(s)...\n", len(hosts))

		// Create orchestrator
		orch := orchestration.NewDistributedStackOrchestrator(store)
		defer orch.Close()

		// Register hosts
		for _, hostInfo := range hosts {
			if err := orch.RegisterHost(hostInfo.Host, hostInfo.DockerSocket); err != nil {
				stack.Status = "error"
				stack.ErrorMessage = fmt.Sprintf("Failed to register host %s: %v", hostInfo.Host.ID, err)
				store.UpdateStack(stack)
				return fmt.Errorf("failed to register host %s: %w", hostInfo.Host.ID, err)
			}
		}

		// Deploy stack
		deployment, err := orch.DeployStack(cmd.Context(), stack, definition, hosts)
		if err != nil {
			return fmt.Errorf("failed to deploy stack: %w", err)
		}

		fmt.Printf("\n✓ Stack deployed successfully!\n\n")
		fmt.Printf("Stack ID:     %s\n", deployment.StackID)
		fmt.Printf("Status:       %s\n", deployment.Status)
		fmt.Printf("Containers:   %d\n", len(deployment.Placements))
		fmt.Printf("\nContainer Placements:\n")

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "CONTAINER\tHOST\tIP ADDRESS\tPORTS")
		for name, placement := range deployment.Placements {
			ports := ""
			for containerPort, hostPort := range placement.Ports {
				if ports != "" {
					ports += ", "
				}
				ports += fmt.Sprintf("%d:%d", containerPort, hostPort)
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", name, placement.HostID, placement.IPAddress, ports)
		}
		w.Flush()

		return nil
	},
}

var stackListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all stacks",
	Long:  "List all deployed stacks with their current status.",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Initialize storage
		store, err := storage.New(cfg)
		if err != nil {
			return fmt.Errorf("failed to initialize storage: %w", err)
		}
		defer store.Close()

		// Build filters
		filters := make(map[string]interface{})
		if datacenter != "" {
			filters["location"] = datacenter
		}

		// List stacks
		stacks, err := store.ListStacks(filters)
		if err != nil {
			return fmt.Errorf("failed to list stacks: %w", err)
		}

		if len(stacks) == 0 {
			fmt.Println("No stacks found.")
			return nil
		}

		// Output based on format
		if outputFormat == "json" {
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(stacks)
		}

		// Table format
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tSTATUS\tDATACENTER\tCONTAINERS\tCREATED")
		for _, stack := range stacks {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\n",
				stack.ID,
				stack.Name,
				stack.Status,
				stack.Datacenter,
				len(stack.Containers),
				stack.CreatedAt.Format("2006-01-02 15:04:05"),
			)
		}
		w.Flush()

		return nil
	},
}

var stackStatusCmd = &cobra.Command{
	Use:   "status <stack-id>",
	Short: "Show detailed status of a stack",
	Long:  "Show detailed status information including container placements and network configuration.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		stackID := args[0]

		// Initialize storage
		store, err := storage.New(cfg)
		if err != nil {
			return fmt.Errorf("failed to initialize storage: %w", err)
		}
		defer store.Close()

		// Get stack
		stack, err := store.GetStack(stackID)
		if err != nil {
			return fmt.Errorf("stack not found: %w", err)
		}

		// Output based on format
		if outputFormat == "json" {
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(stack)
		}

		// Human-readable format
		fmt.Printf("Stack Information:\n")
		fmt.Printf("  ID:                %s\n", stack.ID)
		fmt.Printf("  Name:              %s\n", stack.Name)
		fmt.Printf("  Description:       %s\n", stack.Description)
		fmt.Printf("  Status:            %s\n", stack.Status)
		fmt.Printf("  Datacenter:        %s\n", stack.Datacenter)
		fmt.Printf("  Placement Strategy: %s\n", stack.Deployment.PlacementStrategy)
		fmt.Printf("  Network Mode:      %s\n", stack.Deployment.NetworkMode)
		fmt.Printf("  Created:           %s\n", stack.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("  Updated:           %s\n", stack.UpdatedAt.Format("2006-01-02 15:04:05"))

		if stack.DeployedAt != nil {
			fmt.Printf("  Deployed:          %s\n", stack.DeployedAt.Format("2006-01-02 15:04:05"))
		}

		if stack.ErrorMessage != "" {
			fmt.Printf("  Error:             %s\n", stack.ErrorMessage)
		}

		// Get deployment info
		deployment, err := store.GetDeployment(stackID)
		if err == nil {
			fmt.Printf("\nDeployment Information:\n")
			fmt.Printf("  Status:            %s\n", deployment.Status)
			fmt.Printf("  Started:           %s\n", deployment.StartedAt.Format("2006-01-02 15:04:05"))
			if deployment.CompletedAt != nil {
				fmt.Printf("  Completed:         %s\n", deployment.CompletedAt.Format("2006-01-02 15:04:05"))
			}

			if len(deployment.Placements) > 0 {
				fmt.Printf("\nContainer Placements:\n")
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "  CONTAINER\tHOST\tIP ADDRESS\tSTATUS\tPORTS")
				for name, placement := range deployment.Placements {
					ports := ""
					for containerPort, hostPort := range placement.Ports {
						if ports != "" {
							ports += ", "
						}
						ports += fmt.Sprintf("%d:%d", containerPort, hostPort)
					}
					fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\n",
						name, placement.HostID, placement.IPAddress, placement.Status, ports)
				}
				w.Flush()
			}

			if len(deployment.NetworkConfig.ServiceEndpoints) > 0 {
				fmt.Printf("\nService Endpoints:\n")
				for service, endpoint := range deployment.NetworkConfig.ServiceEndpoints {
					fmt.Printf("  %s: %s\n", service, endpoint)
				}
			}
		}

		return nil
	},
}

var stackStopCmd = &cobra.Command{
	Use:   "stop <stack-id>",
	Short: "Stop a running stack",
	Long:  "Stop all containers in a stack without removing them.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		stackID := args[0]

		// Initialize storage
		store, err := storage.New(cfg)
		if err != nil {
			return fmt.Errorf("failed to initialize storage: %w", err)
		}
		defer store.Close()

		// Get deployment to find hosts
		deployment, err := store.GetDeployment(stackID)
		if err != nil {
			return fmt.Errorf("deployment not found: %w", err)
		}

		// Create orchestrator
		orch := orchestration.NewDistributedStackOrchestrator(store)
		defer orch.Close()

		// Register hosts
		for _, placement := range deployment.Placements {
			host, err := store.GetHost(placement.HostID)
			if err != nil {
				continue
			}
			dockerSocket := fmt.Sprintf("tcp://%s:2375", host.IPAddress)
			if host.IPAddress == "localhost" || host.IPAddress == "127.0.0.1" {
				dockerSocket = "unix:///var/run/docker.sock"
			}
			orch.RegisterHost(host, dockerSocket)
		}

		fmt.Printf("Stopping stack %s...\n", stackID)

		// Stop stack
		if err := orch.StopStack(cmd.Context(), stackID); err != nil {
			return fmt.Errorf("failed to stop stack: %w", err)
		}

		fmt.Printf("✓ Stack stopped successfully\n")
		return nil
	},
}

var stackRemoveCmd = &cobra.Command{
	Use:   "remove <stack-id>",
	Short: "Remove a stack",
	Long:  "Remove a stack and all its containers. Optionally remove volumes.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		stackID := args[0]

		// Initialize storage
		store, err := storage.New(cfg)
		if err != nil {
			return fmt.Errorf("failed to initialize storage: %w", err)
		}
		defer store.Close()

		// Get deployment to find hosts
		deployment, err := store.GetDeployment(stackID)
		if err != nil {
			return fmt.Errorf("deployment not found: %w", err)
		}

		// Create orchestrator
		orch := orchestration.NewDistributedStackOrchestrator(store)
		defer orch.Close()

		// Register hosts
		for _, placement := range deployment.Placements {
			host, err := store.GetHost(placement.HostID)
			if err != nil {
				continue
			}
			dockerSocket := fmt.Sprintf("tcp://%s:2375", host.IPAddress)
			if host.IPAddress == "localhost" || host.IPAddress == "127.0.0.1" {
				dockerSocket = "unix:///var/run/docker.sock"
			}
			orch.RegisterHost(host, dockerSocket)
		}

		fmt.Printf("Removing stack %s", stackID)
		if removeVolumes {
			fmt.Printf(" (including volumes)")
		}
		fmt.Printf("...\n")

		// Remove stack
		if err := orch.RemoveStack(cmd.Context(), stackID, removeVolumes); err != nil {
			return fmt.Errorf("failed to remove stack: %w", err)
		}

		fmt.Printf("✓ Stack removed successfully\n")
		return nil
	},
}

func init() {
	// Deploy command flags
	stackDeployCmd.Flags().StringVarP(&stackName, "name", "n", "", "Stack name (defaults to name in definition file)")
	stackDeployCmd.Flags().StringVarP(&placementStrategy, "strategy", "s", "auto", "Placement strategy (auto, manual, spread, datacenter)")
	stackDeployCmd.Flags().StringVarP(&datacenter, "datacenter", "d", "", "Target datacenter")
	stackDeployCmd.Flags().StringSliceVarP(&targetHosts, "hosts", "H", nil, "Target host IDs (comma-separated)")

	// List command flags
	stackListCmd.Flags().StringVarP(&datacenter, "datacenter", "d", "", "Filter by datacenter")
	stackListCmd.Flags().StringVarP(&outputFormat, "output", "o", "table", "Output format (table, json)")

	// Status command flags
	stackStatusCmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text, json)")

	// Remove command flags
	stackRemoveCmd.Flags().BoolVarP(&removeVolumes, "volumes", "v", false, "Remove volumes")

	// Add subcommands
	stackCmd.AddCommand(stackDeployCmd)
	stackCmd.AddCommand(stackListCmd)
	stackCmd.AddCommand(stackStatusCmd)
	stackCmd.AddCommand(stackStopCmd)
	stackCmd.AddCommand(stackRemoveCmd)
}

// Helper function to get target hosts for deployment
func getTargetHostsForDeploy(ctx context.Context, store *storage.Storage, targetHostIDs []string, datacenter string) ([]*models.HostInfo, error) {
	var hosts []*models.Host

	// If specific hosts requested, get those
	if len(targetHostIDs) > 0 {
		for _, hostID := range targetHostIDs {
			host, err := store.GetHost(hostID)
			if err != nil {
				return nil, fmt.Errorf("host %s not found: %w", hostID, err)
			}
			hosts = append(hosts, host)
		}
	} else if datacenter != "" {
		// Get all hosts in datacenter
		dcHosts, err := store.ListHosts(map[string]interface{}{
			"location": datacenter,
			"status":   "active",
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list hosts in datacenter %s: %w", datacenter, err)
		}
		hosts = dcHosts
	} else {
		// Get all active hosts
		allHosts, err := store.ListHosts(map[string]interface{}{
			"status": "active",
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list active hosts: %w", err)
		}
		hosts = allHosts
	}

	if len(hosts) == 0 {
		return nil, fmt.Errorf("no suitable hosts found")
	}

	// Convert to HostInfo
	hostInfos := make([]*models.HostInfo, len(hosts))
	for i, host := range hosts {
		dockerSocket := fmt.Sprintf("tcp://%s:2375", host.IPAddress)
		if host.IPAddress == "localhost" || host.IPAddress == "127.0.0.1" {
			dockerSocket = "unix:///var/run/docker.sock"
		}

		// Try to get resource info
		cli, err := client.NewClientWithOpts(
			client.WithHost(dockerSocket),
			client.WithAPIVersionNegotiation(),
		)
		if err != nil {
			// Use placeholder values
			hostInfos[i] = &models.HostInfo{
				Host:         host,
				DockerSocket: dockerSocket,
				CurrentLoad: models.ResourceLoad{
					CPUUsage:       0,
					MemoryUsage:    0,
					ContainerCount: 0,
				},
				AvailableResources: models.Resources{
					CPU:    host.CPU,
					Memory: host.Memory,
				},
				Labels: make(map[string]string),
			}
			continue
		}

		hostInfo, err := orchestration.GetHostResourceInfo(ctx, cli, host)
		cli.Close()

		if err != nil {
			// Use placeholder values
			hostInfos[i] = &models.HostInfo{
				Host:         host,
				DockerSocket: dockerSocket,
				CurrentLoad: models.ResourceLoad{
					CPUUsage:       0,
					MemoryUsage:    0,
					ContainerCount: 0,
				},
				AvailableResources: models.Resources{
					CPU:    host.CPU,
					Memory: host.Memory,
				},
				Labels: make(map[string]string),
			}
			continue
		}

		hostInfos[i] = hostInfo
	}

	return hostInfos, nil
}
