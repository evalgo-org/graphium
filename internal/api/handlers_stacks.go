package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/docker/docker/client"
	"github.com/labstack/echo/v4"
	"evalgo.org/graphium/internal/orchestration"
	"evalgo.org/graphium/models"
	"eve.evalgo.org/containers/stacks"
)

// DeployStackRequest represents a stack deployment request.
type DeployStackRequest struct {
	Name               string                     `json:"name" validate:"required"`
	Description        string                     `json:"description"`
	DefinitionPath     string                     `json:"definitionPath" validate:"required"`
	Datacenter         string                     `json:"datacenter"`
	PlacementStrategy  string                     `json:"placementStrategy" validate:"required,oneof=auto manual spread datacenter"`
	HostConstraints    []models.HostConstraint    `json:"hostConstraints"`
	NetworkMode        string                     `json:"networkMode" validate:"oneof=host-port overlay"`
	TargetHosts        []string                   `json:"targetHosts"`
}

// StackResponse represents a stack in API responses.
type StackResponse struct {
	ID             string                  `json:"id"`
	Name           string                  `json:"name"`
	Description    string                  `json:"description"`
	Status         string                  `json:"status"`
	Datacenter     string                  `json:"datacenter"`
	Deployment     models.DeploymentConfig `json:"deployment"`
	Containers     []string                `json:"containers"`
	DefinitionPath string                  `json:"definitionPath"`
	DeploymentID   string                  `json:"deploymentId"`
	CreatedAt      time.Time               `json:"createdAt"`
	UpdatedAt      time.Time               `json:"updatedAt"`
	DeployedAt     *time.Time              `json:"deployedAt"`
	Owner          string                  `json:"owner"`
	ErrorMessage   string                  `json:"errorMessage,omitempty"`
}

// DeploymentResponse represents a deployment in API responses.
type DeploymentResponse struct {
	StackID       string                              `json:"stackId"`
	Placements    map[string]models.ContainerPlacement `json:"placements"`
	NetworkConfig models.NetworkConfig                 `json:"networkConfig"`
	StartedAt     time.Time                            `json:"startedAt"`
	CompletedAt   *time.Time                           `json:"completedAt"`
	Status        string                               `json:"status"`
	ErrorMessage  string                               `json:"errorMessage,omitempty"`
}

// deployStack deploys a new stack across multiple hosts.
// @Summary Deploy a new stack
// @Description Deploy a multi-container stack across multiple Docker hosts
// @Tags stacks
// @Accept json
// @Produce json
// @Param stack body DeployStackRequest true "Stack deployment configuration"
// @Success 201 {object} DeploymentResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/stacks [post]
func (s *Server) deployStack(c echo.Context) error {
	var req DeployStackRequest
	if err := c.Bind(&req); err != nil {
		return BadRequestError("Invalid request body", err.Error())
	}

	if err := c.Validate(&req); err != nil {
		return BadRequestError("Validation failed", err.Error())
	}

	ctx := c.Request().Context()

	// Load stack definition from file
	definition, err := stacks.LoadStackFromFile(req.DefinitionPath)
	if err != nil {
		return BadRequestError("Failed to load stack definition", err.Error())
	}

	// Create stack model
	stack := &models.Stack{
		Context:     "https://schema.org",
		Type:        "ItemList",
		ID:          fmt.Sprintf("stack-%s-%d", req.Name, time.Now().Unix()),
		Name:        req.Name,
		Description: req.Description,
		Status:      "pending",
		Datacenter:  req.Datacenter,
		Deployment: models.DeploymentConfig{
			Mode:              "multi-host",
			PlacementStrategy: req.PlacementStrategy,
			HostConstraints:   req.HostConstraints,
			NetworkMode:       req.NetworkMode,
		},
		DefinitionPath: req.DefinitionPath,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		Owner:          getUserFromContext(c),
	}

	// Save stack to database
	if err := s.storage.SaveStack(stack); err != nil {
		return InternalError("Failed to save stack", err.Error())
	}

	// Get target hosts
	hosts, err := s.getTargetHosts(ctx, req.TargetHosts, req.Datacenter)
	if err != nil {
		stack.Status = "error"
		stack.ErrorMessage = fmt.Sprintf("Failed to get target hosts: %v", err)
		s.storage.UpdateStack(stack)
		return BadRequestError("Failed to get target hosts", err.Error())
	}

	// Create orchestrator
	orch := orchestration.NewDistributedStackOrchestrator(s.storage)
	defer orch.Close()

	// Register hosts with orchestrator
	for _, hostInfo := range hosts {
		if err := orch.RegisterHost(hostInfo.Host, hostInfo.DockerSocket); err != nil {
			stack.Status = "error"
			stack.ErrorMessage = fmt.Sprintf("Failed to register host %s: %v", hostInfo.Host.ID, err)
			s.storage.UpdateStack(stack)
			return InternalError(fmt.Sprintf("Failed to register host %s", hostInfo.Host.ID), err.Error())
		}
	}

	// Deploy stack
	deployment, err := orch.DeployStack(ctx, stack, definition, hosts)
	if err != nil {
		return InternalError("Failed to deploy stack", err.Error())
	}

	// Convert to response
	response := &DeploymentResponse{
		StackID:       deployment.StackID,
		Placements:    deployment.Placements,
		NetworkConfig: deployment.NetworkConfig,
		StartedAt:     deployment.StartedAt,
		CompletedAt:   deployment.CompletedAt,
		Status:        deployment.Status,
		ErrorMessage:  deployment.ErrorMessage,
	}

	return c.JSON(http.StatusCreated, response)
}

// listStacks lists all stacks with optional filters.
// @Summary List stacks
// @Description List all stacks with optional status and datacenter filters
// @Tags stacks
// @Accept json
// @Produce json
// @Param status query string false "Filter by status"
// @Param datacenter query string false "Filter by datacenter"
// @Success 200 {array} StackResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/stacks [get]
func (s *Server) listStacks(c echo.Context) error {
	filters := make(map[string]interface{})

	if status := c.QueryParam("status"); status != "" {
		filters["status"] = status
	}

	if datacenter := c.QueryParam("datacenter"); datacenter != "" {
		filters["location"] = datacenter
	}

	stacks, err := s.storage.ListStacks(filters)
	if err != nil {
		return InternalError("Failed to list stacks", err.Error())
	}

	// Convert to response
	responses := make([]StackResponse, len(stacks))
	for i, stack := range stacks {
		responses[i] = StackResponse{
			ID:             stack.ID,
			Name:           stack.Name,
			Description:    stack.Description,
			Status:         stack.Status,
			Datacenter:     stack.Datacenter,
			Deployment:     stack.Deployment,
			Containers:     stack.Containers,
			DefinitionPath: stack.DefinitionPath,
			DeploymentID:   stack.DeploymentID,
			CreatedAt:      stack.CreatedAt,
			UpdatedAt:      stack.UpdatedAt,
			DeployedAt:     stack.DeployedAt,
			Owner:          stack.Owner,
			ErrorMessage:   stack.ErrorMessage,
		}
	}

	return c.JSON(http.StatusOK, responses)
}

// getStack retrieves a specific stack by ID.
// @Summary Get stack by ID
// @Description Get detailed information about a specific stack
// @Tags stacks
// @Accept json
// @Produce json
// @Param id path string true "Stack ID"
// @Success 200 {object} StackResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/stacks/{id} [get]
func (s *Server) getStack(c echo.Context) error {
	id := c.Param("id")

	stack, err := s.storage.GetStack(id)
	if err != nil {
		return NotFoundError("Stack", id)
	}

	response := StackResponse{
		ID:             stack.ID,
		Name:           stack.Name,
		Description:    stack.Description,
		Status:         stack.Status,
		Datacenter:     stack.Datacenter,
		Deployment:     stack.Deployment,
		Containers:     stack.Containers,
		DefinitionPath: stack.DefinitionPath,
		DeploymentID:   stack.DeploymentID,
		CreatedAt:      stack.CreatedAt,
		UpdatedAt:      stack.UpdatedAt,
		DeployedAt:     stack.DeployedAt,
		Owner:          stack.Owner,
		ErrorMessage:   stack.ErrorMessage,
	}

	return c.JSON(http.StatusOK, response)
}

// getStackDeployment retrieves deployment information for a stack.
// @Summary Get stack deployment
// @Description Get detailed deployment information including container placements
// @Tags stacks
// @Accept json
// @Produce json
// @Param id path string true "Stack ID"
// @Success 200 {object} DeploymentResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/stacks/{id}/deployment [get]
func (s *Server) getStackDeployment(c echo.Context) error {
	id := c.Param("id")

	deployment, err := s.storage.GetDeployment(id)
	if err != nil {
		return NotFoundError("Deployment", id)
	}

	response := &DeploymentResponse{
		StackID:       deployment.StackID,
		Placements:    deployment.Placements,
		NetworkConfig: deployment.NetworkConfig,
		StartedAt:     deployment.StartedAt,
		CompletedAt:   deployment.CompletedAt,
		Status:        deployment.Status,
		ErrorMessage:  deployment.ErrorMessage,
	}

	return c.JSON(http.StatusOK, response)
}

// stopStack stops a running stack.
// @Summary Stop stack
// @Description Stop all containers in a stack
// @Tags stacks
// @Accept json
// @Produce json
// @Param id path string true "Stack ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/stacks/{id}/stop [post]
func (s *Server) stopStack(c echo.Context) error {
	id := c.Param("id")
	ctx := c.Request().Context()

	// Create orchestrator
	orch := orchestration.NewDistributedStackOrchestrator(s.storage)
	defer orch.Close()

	// Get deployment to find hosts
	deployment, err := s.storage.GetDeployment(id)
	if err != nil {
		return NotFoundError("Deployment", id)
	}

	// Register hosts
	for _, placement := range deployment.Placements {
		host, err := s.storage.GetHost(placement.HostID)
		if err != nil {
			continue
		}
		dockerSocket := fmt.Sprintf("tcp://%s:2375", host.IPAddress)
		orch.RegisterHost(host, dockerSocket)
	}

	// Stop stack
	if err := orch.StopStack(ctx, id); err != nil {
		return InternalError("Failed to stop stack", err.Error())
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "Stack stopped successfully",
		"stackId": id,
	})
}

// removeStack removes a stack and its containers.
// @Summary Remove stack
// @Description Remove a stack and all its containers
// @Tags stacks
// @Accept json
// @Produce json
// @Param id path string true "Stack ID"
// @Param removeVolumes query bool false "Remove volumes" default(false)
// @Success 200 {object} map[string]string
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/stacks/{id} [delete]
func (s *Server) removeStack(c echo.Context) error {
	id := c.Param("id")
	ctx := c.Request().Context()

	removeVolumes := c.QueryParam("removeVolumes") == "true"

	// Create orchestrator
	orch := orchestration.NewDistributedStackOrchestrator(s.storage)
	defer orch.Close()

	// Get deployment to find hosts
	deployment, err := s.storage.GetDeployment(id)
	if err != nil {
		return NotFoundError("Deployment", id)
	}

	// Register hosts
	for _, placement := range deployment.Placements {
		host, err := s.storage.GetHost(placement.HostID)
		if err != nil {
			continue
		}
		dockerSocket := fmt.Sprintf("tcp://%s:2375", host.IPAddress)
		orch.RegisterHost(host, dockerSocket)
	}

	// Remove stack
	if err := orch.RemoveStack(ctx, id, removeVolumes); err != nil {
		return InternalError("Failed to remove stack", err.Error())
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "Stack removed successfully",
		"stackId": id,
	})
}

// Helper functions

// getTargetHosts retrieves and prepares host information for deployment.
func (s *Server) getTargetHosts(ctx context.Context, targetHostIDs []string, datacenter string) ([]*models.HostInfo, error) {
	var hosts []*models.Host

	// If specific hosts requested, get those
	if len(targetHostIDs) > 0 {
		for _, hostID := range targetHostIDs {
			host, err := s.storage.GetHost(hostID)
			if err != nil {
				return nil, fmt.Errorf("host %s not found: %w", hostID, err)
			}
			hosts = append(hosts, host)
		}
	} else if datacenter != "" {
		// Get all hosts in datacenter
		dcHosts, err := s.storage.ListHosts(map[string]interface{}{
			"location": datacenter,
			"status":   "active",
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list hosts in datacenter %s: %w", datacenter, err)
		}
		hosts = dcHosts
	} else {
		// Get all active hosts
		allHosts, err := s.storage.ListHosts(map[string]interface{}{
			"status": "active",
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list active hosts: %w", err)
		}
		hosts = allHosts
	}

	if len(hosts) == 0 {
		return nil, fmt.Errorf("no suitable hosts found")
	}

	// Convert to HostInfo with resource information
	hostInfos := make([]*models.HostInfo, len(hosts))
	for i, host := range hosts {
		// Create Docker client to get resource info
		dockerSocket := fmt.Sprintf("tcp://%s:2375", host.IPAddress)
		if host.IPAddress == "localhost" || host.IPAddress == "127.0.0.1" {
			dockerSocket = "unix:///var/run/docker.sock"
		}

		cli, err := client.NewClientWithOpts(
			client.WithHost(dockerSocket),
			client.WithAPIVersionNegotiation(),
		)
		if err != nil {
			// If we can't connect, use placeholder values
			hostInfos[i] = &models.HostInfo{
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
			}
			continue
		}

		// Get actual resource info
		hostInfo, err := orchestration.GetHostResourceInfo(ctx, cli, host)
		if err != nil {
			cli.Close()
			// Use placeholder values
			hostInfos[i] = &models.HostInfo{
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
			}
			continue
		}

		cli.Close()
		hostInfos[i] = hostInfo
	}

	return hostInfos, nil
}

// getUserFromContext extracts the username from the request context.
func getUserFromContext(c echo.Context) string {
	// Try to get user from JWT claims
	user := c.Get("user")
	if user != nil {
		if claims, ok := user.(map[string]interface{}); ok {
			if username, ok := claims["username"].(string); ok {
				return username
			}
		}
	}
	return "unknown"
}

// Helper function to marshal JSON for error responses
func marshalJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
