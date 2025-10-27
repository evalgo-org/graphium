package api

import (
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
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to list hosts",
			Details: err.Error(),
		})
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

	host, err := s.storage.GetHost(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "host not found",
			Details: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, host)
}

// createHost handles POST /api/v1/hosts
func (s *Server) createHost(c echo.Context) error {
	var host models.Host

	if err := c.Bind(&host); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid request body",
			Details: err.Error(),
		})
	}

	// Validate required fields
	if host.Name == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "name is required",
		})
	}
	if host.IPAddress == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "ipAddress is required",
		})
	}

	// Generate ID if not provided
	if host.ID == "" {
		host.ID = generateID("host", host.Name)
	}

	// Save host
	if err := s.storage.SaveHost(&host); err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to create host",
			Details: err.Error(),
		})
	}

	// Broadcast WebSocket event
	s.BroadcastGraphEvent(EventHostAdded, host)

	return c.JSON(http.StatusCreated, host)
}

// updateHost handles PUT /api/v1/hosts/:id
func (s *Server) updateHost(c echo.Context) error {
	id := c.Param("id")

	// Check if host exists
	existing, err := s.storage.GetHost(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "host not found",
			Details: err.Error(),
		})
	}

	var host models.Host
	if err := c.Bind(&host); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid request body",
			Details: err.Error(),
		})
	}

	// Preserve ID and revision
	host.ID = id
	host.Rev = existing.Rev

	// Update host
	if err := s.storage.SaveHost(&host); err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to update host",
			Details: err.Error(),
		})
	}

	// Broadcast WebSocket event
	s.BroadcastGraphEvent(EventHostUpdated, host)

	return c.JSON(http.StatusOK, host)
}

// deleteHost handles DELETE /api/v1/hosts/:id
func (s *Server) deleteHost(c echo.Context) error {
	id := c.Param("id")

	// Get host to retrieve revision
	host, err := s.storage.GetHost(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "host not found",
			Details: err.Error(),
		})
	}

	// Delete host
	if err := s.storage.DeleteHost(id, host.Rev); err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to delete host",
			Details: err.Error(),
		})
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
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid request body",
			Details: err.Error(),
		})
	}

	// Generate IDs for hosts without one
	for _, host := range hosts {
		if host.ID == "" {
			host.ID = generateID("host", host.Name)
		}
	}

	// Bulk save
	results, err := s.storage.BulkSaveHosts(hosts)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to bulk create hosts",
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

// getHostsByDatacenter handles GET /api/v1/query/hosts/by-datacenter/:datacenter
func (s *Server) getHostsByDatacenter(c echo.Context) error {
	datacenter := c.Param("datacenter")

	hosts, err := s.storage.GetHostsByDatacenter(datacenter)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to get hosts by datacenter",
			Details: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, HostsResponse{
		Count: len(hosts),
		Hosts: hosts,
	})
}
