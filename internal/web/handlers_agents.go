package web

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"evalgo.org/graphium/internal/auth"
	"evalgo.org/graphium/models"
)

// AgentsPage renders the agents list page
func (h *Handler) AgentsPage(c echo.Context) error {
	// Get current user from context
	var user *models.User
	if claims, ok := c.Get("claims").(*auth.Claims); ok {
		user, _ = h.storage.GetUser(claims.UserID)
	}

	// Fetch agents from storage
	agents, err := h.listAgents(c)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to load agents: %v", err))
	}

	return Render(c, AgentsListWithUser(agents, user))
}

// AgentsTableHandler handles HTMX requests for the agents table
func (h *Handler) AgentsTableHandler(c echo.Context) error {
	// Get filter parameters
	search := c.QueryParam("search")
	status := c.QueryParam("status")

	// Fetch agents from API
	agents, err := h.listAgents(c)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to load agents: %v", err))
	}

	// Apply filters
	filtered := filterAgents(agents, search, status)

	return Render(c, AgentsTable(filtered))
}

// AgentDetailPage renders the agent detail page
func (h *Handler) AgentDetailPage(c echo.Context) error {
	// Get current user from context
	var user *models.User
	if claims, ok := c.Get("claims").(*auth.Claims); ok {
		user, _ = h.storage.GetUser(claims.UserID)
	}

	agentID := c.Param("id")
	if agentID == "" {
		return c.String(http.StatusBadRequest, "Agent ID is required")
	}

	// Fetch agent from API
	agent, err := h.getAgentInfo(agentID)
	if err != nil {
		return c.String(http.StatusNotFound, fmt.Sprintf("Agent not found: %v", err))
	}

	return Render(c, AgentDetailWithUser(agent, user))
}

// StartAgentHandler handles starting an agent
func (h *Handler) StartAgentHandler(c echo.Context) error {
	agentID := c.Param("id")
	if agentID == "" {
		return c.String(http.StatusBadRequest, "Agent ID is required")
	}

	// Call API to start agent
	apiURL := fmt.Sprintf("http://localhost:%d/api/v1/agents/%s/start", h.config.Server.Port, agentID)
	req, err := http.NewRequest("POST", apiURL, nil)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to create request: %v", err))
	}

	// TODO: Add auth token from context
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to start agent: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.String(resp.StatusCode, "Failed to start agent")
	}

	// Return updated table
	agents, err := h.listAgents(c)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to load agents: %v", err))
	}

	return Render(c, AgentsTable(agents))
}

// StopAgentHandler handles stopping an agent
func (h *Handler) StopAgentHandler(c echo.Context) error {
	agentID := c.Param("id")
	if agentID == "" {
		return c.String(http.StatusBadRequest, "Agent ID is required")
	}

	// Call API to stop agent
	apiURL := fmt.Sprintf("http://localhost:%d/api/v1/agents/%s/stop", h.config.Server.Port, agentID)
	req, err := http.NewRequest("POST", apiURL, nil)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to create request: %v", err))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to stop agent: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.String(resp.StatusCode, "Failed to stop agent")
	}

	// Return updated table
	agents, err := h.listAgents(c)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to load agents: %v", err))
	}

	return Render(c, AgentsTable(agents))
}

// RestartAgentHandler handles restarting an agent
func (h *Handler) RestartAgentHandler(c echo.Context) error {
	agentID := c.Param("id")
	if agentID == "" {
		return c.String(http.StatusBadRequest, "Agent ID is required")
	}

	// Call API to restart agent
	apiURL := fmt.Sprintf("http://localhost:%d/api/v1/agents/%s/restart", h.config.Server.Port, agentID)
	req, err := http.NewRequest("POST", apiURL, nil)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to create request: %v", err))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to restart agent: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.String(resp.StatusCode, "Failed to restart agent")
	}

	// Return updated table
	agents, err := h.listAgents(c)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to load agents: %v", err))
	}

	return Render(c, AgentsTable(agents))
}

// DeleteAgentHandler handles deleting an agent
func (h *Handler) DeleteAgentHandler(c echo.Context) error {
	agentID := c.Param("id")
	if agentID == "" {
		return c.String(http.StatusBadRequest, "Agent ID is required")
	}

	// Get agent config to get the Rev
	config, err := h.storage.GetAgentConfig(agentID)
	if err != nil {
		return c.String(http.StatusNotFound, "Agent not found")
	}

	// Delete agent config
	if err := h.storage.DeleteAgentConfig(agentID, config.Rev); err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to delete agent: %v", err))
	}

	// Return updated table
	agents, err := h.listAgents(c)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to load agents: %v", err))
	}

	return Render(c, AgentsTable(agents))
}

// Helper function to list agents
func (h *Handler) listAgents(c echo.Context) ([]AgentInfo, error) {
	// Get agent configurations from storage
	configs, err := h.storage.ListAgentConfigs(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list agent configurations: %w", err)
	}

	// For now, create stub states since we don't have direct access to agent manager
	// In a real implementation, you'd call the agent manager through an API
	agents := make([]AgentInfo, len(configs))
	for i, cfg := range configs {
		// Create a default stopped state
		state := &models.AgentState{
			ConfigID:       cfg.ID,
			Status:         "stopped",
			ContainerCount: 0,
		}

		agents[i] = AgentInfo{
			Config: cfg,
			State:  state,
		}
	}

	return agents, nil
}

// Helper function to get agent info
func (h *Handler) getAgentInfo(agentID string) (AgentInfo, error) {
	// Get agent configuration
	config, err := h.storage.GetAgentConfig(agentID)
	if err != nil {
		return AgentInfo{}, fmt.Errorf("failed to get agent configuration: %w", err)
	}

	// Create a default stopped state
	state := &models.AgentState{
		ConfigID:       config.ID,
		Status:         "stopped",
		ContainerCount: 0,
	}

	return AgentInfo{
		Config: config,
		State:  state,
	}, nil
}

// filterAgents filters agents based on search and status
func filterAgents(agents []AgentInfo, search, status string) []AgentInfo {
	if search == "" && status == "" {
		return agents
	}

	filtered := make([]AgentInfo, 0, len(agents))
	search = strings.ToLower(search)

	for _, agent := range agents {
		// Apply status filter
		if status != "" && agent.State.Status != status {
			continue
		}

		// Apply search filter
		if search != "" {
			nameMatch := strings.Contains(strings.ToLower(agent.Config.Name), search)
			hostMatch := strings.Contains(strings.ToLower(agent.Config.HostID), search)
			if !nameMatch && !hostMatch {
				continue
			}
		}

		filtered = append(filtered, agent)
	}

	return filtered
}
