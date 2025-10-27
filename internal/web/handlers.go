package web

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"evalgo.org/graphium/internal/auth"
	"evalgo.org/graphium/internal/config"
	"evalgo.org/graphium/internal/storage"
	"evalgo.org/graphium/models"
)

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

	// Get containers
	containers, err := h.storage.ListContainers(filters)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to load containers")
	}

	return Render(c, ContainersListWithUser(containers, user))
}

// ContainersTable renders just the containers table (for HTMX).
func (h *Handler) ContainersTable(c echo.Context) error {
	// Get filters from query params
	filters := make(map[string]interface{})
	if status := c.QueryParam("status"); status != "" {
		filters["status"] = status
	}
	if host := c.QueryParam("host"); host != "" {
		filters["hostedOn"] = host
	}

	// Get containers
	containers, err := h.storage.ListContainers(filters)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to load containers")
	}

	return Render(c, ContainersTable(containers))
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

	// Get hosts
	hosts, err := h.storage.ListHosts(filters)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to load hosts")
	}

	return Render(c, HostsListWithUser(hosts, user))
}

// HostsTable renders just the hosts table (for HTMX).
func (h *Handler) HostsTable(c echo.Context) error {
	// Get filters from query params
	filters := make(map[string]interface{})
	if status := c.QueryParam("status"); status != "" {
		filters["status"] = status
	}
	if datacenter := c.QueryParam("datacenter"); datacenter != "" {
		filters["location"] = datacenter
	}

	// Get hosts
	hosts, err := h.storage.ListHosts(filters)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to load hosts")
	}

	return Render(c, HostsTable(hosts))
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
