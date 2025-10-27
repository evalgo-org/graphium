package api

import (
	"strconv"

	"github.com/labstack/echo/v4"

	"evalgo.org/graphium/models"
)

// parsePagination parses limit and offset from query parameters.
// Default limit is 100, default offset is 0.
// Maximum limit is 1000 to prevent excessive memory usage.
func parsePagination(c echo.Context) (limit, offset int) {
	// Parse limit with default of 100
	limit = 100
	if limitParam := c.QueryParam("limit"); limitParam != "" {
		if parsed, err := strconv.Atoi(limitParam); err == nil && parsed > 0 {
			limit = parsed
			// Cap at 1000
			if limit > 1000 {
				limit = 1000
			}
		}
	}

	// Parse offset with default of 0
	offset = 0
	if offsetParam := c.QueryParam("offset"); offsetParam != "" {
		if parsed, err := strconv.Atoi(offsetParam); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	return limit, offset
}

// paginateSliceContainers applies pagination to a slice of containers.
func paginateSliceContainers(containers []*models.Container, limit, offset int) []*models.Container {
	// Handle edge cases
	if offset >= len(containers) {
		return []*models.Container{}
	}

	end := offset + limit
	if end > len(containers) {
		end = len(containers)
	}

	return containers[offset:end]
}

// paginateSliceHosts applies pagination to a slice of hosts.
func paginateSliceHosts(hosts []*models.Host, limit, offset int) []*models.Host {
	// Handle edge cases
	if offset >= len(hosts) {
		return []*models.Host{}
	}

	end := offset + limit
	if end > len(hosts) {
		end = len(hosts)
	}

	return hosts[offset:end]
}
