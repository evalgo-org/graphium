package storage

import (
	"encoding/json"

	"evalgo.org/graphium/models"
	"eve.evalgo.org/db"
)

// Statistics contains overview statistics for the dashboard
type Statistics struct {
	TotalContainers     int
	RunningContainers   int
	TotalHosts          int
	TotalStacks         int
	RunningStacks       int
	HostContainerCounts map[string]int // host ID -> container count
	TotalAgents         int
	RunningAgents       int
	TotalActions        int // Total scheduled actions
	SuccessfulActions   int // Scheduled actions with last execution successful
	FailedActions       int // Scheduled actions with last execution failed
}

// DatacenterTopology contains the topology information for a single datacenter
type DatacenterTopology struct {
	Datacenter string
	Hosts      map[string]*HostTopology
}

// HostTopology contains information about a host and its containers
type HostTopology struct {
	Host           *models.Host
	Containers     []*models.Container
	ContainerCount int
}

// GetStatistics calculates and returns infrastructure statistics
func (s *Storage) GetStatistics() (*Statistics, error) {
	stats := &Statistics{
		HostContainerCounts: make(map[string]int),
	}

	// Get all containers
	containers, err := s.ListContainers(nil)
	if err != nil {
		return nil, err
	}
	stats.TotalContainers = len(containers)

	// Count running containers and build host container counts
	for _, container := range containers {
		if container.Status == "running" {
			stats.RunningContainers++
		}
		if container.HostedOn != "" {
			stats.HostContainerCounts[container.HostedOn]++
		}
	}

	// Get all hosts
	hosts, err := s.ListHosts(nil)
	if err != nil {
		return nil, err
	}
	stats.TotalHosts = len(hosts)

	// Get all stacks
	stacks, err := s.ListStacks(nil)
	if err != nil {
		return nil, err
	}
	stats.TotalStacks = len(stacks)

	// Count running stacks (stacks with at least one running container)
	for _, stack := range stacks {
		hasRunning := false
		for _, containerID := range stack.Containers {
			container, err := s.GetContainer(containerID)
			if err == nil && container.Status == "running" {
				hasRunning = true
				break
			}
		}
		if hasRunning {
			stats.RunningStacks++
		}
	}

	// Get agent statistics
	agentConfigs, err := s.ListAgentConfigs(nil)
	if err == nil {
		stats.TotalAgents = len(agentConfigs)
		// For now, we'll consider all configured agents as running
		// In the future, this could check actual agent status
		stats.RunningAgents = len(agentConfigs)
	}

	// Get scheduled actions statistics
	actions, err := s.ListScheduledActions(nil)
	if err == nil {
		stats.TotalActions = len(actions)

		// For each action, get the most recent task to determine success/failure
		for _, action := range actions {
			tasks, err := s.GetTasksByScheduledAction(action.ID)
			if err != nil || len(tasks) == 0 {
				continue
			}

			// Find the most recent completed task
			var mostRecentTask *models.AgentTask
			for _, task := range tasks {
				if task.Status == "completed" || task.Status == "failed" {
					if mostRecentTask == nil || (task.CompletedAt != nil && mostRecentTask.CompletedAt != nil && task.CompletedAt.After(*mostRecentTask.CompletedAt)) {
						mostRecentTask = task
					}
				}
			}

			// Count success or failure based on most recent task
			if mostRecentTask != nil {
				switch mostRecentTask.Status {
				case "completed":
					stats.SuccessfulActions++
				case "failed":
					stats.FailedActions++
				}
			}
		}
	}

	return stats, nil
}

// GetDatacenterTopology returns the topology for a specific datacenter
func (s *Storage) GetDatacenterTopology(datacenter string) (*DatacenterTopology, error) {
	topology := &DatacenterTopology{
		Datacenter: datacenter,
		Hosts:      make(map[string]*HostTopology),
	}

	// Get all hosts in this datacenter
	filters := map[string]interface{}{
		"location": datacenter,
	}
	hosts, err := s.ListHosts(filters)
	if err != nil {
		return nil, err
	}

	// For each host, count containers
	for _, host := range hosts {
		containers, err := s.GetContainersByHost(host.ID)
		if err != nil {
			containers = []*models.Container{}
		}

		topology.Hosts[host.ID] = &HostTopology{
			Host:           host,
			Containers:     containers,
			ContainerCount: len(containers),
		}
	}

	return topology, nil
}

// CountContainers returns the count of containers matching the given filters
func (s *Storage) CountContainers(filters map[string]interface{}) (int, error) {
	containers, err := s.ListContainers(filters)
	if err != nil {
		return 0, err
	}
	return len(containers), nil
}

// CountHosts returns the count of hosts matching the given filters
func (s *Storage) CountHosts(filters map[string]interface{}) (int, error) {
	hosts, err := s.ListHosts(filters)
	if err != nil {
		return 0, err
	}
	return len(hosts), nil
}

// GetContainerDependencyGraph builds a dependency graph for a container
// This is a simplified version that looks for network dependencies
func (s *Storage) GetContainerDependencyGraph(containerID string, maxDepth int) (*db.RelationshipGraph, error) {
	graph := &db.RelationshipGraph{
		Nodes: make(map[string]json.RawMessage),
		Edges: []db.RelationshipEdge{},
	}

	// Get the root container
	container, err := s.GetContainer(containerID)
	if err != nil {
		return nil, err
	}

	// Marshal container to JSON
	containerJSON, err := json.Marshal(container)
	if err != nil {
		return nil, err
	}
	graph.Nodes[containerID] = containerJSON

	// For now, return a simple graph with just the root node
	// A full implementation would traverse network links, volume dependencies, etc.

	return graph, nil
}
