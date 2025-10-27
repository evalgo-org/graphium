package web

import (
	"net/http"

	"evalgo.org/graphium/internal/config"
	"evalgo.org/graphium/internal/storage"
	"github.com/labstack/echo/v4"
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
	// Get statistics
	stats, err := h.storage.GetStatistics()
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to load statistics")
	}

	return Render(c, Dashboard(stats))
}

// ContainersList renders the containers list page.
func (h *Handler) ContainersList(c echo.Context) error {
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

	return Render(c, ContainersList(containers))
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

	return Render(c, HostsList(hosts))
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
	datacenter := c.QueryParam("datacenter")
	if datacenter == "" {
		datacenter = "all"
	}

	// Get topology data
	var topology *storage.DatacenterTopology
	var err error

	if datacenter != "all" {
		topology, err = h.storage.GetDatacenterTopology(datacenter)
		if err != nil {
			return c.String(http.StatusInternalServerError, "Failed to load topology")
		}
	}

	return Render(c, TopologyView(topology, datacenter))
}

// GraphView renders the interactive graph visualization.
func (h *Handler) GraphView(c echo.Context) error {
	return Render(c, GraphView(h.config))
}
