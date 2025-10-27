package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// getStatistics handles GET /api/v1/stats
func (s *Server) getStatistics(c echo.Context) error {
	stats, err := s.storage.GetStatistics()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to get statistics",
			Details: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"totalContainers":       stats.TotalContainers,
		"runningContainers":     stats.RunningContainers,
		"totalHosts":            stats.TotalHosts,
		"hostsWithContainers":   len(stats.HostContainerCounts),
		"containerDistribution": stats.HostContainerCounts,
	})
}

// getContainerCount handles GET /api/v1/stats/containers/count
func (s *Server) getContainerCount(c echo.Context) error {
	// Parse filters from query params
	filters := make(map[string]interface{})

	if status := c.QueryParam("status"); status != "" {
		filters["status"] = status
	}
	if host := c.QueryParam("host"); host != "" {
		filters["hostedOn"] = host
	}

	count, err := s.storage.CountContainers(filters)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to count containers",
			Details: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"count":   count,
		"filters": filters,
	})
}

// getHostCount handles GET /api/v1/stats/hosts/count
func (s *Server) getHostCount(c echo.Context) error {
	// Parse filters from query params
	filters := make(map[string]interface{})

	if status := c.QueryParam("status"); status != "" {
		filters["status"] = status
	}
	if datacenter := c.QueryParam("datacenter"); datacenter != "" {
		filters["location"] = datacenter
	}

	count, err := s.storage.CountHosts(filters)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to count hosts",
			Details: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"count":   count,
		"filters": filters,
	})
}

// getHostContainerDistribution handles GET /api/v1/stats/distribution
func (s *Server) getHostContainerDistribution(c echo.Context) error {
	distribution, err := s.storage.GetHostContainerCount()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to get container distribution",
			Details: err.Error(),
		})
	}

	// Calculate statistics
	totalHosts := len(distribution)
	totalContainers := 0
	maxContainersPerHost := 0
	minContainersPerHost := int(^uint(0) >> 1) // max int

	for _, count := range distribution {
		totalContainers += count
		if count > maxContainersPerHost {
			maxContainersPerHost = count
		}
		if count < minContainersPerHost {
			minContainersPerHost = count
		}
	}

	avgContainersPerHost := float64(0)
	if totalHosts > 0 {
		avgContainersPerHost = float64(totalContainers) / float64(totalHosts)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"distribution":         distribution,
		"totalHosts":           totalHosts,
		"totalContainers":      totalContainers,
		"maxContainersPerHost": maxContainersPerHost,
		"minContainersPerHost": minContainersPerHost,
		"avgContainersPerHost": avgContainersPerHost,
	})
}

// getDatabaseInfo handles GET /api/v1/info
func (s *Server) getDatabaseInfo(c echo.Context) error {
	info, err := s.storage.GetDatabaseInfo()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to get database info",
			Details: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, info)
}
