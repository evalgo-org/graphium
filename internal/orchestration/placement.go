package orchestration

import (
	"context"
	"fmt"
	"sort"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/client"
	"evalgo.org/graphium/models"
	"eve.evalgo.org/containers/stacks"
)

// PlacementStrategy defines how containers are placed on hosts.
type PlacementStrategy interface {
	// PlaceContainers determines which host each container should be placed on.
	// Returns a map of container name to host ID.
	PlaceContainers(
		ctx context.Context,
		stack *models.Stack,
		definition *stacks.Stack,
		hosts []*models.HostInfo,
	) (map[string]string, error)
}

// AutoPlacementStrategy places containers based on available resources.
// It scores each host and selects the best fit for each container.
type AutoPlacementStrategy struct{}

// PlaceContainers implements automatic resource-based placement.
func (s *AutoPlacementStrategy) PlaceContainers(
	ctx context.Context,
	stack *models.Stack,
	definition *stacks.Stack,
	hosts []*models.HostInfo,
) (map[string]string, error) {
	if len(hosts) == 0 {
		return nil, fmt.Errorf("no hosts available for placement")
	}

	placement := make(map[string]string)

	// Place each container
	for _, item := range definition.ItemListElement {
		containerName := item.Name

		// Score each host for this container
		scores := s.scoreHosts(ctx, item, hosts, stack)

		// Select host with highest score
		if len(scores) == 0 {
			return nil, fmt.Errorf("no suitable host found for container %s", containerName)
		}

		// Sort by score (highest first)
		sort.Slice(scores, func(i, j int) bool {
			return scores[i].Score > scores[j].Score
		})

		bestHost := scores[0].HostID
		placement[containerName] = bestHost

		// Update host's theoretical load for next placement
		// (This is a simplified approach; real implementation would track resource reservations)
	}

	return placement, nil
}

// hostScore represents a host's suitability score.
type hostScore struct {
	HostID string
	Score  float64
}

// scoreHosts scores all hosts for a specific container.
func (s *AutoPlacementStrategy) scoreHosts(
	ctx context.Context,
	item stacks.StackItemElement,
	hosts []*models.HostInfo,
	stack *models.Stack,
) []hostScore {
	scores := make([]hostScore, 0, len(hosts))

	for _, host := range hosts {
		score := s.scoreHost(item, host, stack)
		if score > 0 {
			scores = append(scores, hostScore{
				HostID: host.Host.ID,
				Score:  score,
			})
		}
	}

	return scores
}

// scoreHost calculates a score for a single host.
// Higher score = better fit.
// Score of 0 = host cannot accommodate container.
func (s *AutoPlacementStrategy) scoreHost(
	item stacks.StackItemElement,
	host *models.HostInfo,
	stack *models.Stack,
) float64 {
	score := 100.0 // Base score

	// Check datacenter preference
	if stack.Datacenter != "" && host.Host.Datacenter != stack.Datacenter {
		score -= 20 // Penalty for different datacenter
	}

	// Check host status
	if host.Host.Status != "active" {
		return 0 // Cannot use inactive host
	}

	// Score based on CPU availability (0-30 points)
	cpuScore := (1.0 - host.CurrentLoad.CPUUsage/100.0) * 30.0
	score += cpuScore

	// Score based on memory availability (0-30 points)
	if host.AvailableResources.Memory > 0 {
		memoryScore := float64(host.AvailableResources.Memory) / float64(host.Host.Memory) * 30.0
		score += memoryScore
	}

	// Bonus for fewer containers (spread load)
	if host.CurrentLoad.ContainerCount < 5 {
		score += 10
	} else if host.CurrentLoad.ContainerCount > 10 {
		score -= 10
	}

	return score
}

// ManualPlacementStrategy uses user-defined host constraints.
type ManualPlacementStrategy struct{}

// PlaceContainers implements manual placement based on host constraints.
func (s *ManualPlacementStrategy) PlaceContainers(
	ctx context.Context,
	stack *models.Stack,
	definition *stacks.Stack,
	hosts []*models.HostInfo,
) (map[string]string, error) {
	placement := make(map[string]string)
	hostMap := make(map[string]*models.HostInfo)

	// Build host map for quick lookup
	for _, host := range hosts {
		hostMap[host.Host.ID] = host
	}

	// Process each container
	for _, item := range definition.ItemListElement {
		containerName := item.Name

		// Find constraint for this container
		var constraint *models.HostConstraint
		for i := range stack.Deployment.HostConstraints {
			if stack.Deployment.HostConstraints[i].ContainerName == containerName {
				constraint = &stack.Deployment.HostConstraints[i]
				break
			}
		}

		if constraint == nil {
			return nil, fmt.Errorf("no host constraint defined for container %s", containerName)
		}

		// Verify target host exists
		if constraint.TargetHostID == "" {
			return nil, fmt.Errorf("no target host specified for container %s", containerName)
		}

		host, ok := hostMap[constraint.TargetHostID]
		if !ok {
			return nil, fmt.Errorf("target host %s not found for container %s",
				constraint.TargetHostID, containerName)
		}

		// Verify host constraints
		if err := s.verifyConstraints(constraint, host); err != nil {
			return nil, fmt.Errorf("host %s does not meet constraints for container %s: %w",
				constraint.TargetHostID, containerName, err)
		}

		placement[containerName] = constraint.TargetHostID
	}

	return placement, nil
}

// verifyConstraints checks if a host meets the constraints.
func (s *ManualPlacementStrategy) verifyConstraints(
	constraint *models.HostConstraint,
	host *models.HostInfo,
) error {
	// Check datacenter
	if constraint.RequiredDatacenter != "" &&
		host.Host.Datacenter != constraint.RequiredDatacenter {
		return fmt.Errorf("datacenter mismatch: required=%s, actual=%s",
			constraint.RequiredDatacenter, host.Host.Datacenter)
	}

	// Check minimum CPU
	if constraint.MinCPU > 0 && host.AvailableResources.CPU < constraint.MinCPU {
		return fmt.Errorf("insufficient CPU: required=%d, available=%d",
			constraint.MinCPU, host.AvailableResources.CPU)
	}

	// Check minimum memory
	if constraint.MinMemory > 0 && host.AvailableResources.Memory < constraint.MinMemory {
		return fmt.Errorf("insufficient memory: required=%d, available=%d",
			constraint.MinMemory, host.AvailableResources.Memory)
	}

	// Check labels
	for key, requiredValue := range constraint.Labels {
		actualValue, ok := host.Labels[key]
		if !ok {
			return fmt.Errorf("required label %s not found", key)
		}
		if actualValue != requiredValue {
			return fmt.Errorf("label %s mismatch: required=%s, actual=%s",
				key, requiredValue, actualValue)
		}
	}

	return nil
}

// SpreadPlacementStrategy distributes containers evenly across hosts.
type SpreadPlacementStrategy struct{}

// PlaceContainers implements spread placement.
func (s *SpreadPlacementStrategy) PlaceContainers(
	ctx context.Context,
	stack *models.Stack,
	definition *stacks.Stack,
	hosts []*models.HostInfo,
) (map[string]string, error) {
	if len(hosts) == 0 {
		return nil, fmt.Errorf("no hosts available for placement")
	}

	placement := make(map[string]string)
	containerCounts := make(map[string]int)

	// Initialize container counts
	for _, host := range hosts {
		containerCounts[host.Host.ID] = host.CurrentLoad.ContainerCount
	}

	// Place each container on host with fewest containers
	for _, item := range definition.ItemListElement {
		containerName := item.Name

		// Find host with fewest containers
		var selectedHost string
		minCount := -1

		for _, host := range hosts {
			// Skip inactive hosts
			if host.Host.Status != "active" {
				continue
			}

			count := containerCounts[host.Host.ID]
			if minCount == -1 || count < minCount {
				minCount = count
				selectedHost = host.Host.ID
			}
		}

		if selectedHost == "" {
			return nil, fmt.Errorf("no active host found for container %s", containerName)
		}

		placement[containerName] = selectedHost
		containerCounts[selectedHost]++
	}

	return placement, nil
}

// DatacenterPlacementStrategy keeps containers in the same datacenter.
type DatacenterPlacementStrategy struct{}

// PlaceContainers implements datacenter-aware placement.
func (s *DatacenterPlacementStrategy) PlaceContainers(
	ctx context.Context,
	stack *models.Stack,
	definition *stacks.Stack,
	hosts []*models.HostInfo,
) (map[string]string, error) {
	if stack.Datacenter == "" {
		return nil, fmt.Errorf("datacenter not specified in stack configuration")
	}

	// Filter hosts by datacenter
	dcHosts := make([]*models.HostInfo, 0)
	for _, host := range hosts {
		if host.Host.Datacenter == stack.Datacenter {
			dcHosts = append(dcHosts, host)
		}
	}

	if len(dcHosts) == 0 {
		return nil, fmt.Errorf("no hosts available in datacenter %s", stack.Datacenter)
	}

	// Use spread strategy within the datacenter
	spread := &SpreadPlacementStrategy{}
	return spread.PlaceContainers(ctx, stack, definition, dcHosts)
}

// GetHostResourceInfo fetches current resource usage for a host.
func GetHostResourceInfo(ctx context.Context, cli *client.Client, host *models.Host) (*models.HostInfo, error) {
	// Get Docker system info
	info, err := cli.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get Docker info: %w", err)
	}

	// Get container list
	containers, err := cli.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	// Calculate resource usage
	hostInfo := &models.HostInfo{
		Host:         host,
		DockerSocket: cli.DaemonHost(),
		CurrentLoad: models.ResourceLoad{
			CPUUsage:       calculateCPUUsage(info),
			MemoryUsage:    calculateMemoryUsage(info),
			ContainerCount: len(containers),
		},
		AvailableResources: models.Resources{
			CPU:    int(info.NCPU),
			Memory: info.MemTotal,
		},
		Labels: make(map[string]string),
	}

	return hostInfo, nil
}

// calculateCPUUsage estimates CPU usage percentage.
// This is a simplified implementation; real monitoring would use metrics APIs.
func calculateCPUUsage(info system.Info) float64 {
	// For now, return a placeholder
	// Real implementation would query metrics from Docker stats API or node exporter
	return 0.0
}

// calculateMemoryUsage calculates used memory.
func calculateMemoryUsage(info system.Info) int64 {
	return info.MemTotal - int64(info.MemTotal) // Placeholder
}
