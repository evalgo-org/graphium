package web

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	evecommon "eve.evalgo.org/common"
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

// ContainerWithStack holds a container and its optional stack membership
type ContainerWithStack struct {
	Container *models.Container
	StackName string // Empty if not part of a stack
	StackID   string // Empty if not part of a stack
}

// EventBroadcaster is an interface for broadcasting events
type EventBroadcaster interface {
	BroadcastGraphEvent(eventType string, data interface{})
}

// Handler handles web UI requests.
type Handler struct {
	storage     *storage.Storage
	config      *config.Config
	broadcaster EventBroadcaster
}

// debugLog logs a message only if debug mode is enabled in config
func (h *Handler) debugLog(format string, args ...interface{}) {
	if h.config.Server.Debug {
		fmt.Printf(format, args...)
	}
}

// NewHandler creates a new web handler.
func NewHandler(store *storage.Storage, cfg *config.Config, broadcaster EventBroadcaster) *Handler {
	return &Handler{
		storage:     store,
		config:      cfg,
		broadcaster: broadcaster,
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

	// Get stack map for all containers
	stackMap, _ := h.storage.GetContainerStackMap()
	// Ignore error - if we can't get stack info, just show containers without stack names

	// Calculate pagination
	pageSize := 10
	pagination := calculatePagination(len(allContainers), page, pageSize)
	containers := paginateContainers(allContainers, page, pageSize)

	// Get error message from query params (if any)
	errorMsg := c.QueryParam("error")

	return Render(c, ContainersListWithUser(containers, stackMap, pagination, errorMsg, user))
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

	// Get stack map for all containers
	stackMap, _ := h.storage.GetContainerStackMap()
	// Ignore error - if we can't get stack info, just show containers without stack names

	// Calculate pagination
	pageSize := 10
	pagination := calculatePagination(len(allContainers), page, pageSize)
	containers := paginateContainers(allContainers, page, pageSize)

	return Render(c, ContainersTableWithPagination(containers, stackMap, pagination, queryString))
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
		return c.Redirect(http.StatusSeeOther, "/web/containers?error=Container+not+found+(may+have+been+deleted)")
	}

	// Get host if hostedOn is set
	var host *models.Host
	if container.HostedOn != "" {
		host, _ = h.storage.GetHost(container.HostedOn)
		// Ignore error - host might not exist (orphaned reference)
	}

	// Check if container belongs to a stack
	stack, belongsToStack, _ := h.storage.GetContainerStack(id)
	// Ignore error - if we can't determine stack membership, treat as standalone

	return Render(c, ContainerDetailWithUser(container, host, stack, belongsToStack, user))
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

// CreateHostForm renders the host creation form.
func (h *Handler) CreateHostForm(c echo.Context) error {
	var user *models.User
	if claims, ok := c.Get("claims").(*auth.Claims); ok {
		user, _ = h.storage.GetUser(claims.UserID)
	}

	return Render(c, CreateHostFormWithUser(&models.Host{}, "", user))
}

// CreateHost handles host creation form submission.
func (h *Handler) CreateHost(c echo.Context) error {
	var user *models.User
	if claims, ok := c.Get("claims").(*auth.Claims); ok {
		user, _ = h.storage.GetUser(claims.UserID)
	}

	// Parse form
	host := &models.Host{
		Context:    "https://schema.org",
		Type:       "ComputerSystem",
		ID:         c.FormValue("id"),
		Name:       c.FormValue("name"),
		IPAddress:  c.FormValue("ipAddress"),
		Status:     c.FormValue("status"),
		Datacenter: c.FormValue("datacenter"),
	}

	// Parse CPU and Memory
	if cpuStr := c.FormValue("cpu"); cpuStr != "" {
		if cpu, err := strconv.Atoi(cpuStr); err == nil {
			host.CPU = cpu
		}
	}
	if memStr := c.FormValue("memory"); memStr != "" {
		if mem, err := strconv.ParseInt(memStr, 10, 64); err == nil {
			host.Memory = mem
		}
	}

	// Validate
	if host.ID == "" || host.Name == "" || host.IPAddress == "" {
		return Render(c, CreateHostFormWithUser(host, "Host ID, Name, and IP Address are required", user))
	}

	// Save to database
	if err := h.storage.SaveHost(host); err != nil {
		return Render(c, CreateHostFormWithUser(host, "Failed to create host: "+err.Error(), user))
	}

	return c.Redirect(http.StatusSeeOther, "/web/hosts")
}

// EditHostForm renders the host edit form.
func (h *Handler) EditHostForm(c echo.Context) error {
	var user *models.User
	if claims, ok := c.Get("claims").(*auth.Claims); ok {
		user, _ = h.storage.GetUser(claims.UserID)
	}

	id := c.Param("id")
	host, err := h.storage.GetHost(id)
	if err != nil {
		return c.String(http.StatusNotFound, "Host not found")
	}

	return Render(c, EditHostFormWithUser(host, "", user))
}

// UpdateHost handles host update form submission.
func (h *Handler) UpdateHost(c echo.Context) error {
	var user *models.User
	if claims, ok := c.Get("claims").(*auth.Claims); ok {
		user, _ = h.storage.GetUser(claims.UserID)
	}

	id := c.Param("id")
	host, err := h.storage.GetHost(id)
	if err != nil {
		return c.String(http.StatusNotFound, "Host not found")
	}

	// Update fields
	host.Name = c.FormValue("name")
	host.IPAddress = c.FormValue("ipAddress")
	host.Status = c.FormValue("status")
	host.Datacenter = c.FormValue("datacenter")

	if cpuStr := c.FormValue("cpu"); cpuStr != "" {
		if cpu, err := strconv.Atoi(cpuStr); err == nil {
			host.CPU = cpu
		}
	}
	if memStr := c.FormValue("memory"); memStr != "" {
		if mem, err := strconv.ParseInt(memStr, 10, 64); err == nil {
			host.Memory = mem
		}
	}

	// Validate
	if host.Name == "" || host.IPAddress == "" {
		return Render(c, EditHostFormWithUser(host, "Name and IP Address are required", user))
	}

	// Update (SaveHost handles both create and update)
	if err := h.storage.SaveHost(host); err != nil {
		return Render(c, EditHostFormWithUser(host, "Failed to update host: "+err.Error(), user))
	}

	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/web/hosts/%s", id))
}

// removeContainerFromDocker attempts to remove a container from Docker using EVE.
// Returns nil if successful, error if it fails.
func (h *Handler) removeContainerFromDocker(containerID string) error {
	dockerSocket := h.config.Agent.DockerSocket
	if dockerSocket == "" {
		dockerSocket = "/var/run/docker.sock"
	}

	// Use EVE's CtxCli to create Docker client
	ctx, cli, err := evecommon.CtxCli("unix://" + dockerSocket)
	if err != nil {
		return err
	}
	defer cli.Close()

	// Use EVE's ContainerStopAndRemove function
	// Stop timeout: 10 seconds, RemoveVolumes: false
	return evecommon.ContainerStopAndRemove(ctx, cli, containerID, 10, false)
}

// DeleteContainer handles container deletion (only for standalone containers).
func (h *Handler) DeleteContainer(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.String(http.StatusBadRequest, "Container ID is required")
	}

	// Get the container to retrieve its revision
	container, err := h.storage.GetContainer(id)
	if err != nil {
		return c.String(http.StatusNotFound, "Container not found")
	}

	// Remove container from any stacks it belongs to
	if err := h.storage.RemoveContainerFromStacks(id); err != nil {
		h.debugLog("Warning: Failed to remove container %s from stacks: %v\n", id[:12], err)
	}

	// Try to remove from Docker first
	dockerDeleteErr := h.removeContainerFromDocker(id)
	if dockerDeleteErr != nil {
		// Docker deletion failed - log warning
		fmt.Printf("Warning: Failed to remove container %s from Docker: %v\n", id, dockerDeleteErr)
	}

	// Delete from database
	h.debugLog("DEBUG: About to delete container %s from database (rev: %s)\n", id[:12], container.Rev)
	if err := h.storage.DeleteContainer(id, container.Rev); err != nil {
		h.debugLog("DEBUG: Database deletion FAILED for %s: %v\n", id[:12], err)
		return c.String(http.StatusInternalServerError, "Failed to delete container from database: "+err.Error())
	}
	h.debugLog("DEBUG: Successfully deleted container %s from database\n", id[:12])

	// Broadcast WebSocket event for real-time dashboard updates
	if h.broadcaster == nil {
		h.debugLog("DEBUG: WARNING - broadcaster is nil, cannot broadcast deletion event\n")
	} else {
		h.debugLog("DEBUG: Broadcasting container_removed event for %s\n", id[:12])
		h.broadcaster.BroadcastGraphEvent("container_removed", map[string]string{"id": id})
		h.debugLog("DEBUG: Broadcast call completed for %s\n", id[:12])
	}

	// Only add to ignore list if Docker deletion failed
	// If Docker deletion succeeded, allow re-sync if container is recreated
	if dockerDeleteErr != nil {
		if err := h.storage.AddToIgnoreList(id, container.HostedOn, "user-deleted via web UI (Docker removal failed)", "system"); err != nil {
			fmt.Printf("Warning: Failed to add container %s to ignore list: %v\n", id, err)
		}
	}

	return c.Redirect(http.StatusSeeOther, "/web/containers")
}

// BulkDeleteContainers handles bulk deletion of multiple containers
func (h *Handler) BulkDeleteContainers(c echo.Context) error {
	// Parse request body
	var req struct {
		ContainerIDs []string `json:"container_ids"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	if len(req.ContainerIDs) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "No container IDs provided",
		})
	}

	deletedCount := 0
	failedCount := 0
	var errors []string

	// Delete each container
	for _, id := range req.ContainerIDs {
		// Get container info
		container, err := h.storage.GetContainer(id)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: not found", id[:12]))
			failedCount++
			continue
		}

		// Remove container from any stacks it belongs to
		if err := h.storage.RemoveContainerFromStacks(id); err != nil {
			h.debugLog("Warning: Failed to remove container %s from stacks: %v\n", id[:12], err)
		}

		// Try to remove from Docker
		dockerDeleteErr := h.removeContainerFromDocker(id)
		if dockerDeleteErr != nil {
			h.debugLog("Warning: Failed to remove container %s from Docker: %v\n", id[:12], dockerDeleteErr)
		}

		// Delete from database
		if err := h.storage.DeleteContainer(id, container.Rev); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", id[:12], err))
			failedCount++
			continue
		}

		// Broadcast WebSocket event
		if h.broadcaster != nil {
			h.broadcaster.BroadcastGraphEvent("container_removed", map[string]string{"id": id})
		}

		// Only add to ignore list if Docker deletion failed
		if dockerDeleteErr != nil {
			if err := h.storage.AddToIgnoreList(id, container.HostedOn, "user-deleted via bulk delete (Docker removal failed)", "system"); err != nil {
				fmt.Printf("Warning: Failed to add container %s to ignore list: %v\n", id, err)
			}
		}

		deletedCount++
	}

	// Return result
	success := failedCount == 0
	response := map[string]interface{}{
		"success":       success,
		"deleted_count": deletedCount,
		"failed_count":  failedCount,
	}

	if len(errors) > 0 {
		response["error"] = fmt.Sprintf("Failed to delete %d container(s)", failedCount)
		response["details"] = errors
	}

	return c.JSON(http.StatusOK, response)
}

// BulkStopContainers handles bulk stopping of multiple containers
func (h *Handler) BulkStopContainers(c echo.Context) error {
	var req struct {
		ContainerIDs []string `json:"container_ids"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	return h.performDockerBulkAction(c, req.ContainerIDs, "stop", func(dockerClient interface{}, containerID string) error {
		// TODO: Use EVE Docker client to stop container
		return fmt.Errorf("stop action not yet implemented")
	})
}

// BulkStartContainers handles bulk starting of multiple containers
func (h *Handler) BulkStartContainers(c echo.Context) error {
	var req struct {
		ContainerIDs []string `json:"container_ids"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	return h.performDockerBulkAction(c, req.ContainerIDs, "start", func(dockerClient interface{}, containerID string) error {
		// TODO: Use EVE Docker client to start container
		return fmt.Errorf("start action not yet implemented")
	})
}

// BulkRestartContainers handles bulk restarting of multiple containers
func (h *Handler) BulkRestartContainers(c echo.Context) error {
	var req struct {
		ContainerIDs []string `json:"container_ids"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	return h.performDockerBulkAction(c, req.ContainerIDs, "restart", func(dockerClient interface{}, containerID string) error {
		// TODO: Use EVE Docker client to restart container
		return fmt.Errorf("restart action not yet implemented")
	})
}

// performDockerBulkAction is a helper function for Docker bulk operations
func (h *Handler) performDockerBulkAction(c echo.Context, containerIDs []string, actionName string, actionFunc func(interface{}, string) error) error {
	if len(containerIDs) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "No container IDs provided",
		})
	}

	successCount := 0
	failedCount := 0
	var errors []string

	// Perform action on each container
	for _, id := range containerIDs {
		// Get container info to find host
		_, err := h.storage.GetContainer(id)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: not found", id[:12]))
			failedCount++
			continue
		}

		// Perform Docker action
		err = actionFunc(nil, id) // Docker client would be passed here
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", id[:12], err))
			failedCount++
			continue
		}

		// Broadcast WebSocket event
		if h.broadcaster != nil {
			h.broadcaster.BroadcastGraphEvent("container_updated", map[string]string{"id": id, "action": actionName})
		}

		successCount++
	}

	// Return result
	success := failedCount == 0
	response := map[string]interface{}{
		"success":       success,
		"success_count": successCount,
		"failed_count":  failedCount,
	}

	if len(errors) > 0 {
		response["error"] = fmt.Sprintf("Failed to %s %d container(s)", actionName, failedCount)
		response["details"] = errors
	}

	return c.JSON(http.StatusOK, response)
}

// DeleteHost handles host deletion.
func (h *Handler) DeleteHost(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.String(http.StatusBadRequest, "Host ID is required")
	}

	// Get the host to retrieve its revision
	host, err := h.storage.GetHost(id)
	if err != nil {
		return c.String(http.StatusNotFound, "Host not found")
	}

	// Delete with revision
	if err := h.storage.DeleteHost(id, host.Rev); err != nil {
		return c.String(http.StatusInternalServerError, "Failed to delete host: "+err.Error())
	}

	return c.Redirect(http.StatusSeeOther, "/web/hosts")
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
		return c.Redirect(http.StatusSeeOther, "/web/containers?error=Container+not+found+(may+have+been+deleted)")
	}

	// Get host if hostedOn is set
	var host *models.Host
	if container.HostedOn != "" {
		host, _ = h.storage.GetHost(container.HostedOn)
	}

	return Render(c, ContainerLogsView(container, host, user))
}

// AssignContainersToStack assigns multiple containers to a stack.
func (h *Handler) AssignContainersToStack(c echo.Context) error {
	stackID := c.Param("id")
	if stackID == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Stack ID is required",
		})
	}

	var req struct {
		ContainerIDs []string `json:"container_ids"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	if len(req.ContainerIDs) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "No container IDs provided",
		})
	}

	// Get the stack
	stack, err := h.storage.GetStack(stackID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"success": false,
			"error":   "Stack not found",
		})
	}

	// Add containers to stack if not already present
	containerMap := make(map[string]bool)
	for _, id := range stack.Containers {
		containerMap[id] = true
	}

	addedCount := 0
	for _, containerID := range req.ContainerIDs {
		if !containerMap[containerID] {
			stack.Containers = append(stack.Containers, containerID)
			containerMap[containerID] = true
			addedCount++
		}
	}

	// Update the stack
	if addedCount > 0 {
		if err := h.storage.UpdateStack(stack); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"success": false,
				"error":   "Failed to update stack: " + err.Error(),
			})
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"count":   addedCount,
	})
}

// GetStacksJSON returns all stacks as JSON (for web UI dropdowns).
func (h *Handler) GetStacksJSON(c echo.Context) error {
	stacks, err := h.storage.ListStacks(nil)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to load stacks",
		})
	}

	// Convert to simple format
	type StackItem struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	items := make([]StackItem, len(stacks))
	for i, stack := range stacks {
		items[i] = StackItem{
			ID:   stack.ID,
			Name: stack.Name,
		}
	}

	return c.JSON(http.StatusOK, items)
}
