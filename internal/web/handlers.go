package web

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"evalgo.org/graphium/internal/auth"
	"evalgo.org/graphium/internal/config"
	"evalgo.org/graphium/internal/storage"
	"evalgo.org/graphium/models"
)

// PaginationInfo holds pagination metadata
type PaginationInfo struct {
	Page       int
	PageSize   int
	TotalItems int
	TotalPages int
	HasPrev    bool
	HasNext    bool
}

// Handler handles web UI requests.
type Handler struct {
	storage *storage.Storage
	config  *config.Config
}

// NewHandler creates a new web handler.
func NewHandler(store *storage.Storage, cfg *config.Config) *Handler {
	return &Handler{
		storage: store,
		config:  cfg,
	}
}

// Dashboard renders the main dashboard.
func (h *Handler) Dashboard(c echo.Context) error {
	// Get current user from context (if authenticated)
	var user *models.User
	if claims, ok := c.Get("claims").(*auth.Claims); ok {
		user, _ = h.storage.GetUser(claims.UserID)
	}

	// Get statistics
	stats, err := h.storage.GetStatistics()
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to load statistics")
	}

	return Render(c, DashboardWithUser(stats, user))
}

// calculatePagination creates pagination info from total items, current page, and page size
func calculatePagination(totalItems, page, pageSize int) PaginationInfo {
	if page < 1 {
		page = 1
	}
	totalPages := (totalItems + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}

	return PaginationInfo{
		Page:       page,
		PageSize:   pageSize,
		TotalItems: totalItems,
		TotalPages: totalPages,
		HasPrev:    page > 1,
		HasNext:    page < totalPages,
	}
}

// paginateContainers returns a slice of containers for the current page
func paginateContainers(containers []*models.Container, page, pageSize int) []*models.Container {
	start := (page - 1) * pageSize
	if start >= len(containers) {
		return []*models.Container{}
	}
	end := start + pageSize
	if end > len(containers) {
		end = len(containers)
	}
	return containers[start:end]
}

// paginateHosts returns a slice of hosts for the current page
func paginateHosts(hosts []*models.Host, page, pageSize int) []*models.Host {
	start := (page - 1) * pageSize
	if start >= len(hosts) {
		return []*models.Host{}
	}
	end := start + pageSize
	if end > len(hosts) {
		end = len(hosts)
	}
	return hosts[start:end]
}

// ContainersList renders the containers list page.
func (h *Handler) ContainersList(c echo.Context) error {
	// Get current user from context (if authenticated)
	var user *models.User
	if claims, ok := c.Get("claims").(*auth.Claims); ok {
		user, _ = h.storage.GetUser(claims.UserID)
	}

	// Get filters from query params
	filters := make(map[string]interface{})
	if status := c.QueryParam("status"); status != "" {
		filters["status"] = status
	}
	if host := c.QueryParam("host"); host != "" {
		filters["hostedOn"] = host
	}

	// Get page number from query params (default to 1)
	page := 1
	if pageStr := c.QueryParam("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	// Get containers
	allContainers, err := h.storage.ListContainers(filters)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to load containers")
	}

	// Calculate pagination
	pageSize := 10
	pagination := calculatePagination(len(allContainers), page, pageSize)
	containers := paginateContainers(allContainers, page, pageSize)

	return Render(c, ContainersListWithUser(containers, pagination, user))
}

// ContainersTable renders just the containers table (for HTMX).
func (h *Handler) ContainersTable(c echo.Context) error {
	// Get filters from query params
	filters := make(map[string]interface{})
	queryParts := []string{}

	if status := c.QueryParam("status"); status != "" {
		filters["status"] = status
		queryParts = append(queryParts, "status="+status)
	}
	if host := c.QueryParam("host"); host != "" {
		filters["hostedOn"] = host
		queryParts = append(queryParts, "host="+host)
	}

	// Get search query parameter
	search := c.QueryParam("search")
	if search != "" {
		queryParts = append(queryParts, "search="+search)
	}

	queryString := ""
	if len(queryParts) > 0 {
		queryString = strings.Join(queryParts, "&")
	}

	// Get page number from query params (default to 1)
	page := 1
	if pageStr := c.QueryParam("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	// Get containers
	allContainers, err := h.storage.ListContainers(filters)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to load containers")
	}

	// Apply search filter if present (client-side filtering by name)
	if search != "" {
		filteredContainers := make([]*models.Container, 0)
		searchLower := strings.ToLower(search)
		for _, container := range allContainers {
			if strings.Contains(strings.ToLower(container.Name), searchLower) ||
				strings.Contains(strings.ToLower(container.ID), searchLower) {
				filteredContainers = append(filteredContainers, container)
			}
		}
		allContainers = filteredContainers
	}

	// Calculate pagination
	pageSize := 10
	pagination := calculatePagination(len(allContainers), page, pageSize)
	containers := paginateContainers(allContainers, page, pageSize)

	return Render(c, ContainersTableWithPagination(containers, pagination, queryString))
}

// HostsList renders the hosts list page.
func (h *Handler) HostsList(c echo.Context) error {
	// Get current user from context (if authenticated)
	var user *models.User
	if claims, ok := c.Get("claims").(*auth.Claims); ok {
		user, _ = h.storage.GetUser(claims.UserID)
	}

	// Get filters from query params
	filters := make(map[string]interface{})
	if status := c.QueryParam("status"); status != "" {
		filters["status"] = status
	}
	if datacenter := c.QueryParam("datacenter"); datacenter != "" {
		filters["location"] = datacenter
	}

	// Get page number from query params (default to 1)
	page := 1
	if pageStr := c.QueryParam("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	// Get hosts
	allHosts, err := h.storage.ListHosts(filters)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to load hosts")
	}

	// Calculate pagination
	pageSize := 10
	pagination := calculatePagination(len(allHosts), page, pageSize)
	hosts := paginateHosts(allHosts, page, pageSize)

	return Render(c, HostsListWithUser(hosts, pagination, user))
}

// HostsTable renders just the hosts table (for HTMX).
func (h *Handler) HostsTable(c echo.Context) error {
	// Get filters from query params
	filters := make(map[string]interface{})
	queryParts := []string{}

	if status := c.QueryParam("status"); status != "" {
		filters["status"] = status
		queryParts = append(queryParts, "status="+status)
	}
	if datacenter := c.QueryParam("datacenter"); datacenter != "" {
		filters["location"] = datacenter
		queryParts = append(queryParts, "datacenter="+datacenter)
	}

	// Get search query parameter
	search := c.QueryParam("search")
	if search != "" {
		queryParts = append(queryParts, "search="+search)
	}

	queryString := ""
	if len(queryParts) > 0 {
		queryString = strings.Join(queryParts, "&")
	}

	// Get page number from query params (default to 1)
	page := 1
	if pageStr := c.QueryParam("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	// Get hosts
	allHosts, err := h.storage.ListHosts(filters)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to load hosts")
	}

	// Apply search filter if present (client-side filtering by name)
	if search != "" {
		filteredHosts := make([]*models.Host, 0)
		searchLower := strings.ToLower(search)
		for _, host := range allHosts {
			if strings.Contains(strings.ToLower(host.Name), searchLower) ||
				strings.Contains(strings.ToLower(host.ID), searchLower) {
				filteredHosts = append(filteredHosts, host)
			}
		}
		allHosts = filteredHosts
	}

	// Calculate pagination
	pageSize := 10
	pagination := calculatePagination(len(allHosts), page, pageSize)
	hosts := paginateHosts(allHosts, page, pageSize)

	return Render(c, HostsTableWithPagination(hosts, pagination, queryString))
}

// TopologyView renders the topology visualization.
func (h *Handler) TopologyView(c echo.Context) error {
	// Get current user from context (if authenticated)
	var user *models.User
	if claims, ok := c.Get("claims").(*auth.Claims); ok {
		user, _ = h.storage.GetUser(claims.UserID)
	}

	datacenter := c.QueryParam("datacenter")
	if datacenter == "" {
		datacenter = "all"
	}

	// Get all hosts and containers
	hosts, err := h.storage.ListHosts(nil)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to load hosts")
	}

	// Get all datacenters
	datacenters := make(map[string]bool)
	for _, host := range hosts {
		if host.Datacenter != "" {
			datacenters[host.Datacenter] = true
		}
	}

	// Get topology data based on datacenter selection
	var topologies map[string]*storage.DatacenterTopology
	var singleTopology *storage.DatacenterTopology

	if datacenter == "all" {
		// Get topology for all datacenters
		topologies = make(map[string]*storage.DatacenterTopology)
		for dc := range datacenters {
			topo, err := h.storage.GetDatacenterTopology(dc)
			if err == nil && topo != nil {
				topologies[dc] = topo
			}
		}
	} else {
		// Get topology for specific datacenter
		topo, err := h.storage.GetDatacenterTopology(datacenter)
		if err != nil {
			return c.String(http.StatusInternalServerError, "Failed to load topology")
		}
		singleTopology = topo
	}

	return Render(c, TopologyViewWithUser(singleTopology, topologies, datacenters, datacenter, user))
}

// GraphView renders the interactive graph visualization.
func (h *Handler) GraphView(c echo.Context) error {
	// Get current user from context (if authenticated)
	var user *models.User
	if claims, ok := c.Get("claims").(*auth.Claims); ok {
		user, _ = h.storage.GetUser(claims.UserID)
	}

	return Render(c, GraphViewWithUser(h.config, user))
}

// ContainerDetail renders the container detail page.
func (h *Handler) ContainerDetail(c echo.Context) error {
	// Get current user from context (if authenticated)
	var user *models.User
	if claims, ok := c.Get("claims").(*auth.Claims); ok {
		user, _ = h.storage.GetUser(claims.UserID)
	}

	id := c.Param("id")
	if id == "" {
		return c.String(http.StatusBadRequest, "Container ID is required")
	}

	// Get container
	container, err := h.storage.GetContainer(id)
	if err != nil {
		return c.String(http.StatusNotFound, "Container not found")
	}

	// Get host if hostedOn is set
	var host *models.Host
	if container.HostedOn != "" {
		host, _ = h.storage.GetHost(container.HostedOn)
		// Ignore error - host might not exist (orphaned reference)
	}

	return Render(c, ContainerDetailWithUser(container, host, user))
}

// HostDetail renders the host detail page.
func (h *Handler) HostDetail(c echo.Context) error {
	// Get current user from context (if authenticated)
	var user *models.User
	if claims, ok := c.Get("claims").(*auth.Claims); ok {
		user, _ = h.storage.GetUser(claims.UserID)
	}

	id := c.Param("id")
	if id == "" {
		return c.String(http.StatusBadRequest, "Host ID is required")
	}

	// Get host
	host, err := h.storage.GetHost(id)
	if err != nil {
		return c.String(http.StatusNotFound, "Host not found")
	}

	// Get containers on this host
	containers, err := h.storage.GetContainersByHost(id)
	if err != nil {
		// If error, just use empty list
		containers = []*models.Container{}
	}

	return Render(c, HostDetailWithUser(host, containers, user))
}

// ContainerLogs renders the container logs page.
func (h *Handler) ContainerLogs(c echo.Context) error {
	// Get current user from context (if authenticated)
	var user *models.User
	if claims, ok := c.Get("claims").(*auth.Claims); ok {
		user, _ = h.storage.GetUser(claims.UserID)
	}

	id := c.Param("id")
	if id == "" {
		return c.String(http.StatusBadRequest, "Container ID is required")
	}

	// Get container
	container, err := h.storage.GetContainer(id)
	if err != nil {
		return c.String(http.StatusNotFound, "Container not found")
	}

	// Get host if hostedOn is set
	var host *models.Host
	if container.HostedOn != "" {
		host, _ = h.storage.GetHost(container.HostedOn)
	}

	return Render(c, ContainerLogsView(container, host, user))
}
