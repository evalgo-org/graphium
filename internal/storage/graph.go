package storage

import (
	"encoding/json"
	"fmt"

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

	// For each host, get its containers
	for _, host := range hosts {
		containers, err := s.GetContainersByHost(host.ID)
		if err != nil {
			// Skip hosts we can't get containers for
			continue
		}

		topology.Hosts[host.ID] = &HostTopology{
			Host:       host,
			Containers: containers,
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
	Host       *models.Host
	Containers []*models.Container
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
	// Add type filter
	selector := map[string]interface{}{
		"@type": "SoftwareApplication",
	}

	// Merge additional filters
	for k, v := range filter {
		selector[k] = v
	}

	return s.service.Count(selector)
}

// CountHosts returns the total number of hosts matching the filter.
func (s *Storage) CountHosts(filter map[string]interface{}) (int, error) {
	// Add type filter
	selector := map[string]interface{}{
		"@type": "ComputerServer",
	}

	// Merge additional filters
	for k, v := range filter {
		selector[k] = v
	}

	return s.service.Count(selector)
}

// GetStatistics returns aggregated statistics about the infrastructure.
func (s *Storage) GetStatistics() (*Statistics, error) {
	stats := &Statistics{}

	// Count total containers
	totalContainers, err := s.CountContainers(nil)
	if err == nil {
		stats.TotalContainers = totalContainers
	}

	// Count running containers
	runningContainers, err := s.CountContainers(map[string]interface{}{
		"status": "running",
	})
	if err == nil {
		stats.RunningContainers = runningContainers
	}

	// Count total hosts
	totalHosts, err := s.CountHosts(nil)
	if err == nil {
		stats.TotalHosts = totalHosts
	}

	// Get host container distribution
	hostCounts, err := s.GetHostContainerCount()
	if err == nil {
		stats.HostContainerCounts = hostCounts
	}

	return stats, nil
}

// Statistics contains aggregated infrastructure statistics.
type Statistics struct {
	TotalContainers     int
	RunningContainers   int
	TotalHosts          int
	HostContainerCounts map[string]int
}

// String returns a formatted string representation of the statistics.
func (s *Statistics) String() string {
	return fmt.Sprintf(
		"Statistics:\n"+
			"  Total Containers: %d\n"+
			"  Running Containers: %d\n"+
			"  Total Hosts: %d\n"+
			"  Hosts with Containers: %d\n",
		s.TotalContainers,
		s.RunningContainers,
		s.TotalHosts,
		len(s.HostContainerCounts),
	)
}
