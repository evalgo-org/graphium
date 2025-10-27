package api

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

// traverseGraph handles GET /api/v1/query/traverse/:id
func (s *Server) traverseGraph(c echo.Context) error {
	id := c.Param("id")

	// Parse query parameters
	relationField := c.QueryParam("field")
	if relationField == "" {
		relationField = "dependsOn" // default
	}

	maxDepth := 5 // default
	if depthStr := c.QueryParam("depth"); depthStr != "" {
		if d, err := strconv.Atoi(depthStr); err == nil && d > 0 {
			maxDepth = d
		}
	}

	// Get relationship graph
	graph, err := s.storage.GetContainerDependencyGraph(id, maxDepth)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to traverse graph",
			Details: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"id":            id,
		"relationField": relationField,
		"maxDepth":      maxDepth,
		"graph":         graph,
	})
}

// getDependents handles GET /api/v1/query/dependents/:id
func (s *Server) getDependents(c echo.Context) error {
	id := c.Param("id")

	// Get dependents
	dependents, err := s.storage.GetContainerDependents(id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to get dependents",
			Details: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, ContainersResponse{
		Count:      len(dependents),
		Containers: dependents,
	})
}

// getDatacenterTopology handles GET /api/v1/query/topology/:datacenter
func (s *Server) getDatacenterTopology(c echo.Context) error {
	datacenter := c.Param("datacenter")

	// Get datacenter topology
	topology, err := s.storage.GetDatacenterTopology(datacenter)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to get datacenter topology",
			Details: err.Error(),
		})
	}

	// Build response
	response := map[string]interface{}{
		"datacenter": topology.Datacenter,
		"hosts":      len(topology.Hosts),
		"topology":   make(map[string]interface{}),
	}

	// Convert topology to JSON-friendly format
	totalContainers := 0
	for hostID, hostTopo := range topology.Hosts {
		containerCount := len(hostTopo.Containers)
		totalContainers += containerCount

		response["topology"].(map[string]interface{})[hostID] = map[string]interface{}{
			"host":           hostTopo.Host,
			"containerCount": containerCount,
			"containers":     hostTopo.Containers,
		}
	}

	response["totalContainers"] = totalContainers

	return c.JSON(http.StatusOK, response)
}
