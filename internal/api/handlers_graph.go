package api

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"evalgo.org/graphium/internal/storage"
	"evalgo.org/graphium/models"
)

// GraphNode represents a node in the visualization graph
type GraphNode struct {
	Data GraphNodeData `json:"data"`
}

// GraphNodeData contains the node's properties
type GraphNodeData struct {
	ID             string            `json:"id"`
	Label          string            `json:"label"`
	Type           string            `json:"type"` // "stack", "host", "container"
	Status         string            `json:"status,omitempty"`
	Image          string            `json:"image,omitempty"`
	IP             string            `json:"ip,omitempty"`
	CPU            int               `json:"cpu,omitempty"`
	Memory         int64             `json:"memory,omitempty"`
	Location       string            `json:"location,omitempty"`

	// Stack-specific fields
	ContainerCount int               `json:"containerCount,omitempty"`
	HostCount      int               `json:"hostCount,omitempty"`

	// Cytoscape.js compound node support
	Parent         string            `json:"parent,omitempty"` // Parent node ID for nesting

	Metadata       map[string]string `json:"metadata,omitempty"`
}

// GraphEdge represents an edge (relationship) in the graph
type GraphEdge struct {
	Data GraphEdgeData `json:"data"`
}

// GraphEdgeData contains the edge's properties
type GraphEdgeData struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
	Label  string `json:"label,omitempty"`
	Type   string `json:"type"` // "hosted_on", "connects_to", "depends_on"
}

// GraphData represents the complete graph structure
type GraphData struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// GetGraphData returns the graph visualization data
// @Summary Get graph data
// @Description Get graph visualization data with nodes and edges
// @Tags Graph
// @Accept json
// @Produce json
// @Success 200 {object} GraphData
// @Failure 500 {object} ErrorResponse
// @Router /graph [get]
func (s *Server) GetGraphData(c echo.Context) error {
	// Get all hosts
	hosts, err := s.storage.ListHosts(nil)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to list hosts",
			Details: err.Error(),
		})
	}

	// Get all containers
	containers, err := s.storage.ListContainers(nil)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to list containers",
			Details: err.Error(),
		})
	}

	graphData := GraphData{
		Nodes: make([]GraphNode, 0),
		Edges: make([]GraphEdge, 0),
	}

	// Build a set of valid host IDs for edge validation
	validHosts := make(map[string]bool)
	for _, host := range hosts {
		validHosts[host.ID] = true
	}

	// Add host nodes
	for _, host := range hosts {
		node := GraphNode{
			Data: GraphNodeData{
				ID:       host.ID,
				Label:    host.Name,
				Type:     "host",
				Status:   host.Status,
				IP:       host.IPAddress,
				CPU:      host.CPU,
				Memory:   host.Memory,
				Location: host.Datacenter,
				Metadata: map[string]string{
					"@type": host.Type,
				},
			},
		}
		graphData.Nodes = append(graphData.Nodes, node)
	}

	// Build a set of valid container IDs for dependency edge validation
	validContainers := make(map[string]bool)
	for _, container := range containers {
		validContainers[container.ID] = true
	}

	// Add container nodes and edges
	for _, container := range containers {
		// Add container node
		node := GraphNode{
			Data: GraphNodeData{
				ID:     container.ID,
				Label:  container.Name,
				Type:   "container",
				Status: container.Status,
				Image:  container.Image,
				Metadata: map[string]string{
					"@type": container.Type,
				},
			},
		}
		graphData.Nodes = append(graphData.Nodes, node)

		// Add edge from container to host (only if host exists)
		if container.HostedOn != "" && validHosts[container.HostedOn] {
			edge := GraphEdge{
				Data: GraphEdgeData{
					ID:     container.ID + "-" + container.HostedOn,
					Source: container.ID,
					Target: container.HostedOn,
					Label:  "hosted on",
					Type:   "hosted_on",
				},
			}
			graphData.Edges = append(graphData.Edges, edge)
		}

		// Add dependency edges (only if dependency exists)
		for _, depID := range container.DependsOn {
			if validContainers[depID] {
				edge := GraphEdge{
					Data: GraphEdgeData{
						ID:     container.ID + "-depends-" + depID,
						Source: container.ID,
						Target: depID,
						Label:  "depends on",
						Type:   "depends_on",
					},
				}
				graphData.Edges = append(graphData.Edges, edge)
			}
		}
	}

	return c.JSON(http.StatusOK, graphData)
}

// GetGraphStats returns statistics about the graph
// @Summary Get graph stats
// @Description Get statistics about graph nodes and edges
// @Tags Graph
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} ErrorResponse
// @Router /graph/stats [get]
func (s *Server) GetGraphStats(c echo.Context) error {
	// Get all hosts
	hosts, err := s.storage.ListHosts(nil)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to list hosts",
			Details: err.Error(),
		})
	}

	// Get all containers
	containers, err := s.storage.ListContainers(nil)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to list containers",
			Details: err.Error(),
		})
	}

	// Count by status
	containersByStatus := make(map[string]int)
	hostsByStatus := make(map[string]int)

	for _, container := range containers {
		containersByStatus[container.Status]++
	}

	for _, host := range hosts {
		hostsByStatus[host.Status]++
	}

	// Count relationships
	relationships := 0
	dependencies := 0
	for _, container := range containers {
		if container.HostedOn != "" {
			relationships++
		}
		dependencies += len(container.DependsOn)
	}

	stats := map[string]interface{}{
		"nodes": map[string]interface{}{
			"total":      len(hosts) + len(containers),
			"hosts":      len(hosts),
			"containers": len(containers),
		},
		"edges": map[string]interface{}{
			"total":      relationships + dependencies,
			"hosted_on":  relationships,
			"depends_on": dependencies,
		},
		"containersByStatus": containersByStatus,
		"hostsByStatus":      hostsByStatus,
	}

	return c.JSON(http.StatusOK, stats)
}

// GetGraphLayout returns the graph with a specific layout applied
// @Summary Get graph layout
// @Description Get graph visualization data with specified layout algorithm applied
// @Tags Graph
// @Accept json
// @Produce json
// @Param layout query string false "Layout algorithm" Enums(force, hierarchical, circle, grid)
// @Success 200 {object} GraphData
// @Failure 500 {object} ErrorResponse
// @Router /graph/layout [get]
func (s *Server) GetGraphLayout(c echo.Context) error {
	layout := c.QueryParam("layout")
	if layout == "" {
		layout = "force"
	}

	// Get base graph data
	var graphData GraphData

	// This is a simplified version - in production, you'd calculate actual positions
	// For now, we'll just return the graph data with a layout hint
	// The client-side Cytoscape.js will handle the actual layout

	// Get all hosts
	hosts, err := s.storage.ListHosts(nil)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to list hosts",
			Details: err.Error(),
		})
	}

	// Get all containers
	containers, err := s.storage.ListContainers(nil)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to list containers",
			Details: err.Error(),
		})
	}

	graphData = GraphData{
		Nodes: make([]GraphNode, 0),
		Edges: make([]GraphEdge, 0),
	}

	// Build a set of valid host IDs for edge validation
	validHostsLayout := make(map[string]bool)
	for _, host := range hosts {
		validHostsLayout[host.ID] = true
	}

	// Add host nodes
	for _, host := range hosts {
		node := GraphNode{
			Data: GraphNodeData{
				ID:       host.ID,
				Label:    host.Name,
				Type:     "host",
				Status:   host.Status,
				Location: host.Datacenter,
				Metadata: map[string]string{
					"layout": layout,
				},
			},
		}
		graphData.Nodes = append(graphData.Nodes, node)
	}

	// Add container nodes and edges
	for _, container := range containers {
		node := GraphNode{
			Data: GraphNodeData{
				ID:     container.ID,
				Label:  container.Name,
				Type:   "container",
				Status: container.Status,
				Image:  container.Image,
			},
		}
		graphData.Nodes = append(graphData.Nodes, node)

		// Add edge only if host exists
		if container.HostedOn != "" && validHostsLayout[container.HostedOn] {
			edge := GraphEdge{
				Data: GraphEdgeData{
					ID:     container.ID + "-" + container.HostedOn,
					Source: container.ID,
					Target: container.HostedOn,
					Type:   "hosted_on",
				},
			}
			graphData.Edges = append(graphData.Edges, edge)
		}
	}

	return c.JSON(http.StatusOK, graphData)
}

// DependencyNode represents a node in the dependency tree.
type DependencyNode struct {
	Container *models.Container `json:"container"`
	Depth     int               `json:"depth"`
	Children  []*DependencyNode `json:"children,omitempty"`
}

// DependencyGraphResponse contains the full dependency graph for a container.
type DependencyGraphResponse struct {
	Container    *models.Container `json:"container"`
	Dependencies []*DependencyNode `json:"dependencies"` // Containers this one depends on
	Dependents   []*DependencyNode `json:"dependents"`   // Containers that depend on this one
}

// getContainerDependencies handles GET /api/v1/graph/containers/:id/dependencies
// @Summary Get container dependencies
// @Description Get all containers that the specified container depends on
// @Tags Graph
// @Accept json
// @Produce json
// @Param id path string true "Container ID"
// @Success 200 {object} ContainersResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /graph/containers/{id}/dependencies [get]
func (s *Server) getContainerDependencies(c echo.Context) error {
	id := c.Param("id")

	if id == "" {
		return BadRequestError("Container ID is required", "The 'id' parameter cannot be empty")
	}

	// Get the container
	container, err := s.storage.GetContainer(id)
	if err != nil {
		return NotFoundError("Container", id)
	}

	// If no dependencies, return empty list
	if len(container.DependsOn) == 0 {
		return c.JSON(http.StatusOK, ContainersResponse{
			Count:      0,
			Containers: []*models.Container{},
		})
	}

	// Resolve dependencies
	dependencies := make([]*models.Container, 0, len(container.DependsOn))
	for _, depID := range container.DependsOn {
		dep, err := s.storage.GetContainer(depID)
		if err != nil {
			// Dependency not found - skip but log warning
			continue
		}
		dependencies = append(dependencies, dep)
	}

	return c.JSON(http.StatusOK, ContainersResponse{
		Count:      len(dependencies),
		Containers: dependencies,
	})
}

// getContainerDependents handles GET /api/v1/graph/containers/:id/dependents
// @Summary Get container dependents
// @Description Get all containers that depend on the specified container
// @Tags Graph
// @Accept json
// @Produce json
// @Param id path string true "Container ID"
// @Success 200 {object} ContainersResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /graph/containers/{id}/dependents [get]
func (s *Server) getContainerDependents(c echo.Context) error {
	id := c.Param("id")

	if id == "" {
		return BadRequestError("Container ID is required", "The 'id' parameter cannot be empty")
	}

	// Check if container exists
	_, err := s.storage.GetContainer(id)
	if err != nil {
		return NotFoundError("Container", id)
	}

	// Find all containers that depend on this one
	dependents, err := s.storage.GetContainerDependents(id)
	if err != nil {
		return InternalError("Failed to get dependents", err.Error())
	}

	return c.JSON(http.StatusOK, ContainersResponse{
		Count:      len(dependents),
		Containers: dependents,
	})
}

// getContainerGraph handles GET /api/v1/graph/containers/:id/graph
// @Summary Get container dependency graph
// @Description Get the full dependency graph for a container (dependencies and dependents)
// @Tags Graph
// @Accept json
// @Produce json
// @Param id path string true "Container ID"
// @Param depth query int false "Maximum depth to traverse (default: unlimited)" default(0)
// @Success 200 {object} DependencyGraphResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /graph/containers/{id}/graph [get]
func (s *Server) getContainerGraph(c echo.Context) error {
	id := c.Param("id")

	if id == "" {
		return BadRequestError("Container ID is required", "The 'id' parameter cannot be empty")
	}

	// Get depth parameter (0 = unlimited)
	depth := 0
	if depthStr := c.QueryParam("depth"); depthStr != "" {
		if _, err := fmt.Sscanf(depthStr, "%d", &depth); err != nil {
			return BadRequestError("Invalid depth parameter", "depth must be a positive integer")
		}
		if depth < 0 {
			return BadRequestError("Invalid depth parameter", "depth must be non-negative")
		}
	}

	// Get the container
	container, err := s.storage.GetContainer(id)
	if err != nil {
		return NotFoundError("Container", id)
	}

	// Build the dependency graph
	graph := s.buildDependencyGraph(container, depth)

	return c.JSON(http.StatusOK, graph)
}

// buildDependencyGraph recursively builds the dependency graph for a container.
func (s *Server) buildDependencyGraph(container *models.Container, maxDepth int) *DependencyGraphResponse {
	visited := make(map[string]bool)
	graph := &DependencyGraphResponse{
		Container:    container,
		Dependencies: make([]*DependencyNode, 0),
		Dependents:   make([]*DependencyNode, 0),
	}

	// Build dependencies tree
	if len(container.DependsOn) > 0 {
		graph.Dependencies = s.buildDependencyTree(container.DependsOn, maxDepth, 1, visited, "dependencies")
	}

	// Build dependents tree
	visited = make(map[string]bool) // Reset visited for dependents traversal
	dependents, _ := s.storage.GetContainerDependents(container.ID)
	if len(dependents) > 0 {
		depIDs := make([]string, len(dependents))
		for i, dep := range dependents {
			depIDs[i] = dep.ID
		}
		graph.Dependents = s.buildDependencyTree(depIDs, maxDepth, 1, visited, "dependents")
	}

	return graph
}

// buildDependencyTree recursively builds a tree of dependencies or dependents.
func (s *Server) buildDependencyTree(containerIDs []string, maxDepth, currentDepth int, visited map[string]bool, direction string) []*DependencyNode {
	// Check depth limit
	if maxDepth > 0 && currentDepth > maxDepth {
		return nil
	}

	nodes := make([]*DependencyNode, 0, len(containerIDs))

	for _, id := range containerIDs {
		// Avoid cycles
		if visited[id] {
			continue
		}
		visited[id] = true

		container, err := s.storage.GetContainer(id)
		if err != nil {
			// Skip missing containers
			continue
		}

		node := &DependencyNode{
			Container: container,
			Depth:     currentDepth,
		}

		// Recursively build children
		if direction == "dependencies" && len(container.DependsOn) > 0 {
			node.Children = s.buildDependencyTree(container.DependsOn, maxDepth, currentDepth+1, visited, direction)
		} else if direction == "dependents" {
			dependents, _ := s.storage.GetContainerDependents(id)
			if len(dependents) > 0 {
				depIDs := make([]string, len(dependents))
				for i, dep := range dependents {
					depIDs[i] = dep.ID
				}
				node.Children = s.buildDependencyTree(depIDs, maxDepth, currentDepth+1, visited, direction)
			}
		}

		nodes = append(nodes, node)
	}

	return nodes
}

// GetGraphDataStackView returns the stack-centric graph visualization
// @Summary Get stack-centric graph data
// @Description Get graph with stacks as primary nodes, hosts nested within
// @Tags Graph
// @Accept json
// @Produce json
// @Param view query string false "View mode" Enums(stack, host, hybrid, stack-only) default(stack)
// @Param orphans query boolean false "Include orphaned containers" default(false)
// @Success 200 {object} GraphData
// @Failure 500 {object} ErrorResponse
// @Router /graph/stack-view [get]
func (s *Server) GetGraphDataStackView(c echo.Context) error {
	viewMode := c.QueryParam("view")
	if viewMode == "" {
		viewMode = "stack" // Default to stack view
	}

	includeOrphans := c.QueryParam("orphans") == "true"

	switch viewMode {
	case "stack":
		return s.getStackCentricGraph(c, includeOrphans)
	case "host":
		return s.GetGraphData(c) // Use existing host-centric view
	case "hybrid":
		return s.getStackCentricGraph(c, true) // Always include orphans in hybrid
	case "stack-only":
		return s.getStackOnlyGraph(c)
	default:
		return BadRequestError("Invalid view mode", "Must be one of: stack, host, hybrid, stack-only")
	}
}

// getStackCentricGraph builds stack-centric graph data with compound nodes
func (s *Server) getStackCentricGraph(c echo.Context, includeOrphans bool) error {
	graphData := GraphData{
		Nodes: make([]GraphNode, 0),
		Edges: make([]GraphEdge, 0),
	}

	// Get all stacks
	stacks, err := s.storage.ListStacks(nil)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to list stacks",
			Details: err.Error(),
		})
	}

	// Build a set of all container IDs that belong to stacks
	stackContainerIDs := make(map[string]bool)

	// For each stack, get deployment state and build compound nodes
	for _, stack := range stacks {
		deployments, err := s.storage.GetDeploymentsByStackID(stack.ID)
		if err != nil || len(deployments) == 0 {
			continue // Skip stacks without deployments
		}

		deployment := deployments[len(deployments)-1] // Get latest deployment

		// Add stack node (compound/parent node)
		stackNode := GraphNode{
			Data: GraphNodeData{
				ID:             stack.ID,
				Label:          stack.Name,
				Type:           "stack",
				Status:         stack.Status,
				ContainerCount: len(deployment.Placements),
				HostCount:      storage.CountUniqueHosts(deployment.Placements),
				Metadata: map[string]string{
					"description":      stack.Description,
					"deploymentMode":   stack.Deployment.Mode,
					"placementStrategy": stack.Deployment.PlacementStrategy,
				},
			},
		}
		graphData.Nodes = append(graphData.Nodes, stackNode)

		// Group placements by host
		hostContainers := storage.GroupPlacementsByHost(deployment.Placements)

		// Add host nodes (as children of stack)
		for hostID, containers := range hostContainers {
			host, err := s.storage.GetHost(hostID)
			if err != nil {
				continue // Skip missing hosts
			}

			hostNode := GraphNode{
				Data: GraphNodeData{
					ID:       hostID,
					Label:    host.Name,
					Type:     "host",
					Status:   host.Status,
					IP:       host.IPAddress,
					CPU:      host.CPU,
					Memory:   host.Memory,
					Location: host.Datacenter,
					Parent:   stack.ID, // Nest within stack (compound node)
				},
			}
			graphData.Nodes = append(graphData.Nodes, hostNode)

			// Add container nodes (as children of host)
			for containerName, placement := range containers {
				container, err := s.storage.GetContainer(placement.ContainerID)
				if err != nil {
					continue // Skip missing containers
				}

				// Track that this container belongs to a stack
				stackContainerIDs[container.ID] = true

				containerNode := GraphNode{
					Data: GraphNodeData{
						ID:     container.ID,
						Label:  container.Name,
						Type:   "container",
						Status: container.Status,
						Image:  container.Image,
						Parent: hostID, // Nest within host
						Metadata: map[string]string{
							"stack":         stack.ID,
							"containerName": containerName,
						},
					},
				}
				graphData.Nodes = append(graphData.Nodes, containerNode)

				// Add dependency edges (container to container)
				for _, depID := range container.DependsOn {
					edge := GraphEdge{
						Data: GraphEdgeData{
							ID:     container.ID + "-depends-" + depID,
							Source: container.ID,
							Target: depID,
							Label:  "depends on",
							Type:   "depends_on",
						},
					}
					graphData.Edges = append(graphData.Edges, edge)
				}
			}
		}
	}

	// Optionally include orphaned containers (not in any stack)
	if includeOrphans {
		orphans, err := s.storage.FindOrphanedContainers()
		if err == nil && len(orphans) > 0 {
			// Group orphans by host
			orphansByHost := make(map[string][]*models.Container)
			for _, container := range orphans {
				if container.HostedOn != "" {
					orphansByHost[container.HostedOn] = append(orphansByHost[container.HostedOn], container)
				}
			}

			// Add orphan container nodes (no parent = top-level)
			for hostID, hostOrphans := range orphansByHost {
				// Check if host node already exists
				hostExists := false
				for _, node := range graphData.Nodes {
					if node.Data.ID == hostID && node.Data.Type == "host" {
						hostExists = true
						break
					}
				}

				// Add host node if not already present
				if !hostExists {
					host, err := s.storage.GetHost(hostID)
					if err == nil {
						hostNode := GraphNode{
							Data: GraphNodeData{
								ID:       hostID,
								Label:    host.Name,
								Type:     "host",
								Status:   host.Status,
								IP:       host.IPAddress,
								Location: host.Datacenter,
								// No parent = top-level node
							},
						}
						graphData.Nodes = append(graphData.Nodes, hostNode)
					}
				}

				// Add orphan containers
				for _, container := range hostOrphans {
					containerNode := GraphNode{
						Data: GraphNodeData{
							ID:     container.ID,
							Label:  container.Name,
							Type:   "container",
							Status: container.Status,
							Image:  container.Image,
							Parent: hostID, // Nest within host
							Metadata: map[string]string{
								"orphan": "true",
							},
						},
					}
					graphData.Nodes = append(graphData.Nodes, containerNode)

					// Add dependency edges for orphans too
					for _, depID := range container.DependsOn {
						edge := GraphEdge{
							Data: GraphEdgeData{
								ID:     container.ID + "-depends-" + depID,
								Source: container.ID,
								Target: depID,
								Label:  "depends on",
								Type:   "depends_on",
							},
						}
						graphData.Edges = append(graphData.Edges, edge)
					}
				}
			}
		}
	}

	return c.JSON(http.StatusOK, graphData)
}

// getStackOnlyGraph returns a high-level view with only stack nodes
func (s *Server) getStackOnlyGraph(c echo.Context) error {
	graphData := GraphData{
		Nodes: make([]GraphNode, 0),
		Edges: make([]GraphEdge, 0),
	}

	// Get all stacks
	stacks, err := s.storage.ListStacks(nil)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to list stacks",
			Details: err.Error(),
		})
	}

	// Add only stack nodes (no children)
	for _, stack := range stacks {
		deployments, err := s.storage.GetDeploymentsByStackID(stack.ID)

		containerCount := 0
		hostCount := 0

		if err == nil && len(deployments) > 0 {
			deployment := deployments[len(deployments)-1]
			containerCount = len(deployment.Placements)
			hostCount = storage.CountUniqueHosts(deployment.Placements)
		}

		stackNode := GraphNode{
			Data: GraphNodeData{
				ID:             stack.ID,
				Label:          stack.Name,
				Type:           "stack",
				Status:         stack.Status,
				ContainerCount: containerCount,
				HostCount:      hostCount,
				Metadata: map[string]string{
					"description":      stack.Description,
					"deploymentMode":   stack.Deployment.Mode,
					"placementStrategy": stack.Deployment.PlacementStrategy,
				},
			},
		}
		graphData.Nodes = append(graphData.Nodes, stackNode)
	}

	// TODO: Add stack-to-stack dependency edges if needed in the future

	return c.JSON(http.StatusOK, graphData)
}
