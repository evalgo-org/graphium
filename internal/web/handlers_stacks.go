package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"evalgo.org/graphium/internal/auth"
	"evalgo.org/graphium/internal/orchestration"
	"evalgo.org/graphium/models"
	"eve.evalgo.org/containers/stacks"
)

// paginateStacks returns a slice of stacks for the current page
func paginateStacks(stacks []*models.Stack, page, pageSize int) []*models.Stack {
	start := (page - 1) * pageSize
	if start >= len(stacks) {
		return []*models.Stack{}
	}
	end := start + pageSize
	if end > len(stacks) {
		end = len(stacks)
	}
	return stacks[start:end]
}

// StacksList renders the stacks list page.
func (h *Handler) StacksList(c echo.Context) error {
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
		filters["datacenter"] = datacenter
	}

	// Get page number from query params (default to 1)
	page := 1
	if pageStr := c.QueryParam("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	// Get stacks
	allStacks, err := h.storage.ListStacks(filters)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to load stacks")
	}

	// Calculate pagination
	pageSize := 10
	pagination := calculatePagination(len(allStacks), page, pageSize)
	stacks := paginateStacks(allStacks, page, pageSize)

	return Render(c, StacksListWithUser(stacks, pagination, user))
}

// StacksTable renders just the stacks table (for HTMX).
func (h *Handler) StacksTable(c echo.Context) error {
	// Get filters from query params
	filters := make(map[string]interface{})
	queryParts := []string{}

	if status := c.QueryParam("status"); status != "" {
		filters["status"] = status
		queryParts = append(queryParts, "status="+status)
	}
	if datacenter := c.QueryParam("datacenter"); datacenter != "" {
		filters["datacenter"] = datacenter
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

	// Get stacks
	allStacks, err := h.storage.ListStacks(filters)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to load stacks")
	}

	// Apply search filter if present (client-side filtering by name)
	if search != "" {
		filteredStacks := make([]*models.Stack, 0)
		searchLower := strings.ToLower(search)
		for _, stack := range allStacks {
			if strings.Contains(strings.ToLower(stack.Name), searchLower) ||
				strings.Contains(strings.ToLower(stack.ID), searchLower) {
				filteredStacks = append(filteredStacks, stack)
			}
		}
		allStacks = filteredStacks
	}

	// Calculate pagination
	pageSize := 10
	pagination := calculatePagination(len(allStacks), page, pageSize)
	stacks := paginateStacks(allStacks, page, pageSize)

	return Render(c, StacksTableWithPagination(stacks, pagination, queryString))
}

// StackDetail renders the stack detail page.
func (h *Handler) StackDetail(c echo.Context) error {
	// Get current user from context (if authenticated)
	var user *models.User
	if claims, ok := c.Get("claims").(*auth.Claims); ok {
		user, _ = h.storage.GetUser(claims.UserID)
	}

	id := c.Param("id")
	if id == "" {
		return c.String(http.StatusBadRequest, "Stack ID is required")
	}

	// Get stack
	stack, err := h.storage.GetStack(id)
	if err != nil {
		return c.String(http.StatusNotFound, "Stack not found")
	}

	// Get deployment info if stack is deployed
	var deployment *models.StackDeployment
	if stack.Status == "running" || stack.Status == "stopped" {
		deployment, _ = h.storage.GetDeployment(id)
		// Ignore error - deployment might not exist
	}

	// Load container details for all containers assigned to this stack
	var containers []*models.Container
	if len(stack.Containers) > 0 {
		for _, containerID := range stack.Containers {
			container, err := h.storage.GetContainer(containerID)
			if err == nil {
				containers = append(containers, container)
			}
		}
	}

	return Render(c, StackDetailWithUser(stack, deployment, containers, user))
}

// DeployStackForm renders the stack deployment form.
func (h *Handler) DeployStackForm(c echo.Context) error {
	// Get current user from context (if authenticated)
	var user *models.User
	if claims, ok := c.Get("claims").(*auth.Claims); ok {
		user, _ = h.storage.GetUser(claims.UserID)
	}

	// Get list of available templates
	templates, err := h.listStackTemplates()
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to load templates")
	}

	// Get list of available active hosts
	hosts, err := h.storage.ListHosts(map[string]interface{}{
		"status": "active",
	})
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to load hosts")
	}

	// Get list of available containers
	containers, err := h.storage.ListContainers(nil)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to load containers")
	}

	// Extract unique datacenters
	datacenters := make(map[string]bool)
	for _, host := range hosts {
		if host.Datacenter != "" {
			datacenters[host.Datacenter] = true
		}
	}

	return Render(c, DeployStackFormWithUser(templates, hosts, containers, datacenters, "", user))
}

// DeployStack handles the stack deployment form submission.
func (h *Handler) DeployStack(c echo.Context) error {
	ctx := c.Request().Context()

	// Get current user from context
	var user *models.User
	var username string = "web-user"
	if claims, ok := c.Get("claims").(*auth.Claims); ok {
		user, _ = h.storage.GetUser(claims.UserID)
		if user != nil {
			username = user.Username
		}
	}

	// Parse form data
	stackJSON := c.FormValue("stack_json")
	selectedHostIDs := c.Request().Form["target_hosts[]"]
	selectedContainerIDs := c.Request().Form["existing_containers[]"]
	datacenter := c.FormValue("datacenter")

	if stackJSON == "" {
		templates, _ := h.listStackTemplates()
		hosts, _ := h.storage.ListHosts(map[string]interface{}{"status": "active"})
		containers, _ := h.storage.ListContainers(nil)
		datacenters := make(map[string]bool)
		for _, host := range hosts {
			if host.Datacenter != "" {
				datacenters[host.Datacenter] = true
			}
		}
		return Render(c, DeployStackFormWithUser(templates, hosts, containers, datacenters, "Stack definition is required", user))
	}

	// Helper function to render error with form data
	renderError := func(errorMsg string) error {
		templates, _ := h.listStackTemplates()
		hosts, _ := h.storage.ListHosts(map[string]interface{}{"status": "active"})
		containers, _ := h.storage.ListContainers(nil)
		datacenters := make(map[string]bool)
		for _, host := range hosts {
			if host.Datacenter != "" {
				datacenters[host.Datacenter] = true
			}
		}
		return Render(c, DeployStackFormWithUser(templates, hosts, containers, datacenters, errorMsg, user))
	}

	// Parse the JSON-LD stack definition to validate it
	var definition map[string]interface{}
	if err := json.Unmarshal([]byte(stackJSON), &definition); err != nil {
		return renderError("Invalid JSON: " + err.Error())
	}

	// Validate required fields
	stackName, ok := definition["name"].(string)
	if !ok || stackName == "" {
		return renderError("Stack 'name' field is required in JSON definition")
	}

	// Load stack definition using stacks package
	stackDef, err := stacks.LoadStackFromJSON([]byte(stackJSON))
	if err != nil {
		return renderError("Invalid stack definition: " + err.Error())
	}

	stackDescription := ""
	if desc, ok := definition["description"].(string); ok {
		stackDescription = desc
	}

	// Create stack model
	stack := &models.Stack{
		Context:     "https://schema.org",
		Type:        "ItemList",
		ID:          fmt.Sprintf("stack-%s-%d", stackName, time.Now().Unix()),
		Name:        stackName,
		Description: stackDescription,
		Status:      "pending",
		Deployment: models.DeploymentConfig{
			Mode:              "multi-host",
			PlacementStrategy: "auto",
			NetworkMode:       "host-port",
		},
		Containers: selectedContainerIDs, // Add existing containers to the stack
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		Owner:      username,
	}

	// Save stack to database
	if err := h.storage.SaveStack(stack); err != nil {
		return renderError("Failed to save stack: " + err.Error())
	}

	// Get target hosts based on selection
	var targetHosts []*models.Host

	if len(selectedHostIDs) > 0 {
		// Use specifically selected hosts
		for _, hostID := range selectedHostIDs {
			host, err := h.storage.GetHost(hostID)
			if err == nil && host.Status == "active" {
				targetHosts = append(targetHosts, host)
			}
		}
	} else if datacenter != "" {
		// Use hosts from selected datacenter
		hosts, err := h.storage.ListHosts(map[string]interface{}{
			"location": datacenter,
			"status":   "active",
		})
		if err == nil {
			targetHosts = hosts
		}
	} else {
		// Use all active hosts
		hosts, err := h.storage.ListHosts(map[string]interface{}{
			"status": "active",
		})
		if err == nil {
			targetHosts = hosts
		}
	}

	if len(targetHosts) == 0 {
		stack.Status = "error"
		stack.ErrorMessage = "No active hosts available for deployment"
		h.storage.UpdateStack(stack)
		return renderError("No active hosts available for deployment. Please ensure hosts are registered and active.")
	}

	// Convert to HostInfo format with smart Docker socket detection
	hostInfos := make([]*models.HostInfo, 0, len(targetHosts))
	for _, host := range targetHosts {
		var dockerSocket string

		// Determine Docker socket based on host configuration
		if host.IPAddress == "localhost" || host.IPAddress == "127.0.0.1" || host.IPAddress == "" {
			// Local host - use Unix socket
			dockerSocket = "unix:///var/run/docker.sock"
		} else if strings.HasPrefix(host.IPAddress, "192.168.") || strings.HasPrefix(host.IPAddress, "10.") || strings.HasPrefix(host.IPAddress, "172.") {
			// Private network - use TCP
			dockerSocket = "tcp://" + host.IPAddress + ":2375"
		} else {
			// Public IP or hostname - use TCP
			dockerSocket = "tcp://" + host.IPAddress + ":2375"
		}

		hostInfos = append(hostInfos, &models.HostInfo{
			Host:         host,
			DockerSocket: dockerSocket,
			CurrentLoad: models.ResourceLoad{
				CPUUsage:       0,
				MemoryUsage:    0,
				ContainerCount: 0,
			},
			AvailableResources: models.Resources{
				CPU:    host.CPU,
				Memory: host.Memory,
			},
			Labels: make(map[string]string),
		})
	}

	// Create orchestrator
	orch := orchestration.NewDistributedStackOrchestrator(h.storage)
	defer orch.Close()

	// Register hosts with orchestrator and track failures
	registeredCount := 0
	var registrationErrors []string

	for _, hostInfo := range hostInfos {
		if err := orch.RegisterHost(hostInfo.Host, hostInfo.DockerSocket); err != nil {
			registrationErrors = append(registrationErrors,
				fmt.Sprintf("Host %s (%s): %v", hostInfo.Host.Name, hostInfo.Host.ID, err))
			continue
		}
		registeredCount++
	}

	// Check if we have any hosts successfully registered
	if registeredCount == 0 {
		stack.Status = "error"
		errorMsg := "Failed to register any Docker clients. No hosts are accessible.\n\n"
		errorMsg += "Possible solutions:\n"
		errorMsg += "1. Ensure Docker is running on the target hosts\n"
		errorMsg += "2. For localhost: Verify Docker socket is accessible (usually /var/run/docker.sock)\n"
		errorMsg += "3. For remote hosts: Ensure Docker API is exposed on port 2375\n"
		errorMsg += "4. Run the Graphium agent on each host to auto-register\n\n"
		errorMsg += "Registration errors:\n"
		for _, regErr := range registrationErrors {
			errorMsg += "  - " + regErr + "\n"
		}
		stack.ErrorMessage = errorMsg
		h.storage.UpdateStack(stack)
		return renderError(errorMsg)
	}

	// Deploy stack
	deployment, err := orch.DeployStack(ctx, stack, stackDef, hostInfos)
	if err != nil {
		stack.Status = "error"
		stack.ErrorMessage = err.Error()
		h.storage.UpdateStack(stack)

		// Enhance error message with registration info
		enhancedErr := fmt.Sprintf("Deployment failed: %v\n\n", err)
		if registeredCount < len(hostInfos) {
			enhancedErr += fmt.Sprintf("Note: Only %d of %d hosts were successfully registered.\n",
				registeredCount, len(hostInfos))
			enhancedErr += "Failed hosts:\n"
			for _, regErr := range registrationErrors {
				enhancedErr += "  - " + regErr + "\n"
			}
		}
		return renderError(enhancedErr)
	}

	// Update stack status
	stack.Status = "running"
	stack.DeployedAt = &deployment.StartedAt
	h.storage.UpdateStack(stack)

	// Redirect to stack detail on success
	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/web/stacks/%s", stack.ID))
}

// StopStack handles stopping a stack.
func (h *Handler) StopStack(c echo.Context) error {
	ctx := c.Request().Context()
	id := c.Param("id")
	if id == "" {
		return c.String(http.StatusBadRequest, "Stack ID is required")
	}

	// Get stack
	stack, err := h.storage.GetStack(id)
	if err != nil {
		return c.String(http.StatusNotFound, "Stack not found")
	}

	// Get deployment
	deployment, err := h.storage.GetDeployment(id)
	if err != nil {
		return c.String(http.StatusNotFound, "Deployment not found")
	}

	// Create orchestrator
	orch := orchestration.NewDistributedStackOrchestrator(h.storage)
	defer orch.Close()

	// Register hosts from deployment
	for _, placement := range deployment.Placements {
		host, err := h.storage.GetHost(placement.HostID)
		if err != nil {
			continue
		}
		dockerSocket := "tcp://" + host.IPAddress + ":2375"
		orch.RegisterHost(host, dockerSocket)
	}

	// Stop stack
	if err := orch.StopStack(ctx, id); err != nil {
		return c.String(http.StatusInternalServerError, "Failed to stop stack: "+err.Error())
	}

	// Update stack status
	stack.Status = "stopped"
	h.storage.UpdateStack(stack)

	// Redirect back to stack detail
	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/web/stacks/%s", id))
}

// DeleteStack handles deleting a stack.
func (h *Handler) DeleteStack(c echo.Context) error {
	ctx := c.Request().Context()
	id := c.Param("id")
	if id == "" {
		return c.String(http.StatusBadRequest, "Stack ID is required")
	}

	// Get deployment if it exists
	deployment, err := h.storage.GetDeployment(id)
	if err == nil {
		// Create orchestrator
		orch := orchestration.NewDistributedStackOrchestrator(h.storage)
		defer orch.Close()

		// Register hosts from deployment using the exact HostID from placement
		registeredHosts := make(map[string]bool)
		for _, placement := range deployment.Placements {
			// Skip if already registered
			if registeredHosts[placement.HostID] {
				continue
			}

			host, err := h.storage.GetHost(placement.HostID)
			if err != nil {
				continue
			}

			// Determine Docker socket based on host type
			var dockerSocket string
			if host.IPAddress == "localhost" || host.IPAddress == "127.0.0.1" ||
			   host.IPAddress == "host-"+host.Name || host.Name == "fedora-local" {
				// Local host - use Unix socket
				dockerSocket = "unix:///var/run/docker.sock"
			} else {
				// Remote host - use TCP
				dockerSocket = "tcp://" + host.IPAddress + ":2375"
			}

			// Register using placement.HostID to ensure consistency
			if err := orch.RegisterHostWithID(placement.HostID, dockerSocket); err != nil {
				continue
			}
			registeredHosts[placement.HostID] = true
		}

		// Remove stack containers
		if err := orch.RemoveStack(ctx, id, false); err != nil {
			return c.String(http.StatusInternalServerError, "Failed to remove stack: "+err.Error())
		}

		// Delete deployment
		h.storage.DeleteDeployment(id)
	}

	// Delete stack
	if err := h.storage.DeleteStack(id); err != nil {
		return c.String(http.StatusInternalServerError, "Failed to delete stack: "+err.Error())
	}

	// Redirect to stacks list
	return c.Redirect(http.StatusSeeOther, "/web/stacks")
}

// EditStackForm renders the stack edit form.
func (h *Handler) EditStackForm(c echo.Context) error {
	// Get current user from context
	var user *models.User
	if claims, ok := c.Get("claims").(*auth.Claims); ok {
		user, _ = h.storage.GetUser(claims.UserID)
	}

	id := c.Param("id")
	if id == "" {
		return c.String(http.StatusBadRequest, "Stack ID is required")
	}

	// Get stack
	stack, err := h.storage.GetStack(id)
	if err != nil {
		return c.String(http.StatusNotFound, "Stack not found")
	}

	// Get list of available containers
	containers, err := h.storage.ListContainers(nil)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to load containers")
	}

	// Convert stack back to JSON for editing
	stackJSON, err := json.MarshalIndent(stack, "", "  ")
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to serialize stack")
	}

	return Render(c, EditStackFormWithUser(stack, containers, string(stackJSON), "", user))
}

// UpdateStack handles the stack update form submission.
func (h *Handler) UpdateStack(c echo.Context) error {
	// Get current user from context
	var user *models.User
	if claims, ok := c.Get("claims").(*auth.Claims); ok {
		user, _ = h.storage.GetUser(claims.UserID)
	}

	id := c.Param("id")
	if id == "" {
		return c.String(http.StatusBadRequest, "Stack ID is required")
	}

	// Get existing stack
	stack, err := h.storage.GetStack(id)
	if err != nil {
		return c.String(http.StatusNotFound, "Stack not found")
	}

	// Parse form data
	name := c.FormValue("name")
	description := c.FormValue("description")
	selectedContainerIDs := c.Request().Form["stack_containers[]"]

	// Get containers for error rendering
	containers, _ := h.storage.ListContainers(nil)

	if name == "" {
		stackJSON, _ := json.MarshalIndent(stack, "", "  ")
		return Render(c, EditStackFormWithUser(stack, containers, string(stackJSON), "Stack name is required", user))
	}

	// Find newly assigned containers (in selectedContainerIDs but not in old stack.Containers)
	oldContainerSet := make(map[string]bool)
	for _, cID := range stack.Containers {
		oldContainerSet[cID] = true
	}

	var newlyAssignedContainers []string
	for _, cID := range selectedContainerIDs {
		if !oldContainerSet[cID] {
			newlyAssignedContainers = append(newlyAssignedContainers, cID)
		}
	}

	// Update stack fields
	stack.Name = name
	stack.Description = description
	stack.Containers = selectedContainerIDs // Update container assignments
	stack.UpdatedAt = time.Now()

	// Save updated stack
	if err := h.storage.UpdateStack(stack); err != nil {
		stackJSON, _ := json.MarshalIndent(stack, "", "  ")
		return Render(c, EditStackFormWithUser(stack, containers, string(stackJSON), "Failed to update stack: "+err.Error(), user))
	}

	// Rename newly assigned containers to follow stack naming convention
	for _, containerID := range newlyAssignedContainers {
		// Get container to find its host
		container, err := h.storage.GetContainer(containerID)
		if err != nil {
			h.debugLog("Warning: Could not get container %s: %v\n", containerID, err)
			continue
		}

		// Get host to determine Docker socket
		host, err := h.storage.GetHost(container.HostedOn)
		if err != nil {
			h.debugLog("Warning: Could not get host %s for container %s: %v\n", container.HostedOn, containerID, err)
			continue
		}

		// Determine Docker socket based on host IP
		var dockerSocket string
		if host.IPAddress == "localhost" || host.IPAddress == "127.0.0.1" || host.IPAddress == "" {
			dockerSocket = "unix:///var/run/docker.sock"
		} else {
			dockerSocket = "tcp://" + host.IPAddress + ":2375"
		}

		// Rename container to follow stack naming convention
		if err := h.storage.RenameContainerForStack(containerID, stack.Name, dockerSocket); err != nil {
			h.debugLog("Warning: Failed to rename container %s for stack %s: %v\n", containerID, stack.Name, err)
			// Don't fail the whole operation, just log the warning
		}
	}

	// Redirect to stack detail
	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/web/stacks/%s", id))
}

// StartStack handles starting a stopped stack.
func (h *Handler) StartStack(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.String(http.StatusBadRequest, "Stack ID is required")
	}

	// Get stack
	stack, err := h.storage.GetStack(id)
	if err != nil {
		return c.String(http.StatusNotFound, "Stack not found")
	}

	// Get deployment
	deployment, err := h.storage.GetDeployment(id)
	if err != nil {
		return c.String(http.StatusNotFound, "Deployment not found")
	}

	// Create orchestrator
	orch := orchestration.NewDistributedStackOrchestrator(h.storage)
	defer orch.Close()

	// Register hosts from deployment
	for _, placement := range deployment.Placements {
		host, err := h.storage.GetHost(placement.HostID)
		if err != nil {
			continue
		}
		dockerSocket := "tcp://" + host.IPAddress + ":2375"
		if host.IPAddress == "localhost" || host.IPAddress == "127.0.0.1" {
			dockerSocket = "unix:///var/run/docker.sock"
		}
		orch.RegisterHost(host, dockerSocket)
	}

	// Start all containers in the stack
	// Note: Would need to add StartContainer method to orchestrator
	// For now, we'll just update the status
	_ = deployment

	// Update stack status
	stack.Status = "running"
	h.storage.UpdateStack(stack)

	// Redirect back to stack detail
	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/web/stacks/%s", id))
}

// RestartStack handles restarting a stack.
func (h *Handler) RestartStack(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.String(http.StatusBadRequest, "Stack ID is required")
	}

	// Stop then start
	if err := h.StopStack(c); err != nil {
		return err
	}

	time.Sleep(2 * time.Second) // Brief pause between stop and start

	return h.StartStack(c)
}

// StackLogs displays aggregated logs from all containers in a stack.
func (h *Handler) StackLogs(c echo.Context) error {
	// Get current user from context
	var user *models.User
	if claims, ok := c.Get("claims").(*auth.Claims); ok {
		user, _ = h.storage.GetUser(claims.UserID)
	}

	id := c.Param("id")
	if id == "" {
		return c.String(http.StatusBadRequest, "Stack ID is required")
	}

	// Get stack
	stack, err := h.storage.GetStack(id)
	if err != nil {
		return c.String(http.StatusNotFound, "Stack not found")
	}

	// Get deployment
	deployment, err := h.storage.GetDeployment(id)
	if err != nil {
		return c.String(http.StatusNotFound, "Deployment not found")
	}

	// Collect container IDs from deployment
	containerIDs := make([]string, 0, len(deployment.Placements))
	for _, placement := range deployment.Placements {
		if placement.ContainerID != "" {
			containerIDs = append(containerIDs, placement.ContainerID)
		}
	}

	return Render(c, StackLogsWithUser(stack, deployment, containerIDs, user))
}

// listStackTemplates returns a list of available stack templates.
func (h *Handler) listStackTemplates() ([]StackTemplate, error) {
	templates := []StackTemplate{}

	// Read templates from configs/examples/stacks/
	templateFiles := []string{
		"configs/graphium-dev-stack.json",
		"configs/examples/stacks/3-tier-webapp.json",
		"configs/examples/stacks/microservices.json",
		"configs/examples/stacks/high-availability.json",
	}

	for _, path := range templateFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			continue // Skip missing files
		}

		var stack models.Stack
		if err := json.Unmarshal(data, &stack); err != nil {
			continue // Skip invalid files
		}

		templates = append(templates, StackTemplate{
			Name:        stack.Name,
			Description: stack.Description,
			Path:        path,
			Content:     string(data),
		})
	}

	return templates, nil
}

// StackTemplate represents a stack template for the UI.
type StackTemplate struct {
	Name        string
	Description string
	Path        string
	Content     string
}
