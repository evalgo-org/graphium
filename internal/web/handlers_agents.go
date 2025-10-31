package web

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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

// CreateAgentFormHandler renders the create agent form
func (h *Handler) CreateAgentFormHandler(c echo.Context) error {
	// Get current user from context
	var user *models.User
	if claims, ok := c.Get("claims").(*auth.Claims); ok {
		user, _ = h.storage.GetUser(claims.UserID)
	}

	return Render(c, CreateAgentFormWithUser("", user))
}

// CreateAgentHandler handles creating a new agent
func (h *Handler) CreateAgentHandler(c echo.Context) error {
	// Get current user from context
	var user *models.User
	if claims, ok := c.Get("claims").(*auth.Claims); ok {
		user, _ = h.storage.GetUser(claims.UserID)
	}

	// Parse form data
	name := c.FormValue("name")
	hostID := c.FormValue("hostid")
	dockerSocket := c.FormValue("docker_socket")
	sshKeyPath := c.FormValue("ssh_key_path")
	datacenter := c.FormValue("datacenter")
	syncInterval := 30
	if c.FormValue("sync_interval") != "" {
		fmt.Sscanf(c.FormValue("sync_interval"), "%d", &syncInterval)
	}
	enabled := c.FormValue("enabled") == "true"
	autoStart := c.FormValue("auto_start") == "true"

	// Validate required fields
	if name == "" || hostID == "" {
		return Render(c, CreateAgentFormWithUser("Name and Host ID are required", user))
	}

	// Set defaults
	if dockerSocket == "" {
		dockerSocket = "/var/run/docker.sock"
	}

	// Create agent configuration
	config := &models.AgentConfig{
		ID:           fmt.Sprintf("agent:%s", hostID),
		Name:         name,
		HostID:       hostID,
		DockerSocket: dockerSocket,
		SSHKeyPath:   sshKeyPath,
		Datacenter:   datacenter,
		SyncInterval: syncInterval,
		Enabled:      enabled,
		AutoStart:    autoStart,
	}

	// Save configuration
	if err := h.storage.SaveAgentConfig(config); err != nil {
		return Render(c, CreateAgentFormWithUser(fmt.Sprintf("Failed to create agent: %v", err), user))
	}

	// If enabled, start the agent via API
	if enabled {
		apiURL := fmt.Sprintf("http://localhost:%d/api/v1/agents/%s/start", h.config.Server.Port, config.ID)
		req, err := http.NewRequest("POST", apiURL, nil)
		if err == nil {
			http.DefaultClient.Do(req)
		}
	}

	// Redirect to agents list
	return c.Redirect(http.StatusSeeOther, "/web/agents")
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

	// Get real agent states from agent manager
	agents := make([]AgentInfo, len(configs))
	for i, cfg := range configs {
		// Get actual state from agent manager
		var state *models.AgentState
		if h.agentManager != nil {
			state, err = h.agentManager.GetAgentState(cfg.ID)
			if err != nil {
				// If error, create a default stopped state
				state = &models.AgentState{
					ConfigID:       cfg.ID,
					Status:         "stopped",
					ContainerCount: 0,
				}
			}
		} else {
			// Fallback if no agent manager
			state = &models.AgentState{
				ConfigID:       cfg.ID,
				Status:         "stopped",
				ContainerCount: 0,
			}
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

	// Get actual state from agent manager
	var state *models.AgentState
	if h.agentManager != nil {
		state, err = h.agentManager.GetAgentState(config.ID)
		if err != nil {
			// If error, create a default stopped state
			state = &models.AgentState{
				ConfigID:       config.ID,
				Status:         "stopped",
				ContainerCount: 0,
			}
		}
	} else {
		// Fallback if no agent manager
		state = &models.AgentState{
			ConfigID:       config.ID,
			Status:         "stopped",
			ContainerCount: 0,
		}
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

// AgentLogsHandler handles agent log requests with optional tail parameter
func (h *Handler) AgentLogsHandler(c echo.Context) error {
	agentID := c.Param("id")
	if agentID == "" {
		return c.String(http.StatusBadRequest, "Agent ID is required")
	}

	// Get agent config to verify it exists and get the host ID
	config, err := h.storage.GetAgentConfig(agentID)
	if err != nil {
		return c.String(http.StatusNotFound, "Agent not found")
	}

	// Get tail parameter (number of lines from end)
	tailStr := c.QueryParam("tail")
	tail := 100 // default
	if tailStr != "" {
		if parsed, err := strconv.Atoi(tailStr); err == nil && parsed > 0 {
			tail = parsed
		}
	}

	// Get logs path from config
	logsPath := h.config.Agents.LogsPath
	if logsPath == "" {
		logsPath = "./logs"
	}

	// Construct log file path
	logFilePath := filepath.Join(logsPath, fmt.Sprintf("%s.log", config.HostID))

	// Check if log file exists
	if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
		return c.String(http.StatusOK, "No logs available yet")
	}

	// Read the last N lines from the log file
	lines, err := tailFile(logFilePath, tail)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to read log file: %v", err))
	}

	// Set response headers
	c.Response().Header().Set(echo.HeaderContentType, "text/plain; charset=utf-8")
	c.Response().Header().Set("X-Agent-ID", agentID)
	c.Response().Header().Set("X-Agent-HostID", config.HostID)
	c.Response().WriteHeader(http.StatusOK)

	// Write the lines
	for _, line := range lines {
		if _, err := c.Response().Write([]byte(line + "\n")); err != nil {
			return err
		}
	}

	return nil
}

// AgentLogsDownloadHandler handles agent log download requests
func (h *Handler) AgentLogsDownloadHandler(c echo.Context) error {
	agentID := c.Param("id")
	if agentID == "" {
		return c.String(http.StatusBadRequest, "Agent ID is required")
	}

	// Get agent config to verify it exists and get the host ID
	config, err := h.storage.GetAgentConfig(agentID)
	if err != nil {
		return c.String(http.StatusNotFound, "Agent not found")
	}

	// Get lines parameter
	linesStr := c.QueryParam("lines")
	lines := 1000 // default
	if linesStr != "" {
		if parsed, err := strconv.Atoi(linesStr); err == nil && parsed > 0 {
			lines = parsed
		}
	}

	// Get logs path from config
	logsPath := h.config.Agents.LogsPath
	if logsPath == "" {
		logsPath = "./logs"
	}

	// Construct log file path
	logFilePath := filepath.Join(logsPath, fmt.Sprintf("%s.log", config.HostID))

	// Check if log file exists
	if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
		return c.String(http.StatusNotFound, "Log file not found")
	}

	// Read the last N lines
	logLines, err := tailFile(logFilePath, lines)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to read log file: %v", err))
	}

	// Set download headers
	filename := fmt.Sprintf("%s-agent-logs-%s.txt", config.HostID, time.Now().Format("20060102-150405"))
	c.Response().Header().Set(echo.HeaderContentType, "text/plain; charset=utf-8")
	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("attachment; filename=%s", filename))
	c.Response().WriteHeader(http.StatusOK)

	// Write the lines
	for _, line := range logLines {
		if _, err := c.Response().Write([]byte(line + "\n")); err != nil {
			return err
		}
	}

	return nil
}

// tailFile reads the last n lines from a file
func tailFile(filePath string, n int) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Get file size
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := stat.Size()

	// If file is small enough, just read all lines
	if fileSize < 10000 {
		scanner := bufio.NewScanner(file)
		var lines []string
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			return nil, err
		}

		// Return last n lines
		if len(lines) > n {
			return lines[len(lines)-n:], nil
		}
		return lines, nil
	}

	// For larger files, use a more efficient approach
	// Read from the end of the file
	const avgLineSize = 100 // Estimated average line size
	startPos := fileSize - int64(n*avgLineSize)
	if startPos < 0 {
		startPos = 0
	}

	_, err = file.Seek(startPos, io.SeekStart)
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(file)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Return last n lines
	if len(lines) > n {
		return lines[len(lines)-n:], nil
	}
	return lines, nil
}
