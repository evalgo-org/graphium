package orchestration

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"evalgo.org/graphium/models"
	"eve.evalgo.org/containers/stacks"
)

// DistributedStackOrchestrator orchestrates distributed stack deployments.
type DistributedStackOrchestrator struct {
	clientManager *DockerClientManager
	storage       StackStorage
}

// StackStorage defines the interface for persisting stack data.
type StackStorage interface {
	SaveStack(stack *models.Stack) error
	GetStack(id string) (*models.Stack, error)
	UpdateStack(stack *models.Stack) error
	DeleteStack(id string) error

	SaveDeployment(deployment *models.StackDeployment) error
	GetDeployment(stackID string) (*models.StackDeployment, error)
	UpdateDeployment(deployment *models.StackDeployment) error
}

// NewDistributedStackOrchestrator creates a new orchestrator.
func NewDistributedStackOrchestrator(storage StackStorage) *DistributedStackOrchestrator {
	return &DistributedStackOrchestrator{
		clientManager: NewDockerClientManager(),
		storage:       storage,
	}
}

// RegisterHost registers a Docker host for deployment.
func (o *DistributedStackOrchestrator) RegisterHost(host *models.Host, dockerSocket string) error {
	return o.clientManager.AddHost(host.ID, dockerSocket)
}

// DeployStack deploys a stack across multiple hosts.
func (o *DistributedStackOrchestrator) DeployStack(
	ctx context.Context,
	stack *models.Stack,
	definition *stacks.Stack,
	hosts []*models.HostInfo,
) (*models.StackDeployment, error) {
	// Update stack status
	stack.Status = "deploying"
	if err := o.storage.UpdateStack(stack); err != nil {
		return nil, fmt.Errorf("failed to update stack status: %w", err)
	}

	// Create deployment record
	deployment := &models.StackDeployment{
		StackID:   stack.ID,
		Placements: make(map[string]models.ContainerPlacement),
		NetworkConfig: models.NetworkConfig{
			Mode:                 stack.Deployment.NetworkMode,
			ServiceEndpoints:     make(map[string]string),
			EnvironmentVariables: make(map[string]map[string]string),
		},
		StartedAt: time.Now(),
		Status:    "in_progress",
	}

	// Determine placement strategy
	strategy, err := o.getPlacementStrategy(stack.Deployment.PlacementStrategy)
	if err != nil {
		return nil, fmt.Errorf("failed to get placement strategy: %w", err)
	}

	// Determine container placement
	placements, err := strategy.PlaceContainers(ctx, stack, definition, hosts)
	if err != nil {
		stack.Status = "error"
		stack.ErrorMessage = fmt.Sprintf("placement failed: %v", err)
		o.storage.UpdateStack(stack)
		return nil, fmt.Errorf("placement failed: %w", err)
	}

	// Group containers by host
	hostGroups := o.groupContainersByHost(definition, placements)

	// Prepare cross-host networking
	if err := o.prepareCrossHostNetworking(deployment, hostGroups, hosts); err != nil {
		stack.Status = "error"
		stack.ErrorMessage = fmt.Sprintf("network preparation failed: %v", err)
		o.storage.UpdateStack(stack)
		return nil, fmt.Errorf("network preparation failed: %w", err)
	}

	// Deploy containers to each host
	if err := o.deployToHosts(ctx, stack, definition, hostGroups, deployment, hosts); err != nil {
		stack.Status = "error"
		stack.ErrorMessage = fmt.Sprintf("deployment failed: %v", err)
		o.storage.UpdateStack(stack)
		return nil, fmt.Errorf("deployment failed: %w", err)
	}

	// Update deployment status
	now := time.Now()
	deployment.CompletedAt = &now
	deployment.Status = "completed"

	if err := o.storage.SaveDeployment(deployment); err != nil {
		return nil, fmt.Errorf("failed to save deployment: %w", err)
	}

	// Update stack status
	stack.Status = "running"
	stack.DeployedAt = &now
	stack.ErrorMessage = ""
	if err := o.storage.UpdateStack(stack); err != nil {
		return nil, fmt.Errorf("failed to update stack status: %w", err)
	}

	return deployment, nil
}

// getPlacementStrategy returns the appropriate placement strategy.
func (o *DistributedStackOrchestrator) getPlacementStrategy(strategyName string) (PlacementStrategy, error) {
	switch strategyName {
	case "auto":
		return &AutoPlacementStrategy{}, nil
	case "manual":
		return &ManualPlacementStrategy{}, nil
	case "spread":
		return &SpreadPlacementStrategy{}, nil
	case "datacenter":
		return &DatacenterPlacementStrategy{}, nil
	default:
		return &ManualPlacementStrategy{}, nil // Default to manual
	}
}

// groupContainersByHost groups containers by their target host.
func (o *DistributedStackOrchestrator) groupContainersByHost(
	definition *stacks.Stack,
	placements map[string]string,
) map[string][]stacks.StackItemElement {
	groups := make(map[string][]stacks.StackItemElement)

	for _, item := range definition.ItemListElement {
		hostID := placements[item.Name]
		groups[hostID] = append(groups[hostID], item)
	}

	return groups
}

// prepareCrossHostNetworking configures networking for cross-host communication.
func (o *DistributedStackOrchestrator) prepareCrossHostNetworking(
	deployment *models.StackDeployment,
	hostGroups map[string][]stacks.StackItemElement,
	hosts []*models.HostInfo,
) error {
	// Build host map
	hostMap := make(map[string]*models.HostInfo)
	for _, host := range hosts {
		hostMap[host.Host.ID] = host
	}

	// For host-port networking mode, prepare service endpoints
	if deployment.NetworkConfig.Mode == "" || deployment.NetworkConfig.Mode == "host-port" {
		deployment.NetworkConfig.Mode = "host-port"

		for hostID, containers := range hostGroups {
			host := hostMap[hostID]
			for _, container := range containers {
				// Map service endpoints (will be filled after deployment)
				if len(container.Ports) > 0 {
					// Use first port for the service endpoint
					port := container.Ports[0]
					endpoint := fmt.Sprintf("%s:%d", host.Host.IPAddress, port.HostPort)
					deployment.NetworkConfig.ServiceEndpoints[container.Name] = endpoint
				}
			}
		}
	}

	return nil
}

// deployToHosts deploys containers to their assigned hosts.
func (o *DistributedStackOrchestrator) deployToHosts(
	ctx context.Context,
	stack *models.Stack,
	definition *stacks.Stack,
	hostGroups map[string][]stacks.StackItemElement,
	deployment *models.StackDeployment,
	hosts []*models.HostInfo,
) error {
	// Build host map
	hostMap := make(map[string]*models.HostInfo)
	for _, host := range hosts {
		hostMap[host.Host.ID] = host
	}

	// Deploy to each host
	for hostID, containers := range hostGroups {
		host := hostMap[hostID]

		// Get Docker client for this host
		cli, err := o.clientManager.GetClient(hostID)
		if err != nil {
			return fmt.Errorf("failed to get client for host %s: %w", hostID, err)
		}

		// Deploy each container on this host
		for _, containerDef := range containers {
			// Inject cross-host environment variables
			env := o.injectCrossHostEnv(containerDef, deployment)
			containerDef.Environment = mergeEnv(containerDef.Environment, env)

			// Deploy container using EVE's production deployment
			containerID, err := o.deployContainer(ctx, cli, containerDef, stack, definition)
			if err != nil {
				return fmt.Errorf("failed to deploy container %s on host %s: %w",
					containerDef.Name, hostID, err)
			}

			// Record placement
			now := time.Now()
			placement := models.ContainerPlacement{
				ContainerID:   containerID,
				ContainerName: containerDef.Name,
				HostID:        hostID,
				IPAddress:     host.Host.IPAddress,
				Ports:         make(map[int]int),
				Status:        "running",
				StartedAt:     &now,
			}

			// Map ports
			for _, portMapping := range containerDef.Ports {
				placement.Ports[portMapping.ContainerPort] = portMapping.HostPort
			}

			deployment.Placements[containerDef.Name] = placement
		}
	}

	return nil
}

// injectCrossHostEnv creates environment variables for cross-host connections.
func (o *DistributedStackOrchestrator) injectCrossHostEnv(
	container stacks.StackItemElement,
	deployment *models.StackDeployment,
) map[string]string {
	env := make(map[string]string)

	// Inject service endpoints for dependencies
	for serviceName, endpoint := range deployment.NetworkConfig.ServiceEndpoints {
		if serviceName == container.Name {
			continue // Don't inject self
		}

		// Create environment variables in standard format
		// Example: POSTGRES_HOST=192.168.1.10, POSTGRES_PORT=5432
		envKey := fmt.Sprintf("%s_ENDPOINT", toEnvName(serviceName))
		env[envKey] = endpoint
	}

	return env
}

// deployContainer deploys a single container using Docker client directly.
func (o *DistributedStackOrchestrator) deployContainer(
	ctx context.Context,
	cli *client.Client,
	containerDef stacks.StackItemElement,
	stack *models.Stack,
	definition *stacks.Stack,
) (string, error) {
	// Build container configuration
	config := &container.Config{
		Image: containerDef.Image,
		Env:   make([]string, 0, len(containerDef.Environment)),
	}

	// Convert environment map to slice
	for key, value := range containerDef.Environment {
		config.Env = append(config.Env, fmt.Sprintf("%s=%s", key, value))
	}

	// Create container
	containerName := fmt.Sprintf("%s-%s", stack.Name, containerDef.Name)
	resp, err := cli.ContainerCreate(ctx, config, nil, nil, nil, containerName)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	// Start container
	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("failed to start container: %w", err)
	}

	return resp.ID, nil
}

// StopStack stops a distributed stack.
func (o *DistributedStackOrchestrator) StopStack(
	ctx context.Context,
	stackID string,
) error {
	// Get stack
	stack, err := o.storage.GetStack(stackID)
	if err != nil {
		return fmt.Errorf("failed to get stack: %w", err)
	}

	// Get deployment
	deployment, err := o.storage.GetDeployment(stackID)
	if err != nil {
		return fmt.Errorf("failed to get deployment: %w", err)
	}

	// Update stack status
	stack.Status = "stopping"
	o.storage.UpdateStack(stack)

	// Stop containers on each host
	for _, placement := range deployment.Placements {
		cli, err := o.clientManager.GetClient(placement.HostID)
		if err != nil {
			return fmt.Errorf("failed to get client for host %s: %w", placement.HostID, err)
		}

		if err := cli.ContainerStop(ctx, placement.ContainerID, container.StopOptions{}); err != nil {
			return fmt.Errorf("failed to stop container %s: %w", placement.ContainerName, err)
		}
	}

	// Update stack status
	stack.Status = "stopped"
	return o.storage.UpdateStack(stack)
}

// RemoveStack removes a distributed stack.
func (o *DistributedStackOrchestrator) RemoveStack(
	ctx context.Context,
	stackID string,
	removeVolumes bool,
) error {
	// Get deployment
	deployment, err := o.storage.GetDeployment(stackID)
	if err != nil {
		return fmt.Errorf("failed to get deployment: %w", err)
	}

	// Remove containers on each host
	for _, placement := range deployment.Placements {
		cli, err := o.clientManager.GetClient(placement.HostID)
		if err != nil {
			return fmt.Errorf("failed to get client for host %s: %w", placement.HostID, err)
		}

		if err := cli.ContainerRemove(ctx, placement.ContainerID, container.RemoveOptions{
			RemoveVolumes: removeVolumes,
			Force:         true,
		}); err != nil {
			return fmt.Errorf("failed to remove container %s: %w", placement.ContainerName, err)
		}
	}

	// Delete stack from storage
	return o.storage.DeleteStack(stackID)
}

// Close closes the orchestrator and all Docker clients.
func (o *DistributedStackOrchestrator) Close() error {
	return o.clientManager.Close()
}

// Helper functions

func toEnvName(name string) string {
	// Convert service name to environment variable format
	// Example: "my-service" -> "MY_SERVICE"
	result := strings.ReplaceAll(name, "-", "_")
	result = strings.ReplaceAll(result, ".", "_")
	return strings.ToUpper(result)
}

func mergeEnv(base, overlay map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range base {
		result[k] = v
	}
	for k, v := range overlay {
		result[k] = v
	}
	return result
}
