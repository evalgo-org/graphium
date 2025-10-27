package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// @Summary Get overall statistics
// @Description Get overall system statistics including container and host counts
// @Tags Statistics
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Statistics with container and host information"
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/stats [get]
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

// @Summary Get container count
// @Description Get the count of containers with optional filters
// @Tags Statistics
// @Accept json
// @Produce json
// @Param status query string false "Filter by container status"
// @Param host query string false "Filter by host"
// @Success 200 {object} map[string]interface{} "Container count with applied filters"
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/stats/containers/count [get]
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

// @Summary Get host count
// @Description Get the count of hosts with optional filters
// @Tags Statistics
// @Accept json
// @Produce json
// @Param status query string false "Filter by host status"
// @Param datacenter query string false "Filter by datacenter location"
// @Success 200 {object} map[string]interface{} "Host count with applied filters"
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/stats/hosts/count [get]
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

// @Summary Get host container distribution
// @Description Get the distribution of containers across hosts with statistics
// @Tags Statistics
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Container distribution with min, max, and average containers per host"
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/stats/distribution [get]
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
