package api

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	_ "evalgo.org/graphium/internal/storage" // imported for Server.storage field
	"evalgo.org/graphium/models"
)

// listContainers handles GET /api/v1/containers
// @Summary List containers
// @Description Get a paginated list of containers with optional filtering by status, host, or datacenter
// @Tags Containers
// @Accept json
// @Produce json
// @Param status query string false "Filter by container status (running, stopped, paused, etc.)"
// @Param host query string false "Filter by host ID"
// @Param datacenter query string false "Filter by datacenter location"
// @Param limit query int false "Maximum number of items to return (default: 100, max: 1000)" minimum(1) maximum(1000)
// @Param offset query int false "Number of items to skip (default: 0)" minimum(0)
// @Success 200 {object} PaginatedContainersResponse "Successfully retrieved containers"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /containers [get]
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
// @Summary Get container by ID
// @Description Get detailed information about a specific container by its ID
// @Tags Containers
// @Accept json
// @Produce json
// @Param id path string true "Container ID"
// @Success 200 {object} models.Container "Successfully retrieved container"
// @Failure 400 {object} APIError "Bad request - Container ID is required"
// @Failure 404 {object} APIError "Container not found"
// @Router /containers/{id} [get]
func (s *Server) getContainer(c echo.Context) error {
	id := c.Param("id")

	if id == "" {
		return BadRequestError("Container ID is required", "The 'id' parameter cannot be empty")
	}

	container, err := s.storage.GetContainer(id)
	if err != nil {
		return NotFoundError("Container", id)
	}

	return c.JSON(http.StatusOK, container)
}

// createContainer handles POST /api/v1/containers
// @Summary Create a new container
// @Description Create a new container with the provided JSON-LD data
// @Tags Containers
// @Accept json
// @Produce json
// @Param container body models.Container true "Container object (JSON-LD format)"
// @Success 201 {object} models.Container "Successfully created container"
// @Failure 400 {object} APIError "Bad request - Invalid request body or validation errors"
// @Failure 500 {object} APIError "Internal server error"
// @Router /containers [post]
func (s *Server) createContainer(c echo.Context) error {
	var container models.Container

	if err := c.Bind(&container); err != nil {
		return BadRequestError("Invalid request body", "Failed to parse JSON: "+err.Error())
	}

	// Validate required fields
	fieldErrors := make(map[string]string)
	if container.Name == "" {
		fieldErrors["name"] = "Container name is required"
	}
	if container.Image == "" {
		fieldErrors["image"] = "Container image (executableName) is required"
	}
	if len(fieldErrors) > 0 {
		return ValidationError("Validation failed", fieldErrors)
	}

	// Generate ID if not provided
	if container.ID == "" {
		container.ID = generateID("container", container.Name)
	}

	// Save container
	if err := s.storage.SaveContainer(&container); err != nil {
		return InternalError("Failed to create container", err.Error())
	}

	// Auto-assign to stack based on naming convention
	if err := s.storage.AutoAssignContainerToStack(container.ID, container.Name); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Warning: Failed to auto-assign container %s to stack: %v\n", container.ID, err)
	}

	// Broadcast WebSocket event
	s.BroadcastGraphEvent(EventContainerAdded, container)

	return c.JSON(http.StatusCreated, container)
}

// updateContainer handles PUT /api/v1/containers/:id
// @Summary Update a container
// @Description Update an existing container with new JSON-LD data
// @Tags Containers
// @Accept json
// @Produce json
// @Param id path string true "Container ID"
// @Param container body models.Container true "Updated container object (JSON-LD format)"
// @Success 200 {object} models.Container "Successfully updated container"
// @Failure 400 {object} APIError "Bad request - Invalid request body or validation errors"
// @Failure 404 {object} APIError "Container not found"
// @Failure 500 {object} APIError "Internal server error"
// @Router /containers/{id} [put]
func (s *Server) updateContainer(c echo.Context) error {
	id := c.Param("id")

	if id == "" {
		return BadRequestError("Container ID is required", "The 'id' parameter cannot be empty")
	}

	// Check if container exists
	existing, err := s.storage.GetContainer(id)
	if err != nil {
		return NotFoundError("Container", id)
	}

	var container models.Container
	if err := c.Bind(&container); err != nil {
		return BadRequestError("Invalid request body", "Failed to parse JSON: "+err.Error())
	}

	// Validate required fields
	fieldErrors := make(map[string]string)
	if container.Name == "" {
		fieldErrors["name"] = "Container name is required"
	}
	if container.Image == "" {
		fieldErrors["image"] = "Container image (executableName) is required"
	}
	if len(fieldErrors) > 0 {
		return ValidationError("Validation failed", fieldErrors)
	}

	// Preserve ID and revision
	container.ID = id
	container.Rev = existing.Rev

	// Update container
	if err := s.storage.SaveContainer(&container); err != nil {
		return InternalError("Failed to update container", err.Error())
	}

	// Auto-assign to stack based on naming convention
	if err := s.storage.AutoAssignContainerToStack(container.ID, container.Name); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Warning: Failed to auto-assign container %s to stack: %v\n", container.ID, err)
	}

	// Broadcast WebSocket event
	s.BroadcastGraphEvent(EventContainerUpdated, container)

	return c.JSON(http.StatusOK, container)
}

// deleteContainer handles DELETE /api/v1/containers/:id
// @Summary Delete a container
// @Description Delete an existing container by its ID
// @Tags Containers
// @Accept json
// @Produce json
// @Param id path string true "Container ID"
// @Success 200 {object} MessageResponse "Successfully deleted container"
// @Failure 400 {object} APIError "Bad request - Container ID is required"
// @Failure 404 {object} APIError "Container not found"
// @Failure 500 {object} APIError "Internal server error"
// @Router /containers/{id} [delete]
func (s *Server) deleteContainer(c echo.Context) error {
	id := c.Param("id")

	if id == "" {
		return BadRequestError("Container ID is required", "The 'id' parameter cannot be empty")
	}

	// Get container to retrieve revision and host info
	container, err := s.storage.GetContainer(id)
	if err != nil {
		return NotFoundError("Container", id)
	}

	// Remove container from any stacks it belongs to
	if err := s.storage.RemoveContainerFromStacks(id); err != nil {
		// Log the error but don't fail the delete operation
		fmt.Printf("Warning: Failed to remove container %s from stacks: %v\n", id, err)
	}

	// Add container to ignore list to prevent agent from re-syncing it
	if err := s.storage.AddToIgnoreList(id, container.HostedOn, "user-deleted via API", "system"); err != nil {
		// Log the error but don't fail the delete operation
		fmt.Printf("Warning: Failed to add container %s to ignore list: %v\n", id, err)
	}

	// Delete container
	if err := s.storage.DeleteContainer(id, container.Rev); err != nil {
		return InternalError("Failed to delete container", err.Error())
	}

	// Broadcast WebSocket event
	s.BroadcastGraphEvent(EventContainerRemoved, map[string]string{"id": id})

	return c.JSON(http.StatusOK, MessageResponse{
		Message: "container deleted successfully",
		ID:      id,
	})
}

// bulkCreateContainers handles POST /api/v1/containers/bulk
// @Summary Bulk create containers
// @Description Create multiple containers in a single request
// @Tags Containers
// @Accept json
// @Produce json
// @Param containers body []models.Container true "Array of container objects (JSON-LD format)"
// @Success 200 {object} BulkResponse "Successfully created containers"
// @Failure 400 {object} APIError "Bad request - Invalid request body or validation errors"
// @Failure 500 {object} APIError "Internal server error"
// @Router /containers/bulk [post]
func (s *Server) bulkCreateContainers(c echo.Context) error {
	var containers []*models.Container

	if err := c.Bind(&containers); err != nil {
		return BadRequestError("Invalid request body", "Failed to parse JSON array: "+err.Error())
	}

	if len(containers) == 0 {
		return BadRequestError("Empty request", "At least one container must be provided")
	}

	// Validate containers and generate IDs
	fieldErrors := make(map[string]string)
	for i, container := range containers {
		if container.Name == "" {
			fieldErrors[fmt.Sprintf("containers[%d].name", i)] = "Container name is required"
		}
		if container.Image == "" {
			fieldErrors[fmt.Sprintf("containers[%d].image", i)] = "Container image is required"
		}
		if container.ID == "" {
			container.ID = generateID("container", container.Name)
		}
	}
	if len(fieldErrors) > 0 {
		return ValidationError("Validation failed for one or more containers", fieldErrors)
	}

	// Bulk save
	results, err := s.storage.BulkSaveContainers(containers)
	if err != nil {
		return InternalError("Failed to bulk create containers", err.Error())
	}

	// Auto-assign containers to stacks based on naming convention
	for i, result := range results {
		if result.OK && i < len(containers) {
			if err := s.storage.AutoAssignContainerToStack(containers[i].ID, containers[i].Name); err != nil {
				// Log error but don't fail the request
				fmt.Printf("Warning: Failed to auto-assign container %s to stack: %v\n", containers[i].ID, err)
			}
		}
	}

	// Count successes and failures and convert to API type
	successCount := 0
	failureCount := 0
	apiResults := make([]BulkResult, len(results))
	for i, result := range results {
		if result.OK {
			successCount++
		} else {
			failureCount++
		}
		// Convert db.BulkResult to api.BulkResult
		apiResults[i] = BulkResult{
			ID:      result.ID,
			Rev:     result.Rev,
			Error:   result.Error,
			Reason:  result.Reason,
			Success: result.OK,
		}
	}

	return c.JSON(http.StatusOK, BulkResponse{
		Total:   len(results),
		Success: successCount,
		Failed:  failureCount,
		Results: apiResults,
	})
}

// getContainersByHost handles GET /api/v1/query/containers/by-host/:hostId
func (s *Server) getContainersByHost(c echo.Context) error {
	hostID := c.Param("hostId")

	if hostID == "" {
		return BadRequestError("Host ID is required", "The 'hostId' parameter cannot be empty")
	}

	containers, err := s.storage.GetContainersByHost(hostID)
	if err != nil {
		return InternalError("Failed to query containers by host", err.Error())
	}

	return c.JSON(http.StatusOK, ContainersResponse{
		Count:      len(containers),
		Containers: containers,
	})
}

// getContainersByStatus handles GET /api/v1/query/containers/by-status/:status
func (s *Server) getContainersByStatus(c echo.Context) error {
	status := c.Param("status")

	if status == "" {
		return BadRequestError("Status is required", "The 'status' parameter cannot be empty")
	}

	// Validate status value
	validStatuses := map[string]bool{
		"running": true, "stopped": true, "paused": true,
		"restarting": true, "exited": true, "dead": true,
	}
	if !validStatuses[status] {
		return BadRequestError("Invalid status", fmt.Sprintf("Status must be one of: running, stopped, paused, restarting, exited, dead. Got: %s", status))
	}

	containers, err := s.storage.GetContainersByStatus(status)
	if err != nil {
		return InternalError("Failed to query containers by status", err.Error())
	}

	return c.JSON(http.StatusOK, ContainersResponse{
		Count:      len(containers),
		Containers: containers,
	})
}

// checkContainerIgnored handles HEAD /api/v1/containers/:id/ignored
// Returns 200 if container is ignored, 404 if not ignored
func (s *Server) checkContainerIgnored(c echo.Context) error {
	id := c.Param("id")

	if id == "" {
		return BadRequestError("Container ID is required", "The 'id' parameter cannot be empty")
	}

	ignored, err := s.storage.IsContainerIgnored(id)
	if err != nil {
		return InternalError("Failed to check ignore list", err.Error())
	}

	if ignored {
		return c.NoContent(http.StatusOK) // 200 means ignored
	}

	return c.NoContent(http.StatusNotFound) // 404 means not ignored
}

// removeFromIgnoreList handles DELETE /api/v1/containers/:id/ignored
// Removes a container from the ignore list
func (s *Server) removeFromIgnoreList(c echo.Context) error {
	id := c.Param("id")

	if id == "" {
		return BadRequestError("Container ID is required", "The 'id' parameter cannot be empty")
	}

	if err := s.storage.RemoveFromIgnoreList(id); err != nil {
		return InternalError("Failed to remove from ignore list", err.Error())
	}

	return c.JSON(http.StatusOK, MessageResponse{
		Message: "container removed from ignore list",
		ID:      id,
	})
}

// listIgnored handles GET /api/v1/containers/ignored
// @Summary List ignored containers
// @Description Get a list of all containers in the ignore list
// @Tags Containers
// @Accept json
// @Produce json
// @Success 200 {array} models.IgnoreListEntry "List of ignored containers"
// @Failure 500 {object} ErrorResponse
// @Router /containers/ignored [get]
func (s *Server) listIgnored(c echo.Context) error {
	entries, err := s.storage.ListIgnored()
	if err != nil {
		return InternalError("Failed to list ignored containers", err.Error())
	}

	return c.JSON(http.StatusOK, entries)
}
