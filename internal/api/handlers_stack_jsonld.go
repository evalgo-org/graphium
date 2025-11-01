package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"evalgo.org/graphium/internal/stack"
	"evalgo.org/graphium/internal/storage"
	"evalgo.org/graphium/models"
	"eve.evalgo.org/common"
)

// DeployJSONLDStackRequest represents a JSON-LD @graph stack deployment request.
type DeployJSONLDStackRequest struct {
	// StackDefinition contains the complete JSON-LD @graph structure
	StackDefinition models.StackDefinition `json:"stackDefinition" validate:"required"`

	// Options for deployment
	Timeout         int  `json:"timeout"`          // Timeout in seconds (default: 300)
	RollbackOnError bool `json:"rollbackOnError"` // Auto-rollback on error (default: true)
	PullImages      bool `json:"pullImages"`       // Pull images before deployment (default: false)
}

// DeploymentStateResponse represents a deployment state in API responses.
type DeploymentStateResponse struct {
	ID            string                                `json:"id"`
	StackID       string                                `json:"stackId"`
	Status        string                                `json:"status"`
	Phase         string                                `json:"phase,omitempty"`
	Progress      int                                   `json:"progress"`
	Placements    map[string]*models.ContainerPlacement `json:"placements"`
	NetworkInfo   *models.DeployedNetworkInfo           `json:"networkInfo,omitempty"`
	VolumeInfo    map[string]*models.VolumeInfo         `json:"volumeInfo,omitempty"`
	Events        []models.DeploymentEvent              `json:"events,omitempty"`
	StartedAt     time.Time                             `json:"startedAt"`
	CompletedAt   *time.Time                            `json:"completedAt,omitempty"`
	ErrorMessage  string                                `json:"errorMessage,omitempty"`
	RollbackState *models.RollbackState                 `json:"rollbackState,omitempty"`
}

// ParseResultResponse represents the result of parsing a stack definition.
type ParseResultResponse struct {
	Valid          bool     `json:"valid"`
	Warnings       []string `json:"warnings"`
	Errors         []string `json:"errors"`
	StackName      string   `json:"stackName,omitempty"`
	ContainerCount int      `json:"containerCount"`
	HasNetwork     bool     `json:"hasNetwork"`
	WaveCount      int      `json:"waveCount"`
}

// deployJSONLDStack deploys a stack from JSON-LD @graph definition.
// @Summary Deploy JSON-LD stack
// @Description Deploy a stack using JSON-LD @graph format with full container specifications
// @Tags stacks
// @Accept json
// @Produce json
// @Param stack body DeployJSONLDStackRequest true "JSON-LD stack deployment configuration"
// @Success 202 {object} DeploymentStateResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/stacks/jsonld [post]
func (s *Server) deployJSONLDStack(c echo.Context) error {
	var req DeployJSONLDStackRequest
	if err := c.Bind(&req); err != nil {
		return BadRequestError("Invalid request body", err.Error())
	}

	ctx := c.Request().Context()

	// Create parser with host resolver
	resolver := &APIHostResolver{storage: s.storage}
	parser := stack.NewStackParser(resolver)

	// Parse the stack definition
	parseResult, err := parser.Parse(&req.StackDefinition)
	if err != nil {
		return BadRequestError("Failed to parse stack definition", err.Error())
	}

	// Check for errors
	if len(parseResult.Errors) > 0 {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":    "Stack validation failed",
			"errors":   parseResult.Errors,
			"warnings": parseResult.Warnings,
		})
	}

	// Create deployer
	dbAdapter := &CouchDBAdapter{storage: s.storage}
	clientFactory := &APIDockerClientFactory{storage: s.storage}
	deployer := stack.NewDeployer(dbAdapter, resolver, clientFactory)

	// Set deployment options
	timeout := time.Duration(req.Timeout) * time.Second
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	opts := stack.DeployOptions{
		Timeout:         timeout,
		RollbackOnError: req.RollbackOnError,
		StackName:       parseResult.Plan.StackNode.Name,
		PullImages:      req.PullImages,
	}

	// Deploy asynchronously
	deploymentState, err := deployer.Deploy(ctx, parseResult.Plan, opts)
	if err != nil {
		// Log the actual error for debugging
		c.Logger().Error("Deployment error: ", err)
		return InternalError("Deployment failed", err.Error())
	}

	// IMPORTANT: Ensure deployment state is saved for stack deletion
	// The deployer should save it, but explicitly save here to be certain
	if err := s.storage.SaveDocument(deploymentState); err != nil {
		c.Logger().Warnf("Failed to save deployment state %s: %v", deploymentState.ID, err)
		// Don't fail deployment, just log warning
	} else {
		c.Logger().Infof("Saved deployment state %s with %d placements", deploymentState.ID, len(deploymentState.Placements))
	}

	// Create or update Stack document with deployed containers
	// This allows the web UI to show stack links and enable stack management
	stackID := deploymentState.StackID
	if stackID != "" {
		// Collect all container IDs from placements
		containerIDs := make([]string, 0, len(deploymentState.Placements))
		for _, placement := range deploymentState.Placements {
			if placement != nil && placement.ContainerID != "" {
				containerIDs = append(containerIDs, placement.ContainerID)
			}
		}

		// Try to get existing stack, if not found create new one
		existingStack, err := s.storage.GetStack(stackID)
		if err != nil {
			// Stack doesn't exist, create new one
			newStack := &models.Stack{
				ID:          stackID,
				Context:     "https://schema.org",
				Type:        "ItemList",
				Name:        parseResult.Plan.StackNode.Name,
				Description: parseResult.Plan.StackNode.Description,
				Status:      "running",
				Containers:  containerIDs,
				Datacenter:  "", // Datacenter is optional, leave empty for now
				DeployedAt:  &deploymentState.StartedAt,
				CreatedAt:   deploymentState.StartedAt,
				UpdatedAt:   deploymentState.StartedAt,
			}

			if err := s.storage.SaveStack(newStack); err != nil {
				c.Logger().Warnf("Failed to create Stack document for deployment %s: %v", deploymentState.ID, err)
				// Don't fail the deployment if Stack creation fails - just log warning
			} else {
				c.Logger().Infof("Created Stack document %s with %d containers", stackID, len(containerIDs))
			}
		} else {
			// Stack exists, update it with new container IDs
			// Merge with existing containers (avoid duplicates)
			existingIDs := make(map[string]bool)
			for _, id := range existingStack.Containers {
				existingIDs[id] = true
			}
			for _, id := range containerIDs {
				if !existingIDs[id] {
					existingStack.Containers = append(existingStack.Containers, id)
				}
			}
			existingStack.Status = "running"
			existingStack.DeployedAt = &deploymentState.StartedAt
			existingStack.UpdatedAt = deploymentState.StartedAt

			if err := s.storage.UpdateStack(existingStack); err != nil {
				c.Logger().Warnf("Failed to update Stack document for deployment %s: %v", deploymentState.ID, err)
				// Don't fail the deployment if Stack update fails - just log warning
			} else {
				c.Logger().Infof("Updated Stack document %s, now has %d containers", stackID, len(existingStack.Containers))
			}
		}
	}

	// Convert to response
	response := &DeploymentStateResponse{
		ID:            deploymentState.ID,
		StackID:       deploymentState.StackID,
		Status:        deploymentState.Status,
		Phase:         deploymentState.Phase,
		Progress:      deploymentState.Progress,
		Placements:    deploymentState.Placements,
		NetworkInfo:   deploymentState.NetworkInfo,
		VolumeInfo:    deploymentState.VolumeInfo,
		Events:        deploymentState.Events,
		StartedAt:     deploymentState.StartedAt,
		CompletedAt:   deploymentState.CompletedAt,
		ErrorMessage:  deploymentState.ErrorMessage,
		RollbackState: deploymentState.RollbackState,
	}

	return c.JSON(http.StatusAccepted, response)
}

// validateJSONLDStack validates a JSON-LD stack definition without deploying.
// @Summary Validate JSON-LD stack
// @Description Validate a JSON-LD stack definition and return any errors or warnings
// @Tags stacks
// @Accept json
// @Produce json
// @Param definition body models.StackDefinition true "JSON-LD stack definition"
// @Success 200 {object} ParseResultResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/stacks/jsonld/validate [post]
func (s *Server) validateJSONLDStack(c echo.Context) error {
	var definition models.StackDefinition
	if err := c.Bind(&definition); err != nil {
		return BadRequestError("Invalid request body", err.Error())
	}

	// Create parser
	resolver := &APIHostResolver{storage: s.storage}
	parser := stack.NewStackParser(resolver)

	// Parse the definition
	result, err := parser.Parse(&definition)

	response := &ParseResultResponse{
		Valid:    len(result.Errors) == 0,
		Warnings: result.Warnings,
		Errors:   result.Errors,
	}

	if err == nil && result.Plan != nil {
		response.StackName = result.Plan.StackNode.Name
		response.ContainerCount = len(result.Plan.ContainerSpecs)
		response.HasNetwork = result.Plan.Network != nil
		response.WaveCount = len(result.Plan.DependencyGraph)
	}

	return c.JSON(http.StatusOK, response)
}

// getJSONLDDeployment retrieves the deployment state for a JSON-LD deployment.
// @Summary Get JSON-LD deployment state
// @Description Get detailed deployment state including progress, events, and placements
// @Tags stacks
// @Accept json
// @Produce json
// @Param id path string true "Deployment ID"
// @Success 200 {object} DeploymentStateResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/stacks/jsonld/deployments/{id} [get]
func (s *Server) getJSONLDDeployment(c echo.Context) error {
	id := c.Param("id")

	var state models.DeploymentState
	if err := s.storage.GetDocument(id, &state); err != nil {
		return NotFoundError("Deployment", id)
	}

	response := &DeploymentStateResponse{
		ID:            state.ID,
		StackID:       state.StackID,
		Status:        state.Status,
		Phase:         state.Phase,
		Progress:      state.Progress,
		Placements:    state.Placements,
		NetworkInfo:   state.NetworkInfo,
		VolumeInfo:    state.VolumeInfo,
		Events:        state.Events,
		StartedAt:     state.StartedAt,
		CompletedAt:   state.CompletedAt,
		ErrorMessage:  state.ErrorMessage,
		RollbackState: state.RollbackState,
	}

	return c.JSON(http.StatusOK, response)
}

// listJSONLDDeployments lists all JSON-LD deployments with optional status filter.
// @Summary List JSON-LD deployments
// @Description List all JSON-LD stack deployments with optional status filter
// @Tags stacks
// @Accept json
// @Produce json
// @Param status query string false "Filter by status"
// @Success 200 {array} DeploymentStateResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/stacks/jsonld/deployments [get]
func (s *Server) listJSONLDDeployments(c echo.Context) error {
	// Get optional status filter
	statusFilter := c.QueryParam("status")

	// Build filters
	filters := make(map[string]interface{})
	if statusFilter != "" {
		filters["status"] = statusFilter
	}

	// Query deployments from storage
	deployments, err := s.storage.ListDeployments(filters)
	if err != nil {
		return InternalError("Failed to list deployments", err.Error())
	}

	// Convert to response format
	responses := make([]DeploymentStateResponse, len(deployments))
	for i, deployment := range deployments {
		responses[i] = DeploymentStateResponse{
			ID:            deployment.ID,
			StackID:       deployment.StackID,
			Status:        deployment.Status,
			Phase:         deployment.Phase,
			Progress:      deployment.Progress,
			Placements:    deployment.Placements,
			NetworkInfo:   deployment.NetworkInfo,
			VolumeInfo:    deployment.VolumeInfo,
			Events:        deployment.Events,
			StartedAt:     deployment.StartedAt,
			CompletedAt:   deployment.CompletedAt,
			ErrorMessage:  deployment.ErrorMessage,
			RollbackState: deployment.RollbackState,
		}
	}

	return c.JSON(http.StatusOK, responses)
}

// APIHostResolver implements the stack.HostResolver interface using the storage layer.
type APIHostResolver struct {
	storage *storage.Storage
}

// ResolveHost resolves an absolute @id URL to host information.
func (r *APIHostResolver) ResolveHost(id string) (*models.HostInfo, error) {
	// For now, treat the @id as the host ID directly
	// In a full implementation, this would parse the URL and query by it
	host, err := r.storage.GetHost(id)
	if err != nil {
		return nil, fmt.Errorf("host %s not found: %w", id, err)
	}

	// Determine Docker socket from agent configuration
	dockerSocket := ""

	// Try to get agent config for this host (agent ID format: "agent:hostId")
	agentID := fmt.Sprintf("agent:%s", id)
	agentConfig, err := r.storage.GetAgentConfig(agentID)
	if err == nil && agentConfig != nil {
		// Use the agent's configured Docker socket
		dockerSocket = agentConfig.DockerSocket
	} else {
		// Fall back to guessing based on IP address
		dockerSocket = fmt.Sprintf("tcp://%s:2375", host.IPAddress)
		if host.IPAddress == "localhost" || host.IPAddress == "127.0.0.1" {
			dockerSocket = "unix:///var/run/docker.sock"
		}
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
			CPUUsage:       0, // TODO: Get actual CPU usage
			MemoryUsage:    0, // TODO: Get actual memory usage
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
func (r *APIHostResolver) ListHosts() ([]*models.HostInfo, error) {
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

// CouchDBAdapter adapts the Storage layer to the stack.Database interface.
type CouchDBAdapter struct {
	storage *storage.Storage
}

// Create creates a new document in CouchDB.
func (a *CouchDBAdapter) Create(ctx context.Context, doc interface{}) error {
	return a.storage.SaveDocument(doc)
}

// Update updates an existing document in CouchDB.
func (a *CouchDBAdapter) Update(ctx context.Context, doc interface{}) error {
	return a.storage.SaveDocument(doc)
}

// APIDockerClientFactory creates Docker clients for different hosts.
type APIDockerClientFactory struct {
	storage *storage.Storage
}

// GetClient returns a Docker client for the given host ID.
// The Docker SDK client already implements common.DockerClient interface.
func (f *APIDockerClientFactory) GetClient(ctx context.Context, hostID string) (common.DockerClient, error) {
	host, err := f.storage.GetHost(hostID)
	if err != nil {
		return nil, fmt.Errorf("host %s not found: %w", hostID, err)
	}

	// Determine Docker socket from agent configuration
	dockerSocket := ""
	sshKeyPath := ""

	// Try to get agent config for this host (agent ID format: "agent:hostId")
	agentID := fmt.Sprintf("agent:%s", hostID)
	agentConfig, err := f.storage.GetAgentConfig(agentID)
	if err == nil && agentConfig != nil {
		// Use the agent's configured Docker socket
		dockerSocket = agentConfig.DockerSocket
		sshKeyPath = agentConfig.SSHKeyPath
	} else {
		// Fall back to guessing based on IP address
		dockerSocket = fmt.Sprintf("tcp://%s:2375", host.IPAddress)
		if host.IPAddress == "localhost" || host.IPAddress == "127.0.0.1" {
			dockerSocket = "unix:///var/run/docker.sock"
		}
	}

	// Use EVE's NewDockerClient which handles SSH tunnels
	cli, err := common.NewDockerClient(dockerSocket, sshKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client for %s (socket: %s): %w", hostID, dockerSocket, err)
	}

	// Docker SDK client already implements common.DockerClient
	return cli, nil
}

// listStacks returns all stacks.
// @Summary List all stacks
// @Description Get a list of all stacks in the system
// @Tags stacks
// @Produce json
// @Success 200 {array} models.Stack
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/stacks [get]
func (s *Server) listStacks(c echo.Context) error {
	stacks, err := s.storage.ListStacks(nil)
	if err != nil {
		return InternalError("Failed to list stacks", err.Error())
	}
	return c.JSON(http.StatusOK, stacks)
}

// getStack returns a single stack by ID.
// @Summary Get stack by ID
// @Description Get detailed information about a specific stack
// @Tags stacks
// @Produce json
// @Param id path string true "Stack ID"
// @Success 200 {object} models.Stack
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/stacks/{id} [get]
func (s *Server) getStack(c echo.Context) error {
	id := c.Param("id")
	stack, err := s.storage.GetStack(id)
	if err != nil {
		return NotFoundError("Stack not found", id)
	}
	return c.JSON(http.StatusOK, stack)
}

// getStackDeployment returns the deployment state for a stack.
// @Summary Get stack deployment state
// @Description Get the current deployment state and container placements for a stack
// @Tags stacks
// @Produce json
// @Param id path string true "Stack ID"
// @Success 200 {object} models.DeploymentState
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/stacks/{id}/deployment [get]
func (s *Server) getStackDeployment(c echo.Context) error {
	id := c.Param("id")

	// Try DeploymentState first (JSON-LD deployments)
	deploymentState, err := s.storage.GetDeploymentState(id)
	if err == nil && deploymentState != nil {
		return c.JSON(http.StatusOK, deploymentState)
	}

	// Fall back to old StackDeployment format
	deployment, err := s.storage.GetDeployment(id)
	if err != nil {
		return NotFoundError("Deployment not found", id)
	}

	return c.JSON(http.StatusOK, deployment)
}
