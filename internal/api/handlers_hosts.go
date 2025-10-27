package api

import (
	"fmt"
	"net/http"

	"evalgo.org/graphium/models"
	"github.com/labstack/echo/v4"
)

// listHosts handles GET /api/v1/hosts
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
