package orchestration

import (
	"context"
	"fmt"
	"sort"
	"strings"

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
//
// Enhanced features:
//   - Port conflict detection
//   - Container dependency awareness (places dependent containers closer)
//   - Resource reservation tracking during placement
//   - Multi-factor scoring (CPU, memory, network, load, affinity)
type AutoPlacementStrategy struct{}

// PlaceContainers implements automatic resource-based placement with intelligent container placement.
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

	// Track resource reservations as we place containers
	reservations := make(map[string]*resourceReservation)
	for _, host := range hosts {
		reservations[host.Host.ID] = &resourceReservation{
			reservedPorts:   make(map[int]bool),
			reservedMemory:  0,
			containerCount:  host.CurrentLoad.ContainerCount,
		}
	}

	// Build dependency graph for intelligent placement
	dependencies := s.buildDependencyGraph(definition)

	// Place each container with intelligent ordering
	// Process containers in dependency order (dependencies first)
	orderedContainers := s.orderContainersByDependencies(definition.ItemListElement, dependencies)

	for _, item := range orderedContainers {
		containerName := item.Name

		// Score each host for this container considering all factors
		scores := s.scoreHostsEnhanced(ctx, item, hosts, stack, placement, dependencies, reservations)

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

		// Update resource reservations for next placement
		reservation := reservations[bestHost]
		for _, port := range item.Ports {
			reservation.reservedPorts[port.HostPort] = true
		}
		// Estimate memory usage (1GB default if not specified)
		reservation.reservedMemory += 1073741824 // 1GB in bytes
		reservation.containerCount++
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

// resourceReservation tracks resources reserved on a host during placement.
type resourceReservation struct {
	reservedPorts  map[int]bool
	reservedMemory int64
	containerCount int
}

// buildDependencyGraph analyzes container dependencies based on environment variables
// and service references. Containers that reference other services are dependent on them.
func (s *AutoPlacementStrategy) buildDependencyGraph(definition *stacks.Stack) map[string][]string {
	dependencies := make(map[string][]string)

	// Build map of service names
	serviceNames := make(map[string]bool)
	for _, item := range definition.ItemListElement {
		serviceNames[item.Name] = true
	}

	// Analyze each container for dependencies
	for _, item := range definition.ItemListElement {
		deps := []string{}

		// Check environment variables for service references
		for key, value := range item.Environment {
			// Look for references to other services
			// Common patterns: SERVICE_HOST, SERVICE_URL, etc.
			for serviceName := range serviceNames {
				if serviceName != item.Name && strings.Contains(value, serviceName) {
					deps = append(deps, serviceName)
				}
				// Check key names like POSTGRES_HOST, REDIS_HOST, etc.
				serviceUpper := strings.ToUpper(strings.ReplaceAll(serviceName, "-", "_"))
				keyUpper := strings.ToUpper(key)
				if strings.Contains(keyUpper, serviceUpper) {
					deps = append(deps, serviceName)
				}
			}
		}

		// Remove duplicates
		seen := make(map[string]bool)
		uniqueDeps := []string{}
		for _, dep := range deps {
			if !seen[dep] {
				seen[dep] = true
				uniqueDeps = append(uniqueDeps, dep)
			}
		}

		if len(uniqueDeps) > 0 {
			dependencies[item.Name] = uniqueDeps
		}
	}

	return dependencies
}

// orderContainersByDependencies orders containers so dependencies are placed first.
// This ensures that when we place a container, its dependencies are already placed,
// allowing us to optimize for locality.
func (s *AutoPlacementStrategy) orderContainersByDependencies(
	containers []stacks.StackItemElement,
	dependencies map[string][]string,
) []stacks.StackItemElement {
	// Build reverse dependency map (who depends on me)
	dependents := make(map[string][]string)
	for container, deps := range dependencies {
		for _, dep := range deps {
			dependents[dep] = append(dependents[dep], container)
		}
	}

	// Topological sort using Khan's algorithm
	inDegree := make(map[string]int)
	for _, container := range containers {
		inDegree[container.Name] = 0
	}
	for _, deps := range dependencies {
		for _, dep := range deps {
			inDegree[dep]++
		}
	}

	// Start with containers that have no dependencies
	queue := []string{}
	for _, container := range containers {
		if len(dependencies[container.Name]) == 0 {
			queue = append(queue, container.Name)
		}
	}

	// Process queue
	ordered := []string{}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		ordered = append(ordered, current)

		// Process dependents
		for _, dependent := range dependents[current] {
			deps := dependencies[dependent]
			// Remove current from dependent's dependencies
			newDeps := []string{}
			for _, dep := range deps {
				if dep != current {
					newDeps = append(newDeps, dep)
				}
			}
			dependencies[dependent] = newDeps

			// If dependent has no more dependencies, add to queue
			if len(newDeps) == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	// Build result maintaining original container objects
	containerMap := make(map[string]stacks.StackItemElement)
	for _, container := range containers {
		containerMap[container.Name] = container
	}

	result := make([]stacks.StackItemElement, 0, len(ordered))
	for _, name := range ordered {
		result = append(result, containerMap[name])
	}

	// Add any remaining containers (in case of cycles)
	if len(result) < len(containers) {
		for _, container := range containers {
			found := false
			for _, name := range ordered {
				if name == container.Name {
					found = true
					break
				}
			}
			if !found {
				result = append(result, container)
			}
		}
	}

	return result
}

// scoreHostsEnhanced scores hosts with enhanced logic considering dependencies,
// port conflicts, and resource reservations.
func (s *AutoPlacementStrategy) scoreHostsEnhanced(
	ctx context.Context,
	item stacks.StackItemElement,
	hosts []*models.HostInfo,
	stack *models.Stack,
	currentPlacements map[string]string,
	dependencies map[string][]string,
	reservations map[string]*resourceReservation,
) []hostScore {
	scores := make([]hostScore, 0, len(hosts))

	for _, host := range hosts {
		score := s.scoreHost(item, host, stack)
		if score == 0 {
			continue // Skip unsuitable hosts
		}

		reservation := reservations[host.Host.ID]

		// Check port conflicts (critical - must have available ports)
		hasPortConflict := false
		for _, port := range item.Ports {
			if reservation.reservedPorts[port.HostPort] {
				hasPortConflict = true
				break
			}
		}
		if hasPortConflict {
			continue // Skip this host entirely
		}

		// Adjust score based on available memory after reservations
		availableMemory := host.AvailableResources.Memory - reservation.reservedMemory
		if availableMemory < 536870912 { // Less than 512MB available
			score -= 30 // Significant penalty for low memory
		} else if availableMemory < 1073741824 { // Less than 1GB
			score -= 15
		}

		// Bonus for co-locating dependent containers (locality)
		affinityBonus := 0.0
		containerDeps := dependencies[item.Name]
		for _, dep := range containerDeps {
			if depHost, exists := currentPlacements[dep]; exists {
				if depHost == host.Host.ID {
					affinityBonus += 25.0 // Strong bonus for same host
				} else if depHost != "" {
					// Check if hosts are in same datacenter
					for _, h := range hosts {
						if h.Host.ID == depHost && h.Host.Datacenter == host.Host.Datacenter {
							affinityBonus += 10.0 // Moderate bonus for same datacenter
							break
						}
					}
				}
			}
		}
		score += affinityBonus

		// Penalty for overloading a single host (encourage spread)
		if reservation.containerCount >= 8 {
			score -= 20
		} else if reservation.containerCount >= 5 {
			score -= 10
		}

		scores = append(scores, hostScore{
			HostID: host.Host.ID,
			Score:  score,
		})
	}

	return scores
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
