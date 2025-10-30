package api

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"evalgo.org/graphium/models"
)

// listHosts handles GET /api/v1/hosts
// @Summary List hosts
// @Description Get a paginated list of hosts with optional filtering by status and datacenter
// @Tags Hosts
// @Accept json
// @Produce json
// @Param limit query int false "Maximum number of items to return" default(10)
// @Param offset query int false "Number of items to skip" default(0)
// @Param status query string false "Filter by host status"
// @Param datacenter query string false "Filter by datacenter location"
// @Success 200 {object} PaginatedHostsResponse
// @Failure 500 {object} ErrorResponse
// @Router /hosts [get]
func (s *Server) listHosts(c echo.Context) error {
	// Parse query parameters
	filters := make(map[string]interface{})

	if status := c.QueryParam("status"); status != "" {
		filters["status"] = status
	}
	if datacenter := c.QueryParam("datacenter"); datacenter != "" {
		filters["location"] = datacenter
	}

	// Parse pagination parameters
	limit, offset := parsePagination(c)

	hosts, err := s.storage.ListHosts(filters)
	if err != nil {
		return InternalError("Failed to list hosts", err.Error())
	}

	// Get total count before pagination
	total := len(hosts)

	// Apply pagination
	hosts = paginateSliceHosts(hosts, limit, offset)

	return c.JSON(http.StatusOK, PaginatedHostsResponse{
		Count:  len(hosts),
		Total:  total,
		Limit:  limit,
		Offset: offset,
		Hosts:  hosts,
	})
}

// getHost handles GET /api/v1/hosts/:id
// @Summary Get a host by ID
// @Description Retrieve detailed information about a specific host
// @Tags Hosts
// @Accept json
// @Produce json
// @Param id path string true "Host ID"
// @Success 200 {object} models.Host
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /hosts/{id} [get]
func (s *Server) getHost(c echo.Context) error {
	id := c.Param("id")

	if id == "" {
		return BadRequestError("Host ID is required", "The 'id' parameter cannot be empty")
	}

	host, err := s.storage.GetHost(id)
	if err != nil {
		return NotFoundError("Host", id)
	}

	return c.JSON(http.StatusOK, host)
}

// createHost handles POST /api/v1/hosts
// @Summary Create a new host
// @Description Create a new host with the provided information. ID will be auto-generated if not provided.
// @Tags Hosts
// @Accept json
// @Produce json
// @Param host body models.Host true "Host object to create"
// @Success 201 {object} models.Host
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /hosts [post]
func (s *Server) createHost(c echo.Context) error {
	var host models.Host

	if err := c.Bind(&host); err != nil {
		return BadRequestError("Invalid request body", "Failed to parse JSON: "+err.Error())
	}

	// Validate required fields
	fieldErrors := make(map[string]string)
	if host.Name == "" {
		fieldErrors["name"] = "Host name is required"
	}
	if host.IPAddress == "" {
		fieldErrors["ipAddress"] = "Host IP address is required"
	}
	if len(fieldErrors) > 0 {
		return ValidationError("Validation failed", fieldErrors)
	}

	// Generate ID if not provided
	if host.ID == "" {
		host.ID = generateID("host", host.Name)
	}

	// Save host
	if err := s.storage.SaveHost(&host); err != nil {
		return InternalError("Failed to create host", err.Error())
	}

	// Broadcast WebSocket event
	s.BroadcastGraphEvent(EventHostAdded, host)

	return c.JSON(http.StatusCreated, host)
}

// updateHost handles PUT /api/v1/hosts/:id
// @Summary Update a host
// @Description Update an existing host with new information. ID and revision are preserved.
// @Tags Hosts
// @Accept json
// @Produce json
// @Param id path string true "Host ID"
// @Param host body models.Host true "Updated host object"
// @Success 200 {object} models.Host
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /hosts/{id} [put]
func (s *Server) updateHost(c echo.Context) error {
	id := c.Param("id")

	if id == "" {
		return BadRequestError("Host ID is required", "The 'id' parameter cannot be empty")
	}

	// Check if host exists
	existing, err := s.storage.GetHost(id)
	if err != nil {
		return NotFoundError("Host", id)
	}

	var host models.Host
	if err := c.Bind(&host); err != nil {
		return BadRequestError("Invalid request body", "Failed to parse JSON: "+err.Error())
	}

	// Validate required fields
	fieldErrors := make(map[string]string)
	if host.Name == "" {
		fieldErrors["name"] = "Host name is required"
	}
	if host.IPAddress == "" {
		fieldErrors["ipAddress"] = "Host IP address is required"
	}
	if len(fieldErrors) > 0 {
		return ValidationError("Validation failed", fieldErrors)
	}

	// Preserve ID and revision
	host.ID = id
	host.Rev = existing.Rev

	// Update host
	if err := s.storage.SaveHost(&host); err != nil {
		return InternalError("Failed to update host", err.Error())
	}

	// Broadcast WebSocket event
	s.BroadcastGraphEvent(EventHostUpdated, host)

	return c.JSON(http.StatusOK, host)
}

// deleteHost handles DELETE /api/v1/hosts/:id
// @Summary Delete a host
// @Description Delete a host by its ID. This operation broadcasts a WebSocket event.
// @Tags Hosts
// @Accept json
// @Produce json
// @Param id path string true "Host ID"
// @Success 200 {object} MessageResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /hosts/{id} [delete]
func (s *Server) deleteHost(c echo.Context) error {
	id := c.Param("id")

	if id == "" {
		return BadRequestError("Host ID is required", "The 'id' parameter cannot be empty")
	}

	// Get host to retrieve revision
	host, err := s.storage.GetHost(id)
	if err != nil {
		return NotFoundError("Host", id)
	}

	// Delete host
	if err := s.storage.DeleteHost(id, host.Rev); err != nil {
		return InternalError("Failed to delete host", err.Error())
	}

	// Broadcast WebSocket event
	s.BroadcastGraphEvent(EventHostRemoved, map[string]string{"id": id})

	return c.JSON(http.StatusOK, MessageResponse{
		Message: "host deleted successfully",
		ID:      id,
	})
}

// bulkCreateHosts handles POST /api/v1/hosts/bulk
// @Summary Bulk create hosts
// @Description Create multiple hosts in a single request. Returns success/failure counts and detailed results.
// @Tags Hosts
// @Accept json
// @Produce json
// @Param hosts body []models.Host true "Array of host objects to create"
// @Success 200 {object} BulkResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /hosts/bulk [post]
func (s *Server) bulkCreateHosts(c echo.Context) error {
	var hosts []*models.Host

	if err := c.Bind(&hosts); err != nil {
		return BadRequestError("Invalid request body", "Failed to parse JSON array: "+err.Error())
	}

	if len(hosts) == 0 {
		return BadRequestError("Empty request", "At least one host must be provided")
	}

	// Validate hosts and generate IDs
	fieldErrors := make(map[string]string)
	for i, host := range hosts {
		if host.Name == "" {
			fieldErrors[fmt.Sprintf("hosts[%d].name", i)] = "Host name is required"
		}
		if host.IPAddress == "" {
			fieldErrors[fmt.Sprintf("hosts[%d].ipAddress", i)] = "Host IP address is required"
		}
		if host.ID == "" {
			host.ID = generateID("host", host.Name)
		}
	}
	if len(fieldErrors) > 0 {
		return ValidationError("Validation failed for one or more hosts", fieldErrors)
	}

	// Bulk save
	results, err := s.storage.BulkSaveHosts(hosts)
	if err != nil {
		return InternalError("Failed to bulk create hosts", err.Error())
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

// getHostsByDatacenter handles GET /api/v1/query/hosts/by-datacenter/:datacenter
func (s *Server) getHostsByDatacenter(c echo.Context) error {
	datacenter := c.Param("datacenter")

	if datacenter == "" {
		return BadRequestError("Datacenter is required", "The 'datacenter' parameter cannot be empty")
	}

	hosts, err := s.storage.GetHostsByDatacenter(datacenter)
	if err != nil {
		return InternalError("Failed to query hosts by datacenter", err.Error())
	}

	return c.JSON(http.StatusOK, HostsResponse{
		Count: len(hosts),
		Hosts: hosts,
	})
}
