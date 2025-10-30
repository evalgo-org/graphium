package storage

import (
	"encoding/json"
	"fmt"
	"log"

	"eve.evalgo.org/db"

	"evalgo.org/graphium/models"
)

// TraverseContainers traverses the container dependency graph starting from a given container.
// It follows the specified relationship field (e.g., "dependsOn", "hostedOn") up to maxDepth levels.
func (s *Storage) TraverseContainers(startID string, relationField string, maxDepth int) ([]*models.Container, error) {
	path, err := s.service.Traverse(db.TraversalOptions{
		StartID:       startID,
		RelationField: relationField,
		Direction:     "outbound",
		Depth:         maxDepth,
	})

	if err != nil {
		return nil, err
	}

	containers := make([]*models.Container, 0, len(path))
	for _, nodeData := range path {
		var container models.Container
		if err := json.Unmarshal(nodeData, &container); err != nil {
			continue // Skip invalid documents
		}
		containers = append(containers, &container)
	}

	return containers, nil
}

// GetContainerDependencyGraph returns the full dependency graph for a container.
// This includes all direct and transitive dependencies.
func (s *Storage) GetContainerDependencyGraph(containerID string, maxDepth int) (*db.RelationshipGraph, error) {
	return s.service.GetRelationshipGraph(containerID, "dependsOn", maxDepth)
}

// GetHostContainerGraph returns the graph of all containers on a host and their dependencies.
func (s *Storage) GetHostContainerGraph(hostID string, maxDepth int) (*db.RelationshipGraph, error) {
	// First get all containers on this host
	containers, err := s.GetContainersByHost(hostID)
	if err != nil {
		return nil, err
	}

	if len(containers) == 0 {
		return &db.RelationshipGraph{
			Nodes: make(map[string]json.RawMessage),
			Edges: []db.RelationshipEdge{},
		}, nil
	}

	// Build a combined graph
	// Aggregate all nodes and edges from container graphs
	rootGraph := &db.RelationshipGraph{
		Nodes: make(map[string]json.RawMessage),
		Edges: []db.RelationshipEdge{},
	}

	for _, container := range containers {
		// Get dependency graph for each container
		containerGraph, err := s.GetContainerDependencyGraph(container.ID, maxDepth)
		if err != nil {
			// Skip containers we can't build graph for
			continue
		}

		// Merge nodes and edges
		for id, node := range containerGraph.Nodes {
			rootGraph.Nodes[id] = node
		}
		rootGraph.Edges = append(rootGraph.Edges, containerGraph.Edges...)
	}

	return rootGraph, nil
}

// FindImpactedContainers finds all containers that would be impacted if the given container changes.
// This performs a reverse dependency lookup to find what depends on this container.
func (s *Storage) FindImpactedContainers(containerID string) ([]*models.Container, error) {
	return s.GetContainerDependents(containerID)
}

// TraverseInfrastructure traverses the infrastructure hierarchy (datacenter → host → container).
func (s *Storage) TraverseInfrastructure(startID string, maxDepth int) ([]json.RawMessage, error) {
	// Traverse through hostedOn relationships
	return s.service.Traverse(db.TraversalOptions{
		StartID:       startID,
		RelationField: "hostedOn",
		Direction:     "outbound",
		Depth:         maxDepth,
	})
}

// GetDatacenterTopology returns the complete topology of a datacenter.
// This includes all hosts and containers in the datacenter.
func (s *Storage) GetDatacenterTopology(datacenterName string) (*DatacenterTopology, error) {
	// Get all hosts in datacenter
	hosts, err := s.GetHostsByDatacenter(datacenterName)
	if err != nil {
		return nil, err
	}

	topology := &DatacenterTopology{
		Datacenter: datacenterName,
		Hosts:      make(map[string]*HostTopology),
	}

	// Get container counts for all hosts in a single query
	hostContainerCounts, err := s.GetHostContainerCount()
	if err != nil {
		// If we can't get counts, just use 0
		hostContainerCounts = make(map[string]int)
	}

	// Build topology with container counts
	for _, host := range hosts {
		// Create empty container list - we only need the count now
		// The UI will link to the filtered containers page
		containers := make([]*models.Container, 0)

		// Get count from the map, defaults to 0 if not found
		containerCount := hostContainerCounts[host.ID]

		topology.Hosts[host.ID] = &HostTopology{
			Host:           host,
			Containers:     containers,
			ContainerCount: containerCount,
		}
	}

	return topology, nil
}

// DatacenterTopology represents the complete view of a datacenter's infrastructure.
type DatacenterTopology struct {
	Datacenter string
	Hosts      map[string]*HostTopology
}

// HostTopology represents a host and all its containers.
type HostTopology struct {
	Host           *models.Host
	Containers     []*models.Container
	ContainerCount int // Number of containers on this host
}

// GetContainersByFilter performs a complex query with multiple conditions.
func (s *Storage) GetContainersByFilter(filter ContainerFilter) ([]*models.Container, error) {
	qb := db.NewQueryBuilder().
		Where("@type", "$eq", "SoftwareApplication")

	if filter.Status != "" {
		qb = qb.And().Where("status", "$eq", filter.Status)
	}

	if filter.HostID != "" {
		qb = qb.And().Where("hostedOn", "$eq", filter.HostID)
	}

	if filter.Image != "" {
		qb = qb.And().Where("executableName", "$regex", filter.Image)
	}

	if filter.Limit > 0 {
		qb = qb.Limit(filter.Limit)
	}

	query := qb.Build()
	containers, err := db.FindTyped[models.Container](s.service, query)
	if err != nil {
		return nil, err
	}

	// Convert to pointer slice
	result := make([]*models.Container, len(containers))
	for i := range containers {
		result[i] = &containers[i]
	}

	return result, nil
}

// GetHostsByFilter performs a complex query for hosts.
func (s *Storage) GetHostsByFilter(filter HostFilter) ([]*models.Host, error) {
	qb := db.NewQueryBuilder().
		Where("@type", "$eq", "ComputerServer")

	if filter.Status != "" {
		qb = qb.And().Where("status", "$eq", filter.Status)
	}

	if filter.Datacenter != "" {
		qb = qb.And().Where("location", "$eq", filter.Datacenter)
	}

	if filter.Limit > 0 {
		qb = qb.Limit(filter.Limit)
	}

	query := qb.Build()
	hosts, err := db.FindTyped[models.Host](s.service, query)
	if err != nil {
		return nil, err
	}

	// Convert to pointer slice
	result := make([]*models.Host, len(hosts))
	for i := range hosts {
		result[i] = &hosts[i]
	}

	return result, nil
}

// ContainerFilter defines filtering criteria for container queries.
type ContainerFilter struct {
	Status string
	HostID string
	Image  string
	Limit  int
}

// HostFilter defines filtering criteria for host queries.
type HostFilter struct {
	Status     string
	Datacenter string
	Limit      int
}

// CountContainers returns the total number of containers matching the filter.
func (s *Storage) CountContainers(filter map[string]interface{}) (int, error) {
	// We can't use Count() directly because CouchDB may have duplicate documents
	// for the same container ID (from sync conflicts or agent restarts).
	// ListContainers has deduplication logic, so use that instead.
	containers, err := s.ListContainers(filter)
	if err != nil {
		return 0, err
	}
	return len(containers), nil
}

// CountHosts returns the total number of hosts matching the filter.
func (s *Storage) CountHosts(filter map[string]interface{}) (int, error) {
	// We can't use Count() directly because CouchDB may have duplicate documents.
	// ListHosts doesn't have explicit deduplication but it's less critical for hosts.
	hosts, err := s.ListHosts(filter)
	if err != nil {
		return 0, err
	}
	return len(hosts), nil
}

// GetStatistics returns aggregated statistics about the infrastructure.
func (s *Storage) GetStatistics() (*Statistics, error) {
	stats := &Statistics{}

	// Count total containers
	totalContainers, err := s.CountContainers(nil)
	if err == nil {
		stats.TotalContainers = totalContainers
	} else {
		log.Printf("Error counting total containers: %v", err)
	}

	// Count running containers
	runningContainers, err := s.CountContainers(map[string]interface{}{
		"status": "running",
	})
	if err == nil {
		stats.RunningContainers = runningContainers
	} else {
		log.Printf("Error counting running containers: %v", err)
	}

	s.debugLog("STATS DEBUG: Total containers: %d, Running containers: %d", stats.TotalContainers, stats.RunningContainers)

	// Count total hosts
	totalHosts, err := s.CountHosts(nil)
	if err == nil {
		stats.TotalHosts = totalHosts
	}

	// Get host container distribution
	hostCounts, err := s.GetHostContainerCount()
	if err == nil {
		stats.HostContainerCounts = hostCounts
		s.debugLog("STATS DEBUG: Host container counts: %v", hostCounts)
	}

	// Count total stacks
	allStacks, err := s.ListStacks(nil)
	if err == nil {
		stats.TotalStacks = len(allStacks)
		// Count running stacks
		runningCount := 0
		for _, stack := range allStacks {
			if stack.Status == "running" {
				runningCount++
			}
		}
		stats.RunningStacks = runningCount
	}

	return stats, nil
}

// Statistics contains aggregated infrastructure statistics.
type Statistics struct {
	TotalContainers     int
	RunningContainers   int
	TotalHosts          int
	TotalStacks         int
	RunningStacks       int
	HostContainerCounts map[string]int
}

// String returns a formatted string representation of the statistics.
func (s *Statistics) String() string {
	return fmt.Sprintf(
		"Statistics:\n"+
			"  Total Containers: %d\n"+
			"  Running Containers: %d\n"+
			"  Total Hosts: %d\n"+
			"  Total Stacks: %d\n"+
			"  Running Stacks: %d\n"+
			"  Hosts with Containers: %d\n",
		s.TotalContainers,
		s.RunningContainers,
		s.TotalHosts,
		s.TotalStacks,
		s.RunningStacks,
		len(s.HostContainerCounts),
	)
}
