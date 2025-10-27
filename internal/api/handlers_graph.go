package api

import (
	"net/http"

	"evalgo.org/graphium/models"
	"github.com/labstack/echo/v4"
)

// GraphNode represents a node in the visualization graph
type GraphNode struct {
	Data GraphNodeData `json:"data"`
}

// GraphNodeData contains the node's properties
type GraphNodeData struct {
	ID       string            `json:"id"`
	Label    string            `json:"label"`
	Type     string            `json:"type"` // "host", "container"
	Status   string            `json:"status,omitempty"`
	Image    string            `json:"image,omitempty"`
	IP       string            `json:"ip,omitempty"`
	CPU      int               `json:"cpu,omitempty"`
	Memory   int64             `json:"memory,omitempty"`
	Location string            `json:"location,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
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
// @Summary Get graph visualization data
// @Description Returns nodes and edges for graph visualization
// @Tags graph
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

		// Add edge from container to host
		if container.HostedOn != "" {
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
	}

	return c.JSON(http.StatusOK, graphData)
}

// GetGraphStats returns statistics about the graph
// @Summary Get graph statistics
// @Description Returns statistics about nodes and edges
// @Tags graph
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
	for _, container := range containers {
		if container.HostedOn != "" {
			relationships++
		}
	}

	stats := map[string]interface{}{
		"nodes": map[string]interface{}{
			"total":      len(hosts) + len(containers),
			"hosts":      len(hosts),
			"containers": len(containers),
		},
		"edges": map[string]interface{}{
			"total":      relationships,
			"hosted_on":  relationships,
			"depends_on": 0, // TODO: Add dependency tracking
		},
		"containersByStatus": containersByStatus,
		"hostsByStatus":      hostsByStatus,
	}

	return c.JSON(http.StatusOK, stats)
}

// GetGraphLayout returns the graph with a specific layout applied
// @Summary Get graph with layout
// @Description Returns graph data with layout coordinates
// @Tags graph
// @Param layout query string false "Layout algorithm" Enums(force, hierarchical, circle, grid)
// @Produce json
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

		if container.HostedOn != "" {
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

// Helper function to convert Host to GraphNode
func hostToNode(host *models.Host) GraphNode {
	return GraphNode{
		Data: GraphNodeData{
			ID:       host.ID,
			Label:    host.Name,
			Type:     "host",
			Status:   host.Status,
			IP:       host.IPAddress,
			CPU:      host.CPU,
			Memory:   host.Memory,
			Location: host.Datacenter,
		},
	}
}

// Helper function to convert Container to GraphNode
func containerToNode(container *models.Container) GraphNode {
	return GraphNode{
		Data: GraphNodeData{
			ID:     container.ID,
			Label:  container.Name,
			Type:   "container",
			Status: container.Status,
			Image:  container.Image,
		},
	}
}
