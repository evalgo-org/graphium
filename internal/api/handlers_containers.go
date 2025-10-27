package api

import (
	"net/http"

	_ "evalgo.org/graphium/internal/storage" // imported for Server.storage field
	"evalgo.org/graphium/models"
	"github.com/labstack/echo/v4"
)

// listContainers handles GET /api/v1/containers
func (s *Server) listContainers(c echo.Context) error {
	// Parse query parameters
	filters := make(map[string]interface{})

	if status := c.QueryParam("status"); status != "" {
		filters["status"] = status
	}
	if hostID := c.QueryParam("host"); hostID != "" {
		filters["hostedOn"] = hostID
	}
	if datacenter := c.QueryParam("datacenter"); datacenter != "" {
		// For datacenter filtering, we need to use a more complex query
		// This is a simplified version
		filters["location"] = datacenter
	}

	// Parse pagination parameters
	limit, offset := parsePagination(c)

	containers, err := s.storage.ListContainers(filters)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to list containers",
			Details: err.Error(),
		})
	}

	// Get total count before pagination
	total := len(containers)

	// Apply pagination
	containers = paginateSliceContainers(containers, limit, offset)

	return c.JSON(http.StatusOK, PaginatedContainersResponse{
		Count:      len(containers),
		Total:      total,
		Limit:      limit,
		Offset:     offset,
		Containers: containers,
	})
}

// getContainer handles GET /api/v1/containers/:id
func (s *Server) getContainer(c echo.Context) error {
	id := c.Param("id")

	container, err := s.storage.GetContainer(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "container not found",
			Details: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, container)
}

// createContainer handles POST /api/v1/containers
func (s *Server) createContainer(c echo.Context) error {
	var container models.Container

	if err := c.Bind(&container); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid request body",
			Details: err.Error(),
		})
	}

	// Validate required fields
	if container.Name == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "name is required",
		})
	}
	if container.Image == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "image is required",
		})
	}

	// Generate ID if not provided
	if container.ID == "" {
		container.ID = generateID("container", container.Name)
	}

	// Save container
	if err := s.storage.SaveContainer(&container); err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to create container",
			Details: err.Error(),
		})
	}

	// Broadcast WebSocket event
	s.BroadcastGraphEvent(EventContainerAdded, container)

	return c.JSON(http.StatusCreated, container)
}

// updateContainer handles PUT /api/v1/containers/:id
func (s *Server) updateContainer(c echo.Context) error {
	id := c.Param("id")

	// Check if container exists
	existing, err := s.storage.GetContainer(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "container not found",
			Details: err.Error(),
		})
	}

	var container models.Container
	if err := c.Bind(&container); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid request body",
			Details: err.Error(),
		})
	}

	// Preserve ID and revision
	container.ID = id
	container.Rev = existing.Rev

	// Update container
	if err := s.storage.SaveContainer(&container); err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to update container",
			Details: err.Error(),
		})
	}

	// Broadcast WebSocket event
	s.BroadcastGraphEvent(EventContainerUpdated, container)

	return c.JSON(http.StatusOK, container)
}

// deleteContainer handles DELETE /api/v1/containers/:id
func (s *Server) deleteContainer(c echo.Context) error {
	id := c.Param("id")

	// Get container to retrieve revision
	container, err := s.storage.GetContainer(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "container not found",
			Details: err.Error(),
		})
	}

	// Delete container
	if err := s.storage.DeleteContainer(id, container.Rev); err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to delete container",
			Details: err.Error(),
		})
	}

	// Broadcast WebSocket event
	s.BroadcastGraphEvent(EventContainerRemoved, map[string]string{"id": id})

	return c.JSON(http.StatusOK, MessageResponse{
		Message: "container deleted successfully",
		ID:      id,
	})
}

// bulkCreateContainers handles POST /api/v1/containers/bulk
func (s *Server) bulkCreateContainers(c echo.Context) error {
	var containers []*models.Container

	if err := c.Bind(&containers); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid request body",
			Details: err.Error(),
		})
	}

	// Generate IDs for containers without one
	for _, container := range containers {
		if container.ID == "" {
			container.ID = generateID("container", container.Name)
		}
	}

	// Bulk save
	results, err := s.storage.BulkSaveContainers(containers)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to bulk create containers",
			Details: err.Error(),
		})
	}

	// Count successes and failures
	successCount := 0
	failureCount := 0
	for _, result := range results {
		if result.OK {
			successCount++
		} else {
			failureCount++
		}
	}

	return c.JSON(http.StatusOK, BulkResponse{
		Total:   len(results),
		Success: successCount,
		Failed:  failureCount,
		Results: results,
	})
}

// getContainersByHost handles GET /api/v1/query/containers/by-host/:hostId
func (s *Server) getContainersByHost(c echo.Context) error {
	hostID := c.Param("hostId")

	containers, err := s.storage.GetContainersByHost(hostID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to get containers by host",
			Details: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, ContainersResponse{
		Count:      len(containers),
		Containers: containers,
	})
}

// getContainersByStatus handles GET /api/v1/query/containers/by-status/:status
func (s *Server) getContainersByStatus(c echo.Context) error {
	status := c.Param("status")

	containers, err := s.storage.GetContainersByStatus(status)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to get containers by status",
			Details: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, ContainersResponse{
		Count:      len(containers),
		Containers: containers,
	})
}
