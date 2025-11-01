package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/client"
	"github.com/labstack/echo/v4"

	"eve.evalgo.org/common"

	"evalgo.org/graphium/internal/auth"
	stackpkg "evalgo.org/graphium/internal/stack"
	"evalgo.org/graphium/internal/storage"
	"evalgo.org/graphium/models"
)

// WebHostResolver implements the stack.HostResolver interface using the storage layer.
type WebHostResolver struct {
	storage *storage.Storage
}

// ResolveHost resolves an absolute @id URL to host information.
func (r *WebHostResolver) ResolveHost(id string) (*models.HostInfo, error) {
	host, err := r.storage.GetHost(id)
	if err != nil {
		return nil, fmt.Errorf("host %s not found: %w", id, err)
	}

	// Determine Docker socket
	dockerSocket := fmt.Sprintf("tcp://%s:2375", host.IPAddress)
	if host.IPAddress == "localhost" || host.IPAddress == "127.0.0.1" {
		dockerSocket = "unix:///var/run/docker.sock"
	}

	// Get container count
	containerCount := 0
	containers, err := r.storage.GetContainersByHost(host.ID)
	if err == nil {
		containerCount = len(containers)
	}

	return &models.HostInfo{
		Host:         host,
		DockerSocket: dockerSocket,
		CurrentLoad: models.ResourceLoad{
			CPUUsage:       0,
			MemoryUsage:    0,
			ContainerCount: containerCount,
		},
		AvailableResources: models.Resources{
			CPU:    host.CPU,
			Memory: host.Memory,
		},
		Labels: make(map[string]string),
	}, nil
}

// ListHosts returns all available hosts for automatic placement.
func (r *WebHostResolver) ListHosts() ([]*models.HostInfo, error) {
	hosts, err := r.storage.ListHosts(map[string]interface{}{
		"status": "active",
	})
	if err != nil {
		return nil, err
	}

	hostInfos := make([]*models.HostInfo, len(hosts))
	for i, host := range hosts {
		info, err := r.ResolveHost(host.ID)
		if err != nil {
			continue
		}
		hostInfos[i] = info
	}

	return hostInfos, nil
}

// WebDatabaseAdapter adapts the Storage layer to the stack.Database interface.
type WebDatabaseAdapter struct {
	storage *storage.Storage
}

// Create creates a new document in CouchDB.
func (a *WebDatabaseAdapter) Create(ctx context.Context, doc interface{}) error {
	return a.storage.SaveDocument(doc)
}

// Update updates an existing document in CouchDB.
func (a *WebDatabaseAdapter) Update(ctx context.Context, doc interface{}) error {
	return a.storage.SaveDocument(doc)
}

// WebDockerClientFactory creates Docker clients for different hosts.
type WebDockerClientFactory struct {
	storage *storage.Storage
}

// GetClient returns a Docker client for the given host ID.
func (f *WebDockerClientFactory) GetClient(ctx context.Context, hostID string) (common.DockerClient, error) {
	host, err := f.storage.GetHost(hostID)
	if err != nil {
		return nil, fmt.Errorf("host %s not found: %w", hostID, err)
	}

	dockerSocket := fmt.Sprintf("tcp://%s:2375", host.IPAddress)
	if host.IPAddress == "localhost" || host.IPAddress == "127.0.0.1" {
		dockerSocket = "unix:///var/run/docker.sock"
	}

	cli, err := client.NewClientWithOpts(
		client.WithHost(dockerSocket),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return cli, nil
}

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

	// Get deployment info if stack is deployed - use DeploymentState instead of StackDeployment
	var deployment *models.StackDeployment
	if stack.Status == "running" || stack.Status == "stopped" {
		// Try to get DeploymentState first
		deploymentState, err := h.storage.GetDeploymentState(id)
		if err == nil && deploymentState != nil {
			// Convert DeploymentState to StackDeployment for template compatibility
			deployment = &models.StackDeployment{
				StackID:   deploymentState.StackID,
				Status:    deploymentState.Status,
				StartedAt: deploymentState.StartedAt,
				Placements: func() map[string]models.ContainerPlacement {
					placements := make(map[string]models.ContainerPlacement)
					for name, placement := range deploymentState.Placements {
						if placement != nil {
							placements[name] = *placement
						}
					}
					return placements
				}(),
			}
		} else {
			// Fall back to old StackDeployment if exists
			deployment, _ = h.storage.GetDeployment(id)
		}
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

// DeployStack handles the stack deployment form submission using JSON-LD deployer.
func (h *Handler) DeployStack(c echo.Context) error {
	// Get current user from context
	var user *models.User
	username := "web-user"
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

	// Parse the JSON-LD stack definition
	var definition models.StackDefinition
	if err := json.Unmarshal([]byte(stackJSON), &definition); err != nil {
		return renderError("Invalid JSON: " + err.Error())
	}

	// Create parser with host resolver
	resolver := &WebHostResolver{storage: h.storage}
	parser := stackpkg.NewStackParser(resolver)

	// Parse the stack definition
	parseResult, err := parser.Parse(&definition)
	if err != nil {
		return renderError("Failed to parse stack definition: " + err.Error())
	}

	// Check for errors
	if len(parseResult.Errors) > 0 {
		errorMsg := "Stack validation failed:\n"
		for _, e := range parseResult.Errors {
			errorMsg += "- " + e + "\n"
		}
		return renderError(errorMsg)
	}

	stackName := parseResult.Plan.StackNode.Name
	stackDescription := parseResult.Plan.StackNode.Description

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

	// Update stack status to "deploying"
	stack.Status = "deploying"
	if err := h.storage.UpdateStack(stack); err != nil {
		return renderError("Failed to update stack status: " + err.Error())
	}

	// Create deployment state
	deploymentState := &models.DeploymentState{
		ID:         stack.ID,
		StackID:    stack.ID,
		Status:     "deploying",
		Phase:      "creating-tasks",
		Progress:   0,
		Placements: make(map[string]*models.ContainerPlacement),
		StartedAt:  time.Now(),
	}

	if err := h.storage.SaveDeploymentState(deploymentState); err != nil {
		stack.Status = "error"
		stack.ErrorMessage = "Failed to create deployment state: " + err.Error()
		h.storage.UpdateStack(stack)
		return renderError(stack.ErrorMessage)
	}

	// Create agent tasks for each container using the new task-based system
	tasks, err := h.CreateDeploymentTasksForStack(stack.ID, parseResult.Plan.ContainerSpecs, username)
	if err != nil {
		stack.Status = "error"
		stack.ErrorMessage = fmt.Sprintf("Failed to create deployment tasks: %v", err)
		h.storage.UpdateStack(stack)
		return renderError(stack.ErrorMessage)
	}

	// Update deployment state
	deploymentState.Phase = "waiting-for-agents"
	deploymentState.Progress = 10
	h.storage.UpdateDeploymentState(deploymentState)

	// Broadcast WebSocket event for real-time UI updates
	h.broadcaster.BroadcastGraphEvent("stack_deploying", map[string]interface{}{
		"stackId":    stack.ID,
		"totalTasks": len(tasks),
		"status":     "deploying",
	})

	// Redirect to stack detail page (which will show deployment progress)
	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/web/stacks/%s", stack.ID))
}

// StopStack handles stopping a stack using JSON-LD deployer.
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

	// Get deployment state
	deploymentState, err := h.storage.GetDeploymentState(id)
	if err != nil {
		// Try old StackDeployment format
		oldDeployment, err2 := h.storage.GetDeployment(id)
		if err2 != nil {
			return c.String(http.StatusNotFound, "Deployment not found")
		}
		// Convert to DeploymentState
		deploymentState = &models.DeploymentState{
			StackID: oldDeployment.StackID,
			Status:  oldDeployment.Status,
			Placements: func() map[string]*models.ContainerPlacement {
				m := make(map[string]*models.ContainerPlacement)
				for name, placement := range oldDeployment.Placements {
					p := placement
					m[name] = &p
				}
				return m
			}(),
			StartedAt: oldDeployment.StartedAt,
		}
	}

	// Create deployer
	resolver := &WebHostResolver{storage: h.storage}
	dbAdapter := &WebDatabaseAdapter{storage: h.storage}
	clientFactory := &WebDockerClientFactory{storage: h.storage}
	deployer := stackpkg.NewDeployer(dbAdapter, resolver, clientFactory)

	// Stop stack
	if err := deployer.Stop(ctx, deploymentState); err != nil {
		return c.String(http.StatusInternalServerError, "Failed to stop stack: "+err.Error())
	}

	// Update stack status
	stack.Status = "stopped"
	h.storage.UpdateStack(stack)

	// Redirect back to stack detail
	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/web/stacks/%s", id))
}

// DeleteStack handles deleting a stack using agent-based task system.
func (h *Handler) DeleteStack(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.String(http.StatusBadRequest, "Stack ID is required")
	}

	// Get user
	username := "web-user"
	if claims, ok := c.Get("claims").(*auth.Claims); ok {
		username = claims.Username
	}

	// Get stack
	stack, err := h.storage.GetStack(id)
	if err != nil {
		return c.String(http.StatusNotFound, "Stack not found")
	}

	// Get deployment states for this stack (there may be multiple)
	deploymentStates, err := h.storage.GetDeploymentsByStackID(id)

	// If no deployment state exists, try old StackDeployment format
	var deploymentState *models.DeploymentState
	if err != nil || len(deploymentStates) == 0 {
		oldDeployment, err := h.storage.GetDeployment(id)
		if err == nil {
			// Convert old format to DeploymentState
			deploymentState = &models.DeploymentState{
				StackID: oldDeployment.StackID,
				Status:  oldDeployment.Status,
				Placements: func() map[string]*models.ContainerPlacement {
					m := make(map[string]*models.ContainerPlacement)
					for name, placement := range oldDeployment.Placements {
						p := placement
						m[name] = &p
					}
					return m
				}(),
				StartedAt: oldDeployment.StartedAt,
			}
		}
	} else {
		// Use the most recent deployment state
		deploymentState = deploymentStates[len(deploymentStates)-1]
	}

	// If no deployment state exists, just delete the stack metadata
	if deploymentState == nil || len(deploymentState.Placements) == 0 {
		if err := h.storage.DeleteStack(id); err != nil {
			return c.String(http.StatusInternalServerError, "Failed to delete stack: "+err.Error())
		}
		return c.Redirect(http.StatusSeeOther, "/web/stacks")
	}

	// Update stack status to "deleting"
	stack.Status = "deleting"
	if err := h.storage.UpdateStack(stack); err != nil {
		return c.String(http.StatusInternalServerError, "Failed to update stack status: "+err.Error())
	}

	// Create deletion tasks for all containers
	tasks, err := h.CreateDeletionTasksForStack(id, deploymentState, username)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to create deletion tasks: %v", err))
	}

	// Broadcast WebSocket event
	h.broadcaster.BroadcastGraphEvent("stack_deleting", map[string]interface{}{
		"stackId":    id,
		"totalTasks": len(tasks),
		"status":     "deleting",
	})

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

// StartStack handles starting a stopped stack using JSON-LD deployer.
func (h *Handler) StartStack(c echo.Context) error {
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

	// Get deployment state
	deploymentState, err := h.storage.GetDeploymentState(id)
	if err != nil {
		// Try old StackDeployment format
		oldDeployment, err2 := h.storage.GetDeployment(id)
		if err2 != nil {
			return c.String(http.StatusNotFound, "Deployment not found")
		}
		// Convert to DeploymentState
		deploymentState = &models.DeploymentState{
			StackID: oldDeployment.StackID,
			Status:  oldDeployment.Status,
			Placements: func() map[string]*models.ContainerPlacement {
				m := make(map[string]*models.ContainerPlacement)
				for name, placement := range oldDeployment.Placements {
					p := placement
					m[name] = &p
				}
				return m
			}(),
			StartedAt: oldDeployment.StartedAt,
		}
	}

	// Create deployer
	resolver := &WebHostResolver{storage: h.storage}
	dbAdapter := &WebDatabaseAdapter{storage: h.storage}
	clientFactory := &WebDockerClientFactory{storage: h.storage}
	deployer := stackpkg.NewDeployer(dbAdapter, resolver, clientFactory)

	// Start stack
	if err := deployer.Start(ctx, deploymentState); err != nil {
		return c.String(http.StatusInternalServerError, "Failed to start stack: "+err.Error())
	}

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

	// Get deployment - try DeploymentState first, then fall back to StackDeployment
	var deployment *models.StackDeployment
	var containerIDs []string

	deploymentState, err := h.storage.GetDeploymentState(id)
	if err == nil && deploymentState != nil {
		// Convert DeploymentState to StackDeployment for template
		deployment = &models.StackDeployment{
			StackID:   deploymentState.StackID,
			Status:    deploymentState.Status,
			StartedAt: deploymentState.StartedAt,
		}
		// Collect container IDs from placements
		for _, placement := range deploymentState.Placements {
			if placement != nil && placement.ContainerID != "" {
				containerIDs = append(containerIDs, placement.ContainerID)
			}
		}
	} else {
		// Fall back to old StackDeployment
		deployment, err = h.storage.GetDeployment(id)
		if err != nil {
			return c.String(http.StatusNotFound, "Deployment not found")
		}
		// Collect container IDs from old format
		for _, placement := range deployment.Placements {
			if placement.ContainerID != "" {
				containerIDs = append(containerIDs, placement.ContainerID)
			}
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
