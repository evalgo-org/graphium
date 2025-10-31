package api

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"evalgo.org/graphium/models"
)

// listAgents handles GET /api/v1/agents
// @Summary List agents
// @Description Get a list of all agent configurations with their runtime states
// @Tags Agents
// @Accept json
// @Produce json
// @Param enabled query boolean false "Filter by enabled status"
// @Param hostId query string false "Filter by host ID"
// @Param datacenter query string false "Filter by datacenter"
// @Success 200 {object} AgentListResponse
// @Failure 500 {object} ErrorResponse
// @Router /agents [get]
func (s *Server) listAgents(c echo.Context) error {
	// Parse query parameters
	filters := make(map[string]interface{})

	if enabled := c.QueryParam("enabled"); enabled != "" {
		filters["enabled"] = enabled == "true"
	}
	if hostID := c.QueryParam("hostId"); hostID != "" {
		filters["hostId"] = hostID
	}
	if datacenter := c.QueryParam("datacenter"); datacenter != "" {
		filters["datacenter"] = datacenter
	}

	// Get agent configurations from storage
	configs, err := s.storage.ListAgentConfigs(filters)
	if err != nil {
		return InternalError("Failed to list agent configurations", err.Error())
	}

	// Get runtime states from agent manager
	states, err := s.agentManager.ListAgentStates()
	if err != nil {
		return InternalError("Failed to list agent states", err.Error())
	}

	// Build state map for quick lookup
	stateMap := make(map[string]*models.AgentState)
	for _, state := range states {
		stateMap[state.ConfigID] = state
	}

	// Combine configs with states
	agents := make([]AgentInfo, len(configs))
	for i, cfg := range configs {
		state := stateMap[cfg.ID]
		if state == nil {
			// No state found, agent is stopped
			state = &models.AgentState{
				ConfigID: cfg.ID,
				Status:   "stopped",
			}
		}

		agents[i] = AgentInfo{
			Config: cfg,
			State:  state,
		}
	}

	return c.JSON(http.StatusOK, AgentListResponse{
		Count:  len(agents),
		Agents: agents,
	})
}

// getAgent handles GET /api/v1/agents/:id
// @Summary Get an agent by ID
// @Description Retrieve detailed information about a specific agent including its configuration and runtime state
// @Tags Agents
// @Accept json
// @Produce json
// @Param id path string true "Agent ID (e.g., agent:host-01)"
// @Success 200 {object} AgentInfo
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /agents/{id} [get]
func (s *Server) getAgent(c echo.Context) error {
	id := c.Param("id")

	if id == "" {
		return BadRequestError("Agent ID is required", "The 'id' parameter cannot be empty")
	}

	// Get agent configuration
	config, err := s.storage.GetAgentConfig(id)
	if err != nil {
		return NotFoundError("Agent", id)
	}

	// Get runtime state
	state, err := s.agentManager.GetAgentState(id)
	if err != nil {
		return InternalError("Failed to get agent state", err.Error())
	}

	return c.JSON(http.StatusOK, AgentInfo{
		Config: config,
		State:  state,
	})
}

// createAgent handles POST /api/v1/agents
// @Summary Create a new agent
// @Description Create a new agent configuration. The agent will not start automatically unless autoStart is true.
// @Tags Agents
// @Accept json
// @Produce json
// @Param agent body models.AgentConfig true "Agent configuration to create"
// @Success 201 {object} models.AgentConfig
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /agents [post]
func (s *Server) createAgent(c echo.Context) error {
	var config models.AgentConfig

	if err := c.Bind(&config); err != nil {
		return BadRequestError("Invalid request body", "Failed to parse JSON: "+err.Error())
	}

	// Validate required fields
	if config.Name == "" {
		return BadRequestError("Validation failed", "Agent name is required")
	}
	if config.HostID == "" {
		return BadRequestError("Validation failed", "Host ID is required")
	}
	if config.DockerSocket == "" {
		return BadRequestError("Validation failed", "Docker socket is required")
	}

	// Generate ID if not provided
	if config.ID == "" {
		config.ID = fmt.Sprintf("agent:%s", config.HostID)
	}

	// Set defaults
	if config.SyncInterval == 0 {
		config.SyncInterval = 30 // Default 30 seconds
	}

	// Save configuration
	if err := s.storage.SaveAgentConfig(&config); err != nil {
		return InternalError("Failed to save agent configuration", err.Error())
	}

	// Auto-start if requested
	if config.Enabled && config.AutoStart {
		if err := s.agentManager.StartAgent(config.ID); err != nil {
			// Log warning but don't fail the creation
			c.Logger().Warnf("Failed to auto-start agent %s: %v", config.ID, err)
		}
	}

	return c.JSON(http.StatusCreated, config)
}

// updateAgent handles PUT /api/v1/agents/:id
// @Summary Update an agent
// @Description Update an existing agent configuration. The agent will be restarted if it's currently running.
// @Tags Agents
// @Accept json
// @Produce json
// @Param id path string true "Agent ID"
// @Param agent body models.AgentConfig true "Updated agent configuration"
// @Success 200 {object} models.AgentConfig
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /agents/{id} [put]
func (s *Server) updateAgent(c echo.Context) error {
	id := c.Param("id")

	if id == "" {
		return BadRequestError("Agent ID is required", "The 'id' parameter cannot be empty")
	}

	// Check if agent exists
	existing, err := s.storage.GetAgentConfig(id)
	if err != nil {
		return NotFoundError("Agent", id)
	}

	var config models.AgentConfig
	if err := c.Bind(&config); err != nil {
		return BadRequestError("Invalid request body", "Failed to parse JSON: "+err.Error())
	}

	// Preserve ID and Rev
	config.ID = existing.ID
	config.Rev = existing.Rev

	// Validate required fields
	if config.Name == "" {
		return BadRequestError("Validation failed", "Agent name is required")
	}
	if config.HostID == "" {
		return BadRequestError("Validation failed", "Host ID is required")
	}
	if config.DockerSocket == "" {
		return BadRequestError("Validation failed", "Docker socket is required")
	}

	// Check if agent is currently running
	state, err := s.agentManager.GetAgentState(id)
	if err != nil {
		return InternalError("Failed to get agent state", err.Error())
	}

	wasRunning := state.Status == "running"

	// Save updated configuration
	if err := s.storage.UpdateAgentConfig(&config); err != nil {
		return InternalError("Failed to update agent configuration", err.Error())
	}

	// Restart agent if it was running
	if wasRunning {
		if err := s.agentManager.RestartAgent(id); err != nil {
			c.Logger().Warnf("Failed to restart agent %s after update: %v", id, err)
		}
	}

	return c.JSON(http.StatusOK, config)
}

// deleteAgent handles DELETE /api/v1/agents/:id
// @Summary Delete an agent
// @Description Delete an agent configuration. The agent will be stopped if it's currently running.
// @Tags Agents
// @Accept json
// @Produce json
// @Param id path string true "Agent ID"
// @Success 204 "Agent deleted successfully"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /agents/{id} [delete]
func (s *Server) deleteAgent(c echo.Context) error {
	id := c.Param("id")

	if id == "" {
		return BadRequestError("Agent ID is required", "The 'id' parameter cannot be empty")
	}

	// Check if agent exists
	config, err := s.storage.GetAgentConfig(id)
	if err != nil {
		return NotFoundError("Agent", id)
	}

	// Stop agent if running
	state, err := s.agentManager.GetAgentState(id)
	if err == nil && state.Status == "running" {
		if err := s.agentManager.StopAgent(id); err != nil {
			c.Logger().Warnf("Failed to stop agent %s before deletion: %v", id, err)
		}
	}

	// Delete configuration from storage
	if err := s.storage.DeleteAgentConfig(id, config.Rev); err != nil {
		return InternalError("Failed to delete agent configuration", err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}

// startAgent handles POST /api/v1/agents/:id/start
// @Summary Start an agent
// @Description Start a stopped agent process
// @Tags Agents
// @Accept json
// @Produce json
// @Param id path string true "Agent ID"
// @Success 200 {object} MessageResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /agents/{id}/start [post]
func (s *Server) startAgent(c echo.Context) error {
	id := c.Param("id")

	if id == "" {
		return BadRequestError("Agent ID is required", "The 'id' parameter cannot be empty")
	}

	// Check if agent config exists
	_, err := s.storage.GetAgentConfig(id)
	if err != nil {
		return NotFoundError("Agent", id)
	}

	// Start the agent
	if err := s.agentManager.StartAgent(id); err != nil {
		return InternalError("Failed to start agent", err.Error())
	}

	return c.JSON(http.StatusOK, MessageResponse{
		Message: fmt.Sprintf("Agent %s started successfully", id),
	})
}

// stopAgent handles POST /api/v1/agents/:id/stop
// @Summary Stop an agent
// @Description Stop a running agent process
// @Tags Agents
// @Accept json
// @Produce json
// @Param id path string true "Agent ID"
// @Success 200 {object} MessageResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /agents/{id}/stop [post]
func (s *Server) stopAgent(c echo.Context) error {
	id := c.Param("id")

	if id == "" {
		return BadRequestError("Agent ID is required", "The 'id' parameter cannot be empty")
	}

	// Check if agent config exists
	_, err := s.storage.GetAgentConfig(id)
	if err != nil {
		return NotFoundError("Agent", id)
	}

	// Stop the agent
	if err := s.agentManager.StopAgent(id); err != nil {
		return InternalError("Failed to stop agent", err.Error())
	}

	return c.JSON(http.StatusOK, MessageResponse{
		Message: fmt.Sprintf("Agent %s stopped successfully", id),
	})
}

// restartAgent handles POST /api/v1/agents/:id/restart
// @Summary Restart an agent
// @Description Restart a running agent process (stop then start)
// @Tags Agents
// @Accept json
// @Produce json
// @Param id path string true "Agent ID"
// @Success 200 {object} MessageResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /agents/{id}/restart [post]
func (s *Server) restartAgent(c echo.Context) error {
	id := c.Param("id")

	if id == "" {
		return BadRequestError("Agent ID is required", "The 'id' parameter cannot be empty")
	}

	// Check if agent config exists
	_, err := s.storage.GetAgentConfig(id)
	if err != nil {
		return NotFoundError("Agent", id)
	}

	// Restart the agent
	if err := s.agentManager.RestartAgent(id); err != nil {
		return InternalError("Failed to restart agent", err.Error())
	}

	return c.JSON(http.StatusOK, MessageResponse{
		Message: fmt.Sprintf("Agent %s restarted successfully", id),
	})
}
