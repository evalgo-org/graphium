package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"evalgo.org/graphium/internal/auth"
	"evalgo.org/graphium/models"
)

// ActionsPage renders the scheduled actions list page
func (h *Handler) ActionsPage(c echo.Context) error {
	// Get current user from context
	var user *models.User
	if claims, ok := c.Get("claims").(*auth.Claims); ok {
		user, _ = h.storage.GetUser(claims.UserID)
	}

	// Fetch actions from storage
	actions, err := h.storage.ListScheduledActions(nil)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to load actions: %v", err))
	}

	return Render(c, ActionsListWithUser(actions, user))
}

// ActionsTableHandler handles HTMX requests for the actions table
func (h *Handler) ActionsTableHandler(c echo.Context) error {
	// Get filter parameters
	search := c.QueryParam("search")
	actionType := c.QueryParam("type")
	status := c.QueryParam("status")
	agentID := c.QueryParam("agent")

	// Build filters
	filters := make(map[string]interface{})
	if actionType != "" {
		filters["@type"] = actionType
	}
	if status != "" {
		filters["actionStatus"] = status
	}
	if agentID != "" {
		filters["agent"] = agentID
	}

	// Fetch actions from storage
	actions, err := h.storage.ListScheduledActions(filters)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to load actions: %v", err))
	}

	// Apply search filter in memory (for name/description)
	if search != "" {
		filtered := make([]*models.ScheduledAction, 0)
		searchLower := strings.ToLower(search)
		for _, action := range actions {
			nameMatch := strings.Contains(strings.ToLower(action.Name), searchLower)
			descMatch := strings.Contains(strings.ToLower(action.Description), searchLower)
			if nameMatch || descMatch {
				filtered = append(filtered, action)
			}
		}
		actions = filtered
	}

	return Render(c, ActionsTable(actions))
}

// ActionDetailPage renders the action detail page with execution history
func (h *Handler) ActionDetailPage(c echo.Context) error {
	// Get current user from context
	var user *models.User
	if claims, ok := c.Get("claims").(*auth.Claims); ok {
		user, _ = h.storage.GetUser(claims.UserID)
	}

	actionID := c.Param("id")
	if actionID == "" {
		return c.String(http.StatusBadRequest, "Action ID is required")
	}

	// Fetch action from storage
	action, err := h.storage.GetScheduledAction(actionID)
	if err != nil {
		return c.String(http.StatusNotFound, fmt.Sprintf("Action not found: %v", err))
	}

	// Fetch execution history (tasks created by this action)
	tasks, err := h.storage.GetTasksByScheduledAction(actionID)
	if err != nil {
		// Log error but continue - we can still show the action details
		tasks = []*models.AgentTask{}
	}

	return Render(c, ActionDetailWithUser(action, tasks, user))
}

// CreateActionFormHandler renders the create action form
func (h *Handler) CreateActionFormHandler(c echo.Context) error {
	// Get current user from context
	var user *models.User
	if claims, ok := c.Get("claims").(*auth.Claims); ok {
		user, _ = h.storage.GetUser(claims.UserID)
	}

	return Render(c, CreateActionFormWithUser("", user))
}

// CreateActionHandler handles creating a new scheduled action
func (h *Handler) CreateActionHandler(c echo.Context) error {
	// Get current user from context
	var user *models.User
	if claims, ok := c.Get("claims").(*auth.Claims); ok {
		user, _ = h.storage.GetUser(claims.UserID)
	}

	// Parse form data
	name := c.FormValue("name")
	description := c.FormValue("description")
	actionType := c.FormValue("action_type")
	agentID := c.FormValue("agent_id")
	repeatFrequency := c.FormValue("repeat_frequency")
	timezone := c.FormValue("timezone")
	enabled := c.FormValue("enabled") == "on"

	// Validate required fields
	if name == "" {
		return Render(c, CreateActionFormWithUser("Action name is required", user))
	}
	if agentID == "" {
		return Render(c, CreateActionFormWithUser("Agent ID is required", user))
	}
	if repeatFrequency == "" {
		return Render(c, CreateActionFormWithUser("Repeat frequency is required", user))
	}

	// Set defaults
	if timezone == "" {
		timezone = "UTC"
	}
	if actionType == "" {
		actionType = models.ActionTypeCheck
	}

	// Create schedule
	schedule := &models.Schedule{
		Type:             "Schedule",
		RepeatFrequency:  repeatFrequency,
		ScheduleTimezone: timezone,
	}

	// Build instrument (parameters) based on action type
	instrument := make(map[string]interface{})

	// For CheckAction, get health check parameters
	if actionType == models.ActionTypeCheck {
		url := c.FormValue("check_url")
		method := c.FormValue("check_method")
		expectedStatus := c.FormValue("check_expected_status")
		timeout := c.FormValue("check_timeout")

		if url == "" {
			return Render(c, CreateActionFormWithUser("URL is required for health checks", user))
		}

		instrument["url"] = url
		if method != "" {
			instrument["method"] = method
		} else {
			instrument["method"] = "GET"
		}
		if expectedStatus != "" {
			var status int
			fmt.Sscanf(expectedStatus, "%d", &status)
			instrument["expectedStatusCode"] = status
		} else {
			instrument["expectedStatusCode"] = 200
		}
		if timeout != "" {
			var timeoutSec int
			fmt.Sscanf(timeout, "%d", &timeoutSec)
			instrument["timeout"] = timeoutSec
		} else {
			instrument["timeout"] = 5
		}
	}

	// Create scheduled action
	action := &models.ScheduledAction{
		Context:      "https://schema.org",
		Type:         actionType,
		ID:           models.GenerateID("action"),
		Name:         name,
		Description:  description,
		ActionStatus: models.ActionStatusPotential,
		Agent:        agentID,
		Schedule:     schedule,
		Instrument:   instrument,
		Enabled:      enabled,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Save to storage
	if err := h.storage.CreateScheduledAction(action); err != nil {
		return Render(c, CreateActionFormWithUser(fmt.Sprintf("Failed to create action: %v", err), user))
	}

	// Redirect to actions list
	return c.Redirect(http.StatusSeeOther, "/web/actions")
}

// ExecuteActionHandler handles immediate execution of an action
func (h *Handler) ExecuteActionHandler(c echo.Context) error {
	actionID := c.Param("id")
	if actionID == "" {
		return c.String(http.StatusBadRequest, "Action ID is required")
	}

	// Call API to execute action
	apiURL := fmt.Sprintf("http://localhost:%d/api/v1/actions/%s/execute", h.config.Server.Port, actionID)
	req, err := http.NewRequest("POST", apiURL, nil)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to create request: %v", err))
	}

	// Add auth token from context if available
	token := c.Request().Header.Get("Authorization")
	if token != "" {
		req.Header.Set("Authorization", token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to execute action: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return c.String(resp.StatusCode, fmt.Sprintf("Failed to execute action: %s", string(body)))
	}

	// Parse response to get task ID
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
		if taskID, ok := result["taskId"].(string); ok {
			return c.JSON(http.StatusOK, map[string]string{
				"message": "Action executed successfully",
				"taskId":  taskID,
			})
		}
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "Action executed successfully",
	})
}

// ToggleActionHandler handles enabling/disabling an action
func (h *Handler) ToggleActionHandler(c echo.Context) error {
	actionID := c.Param("id")
	if actionID == "" {
		return c.String(http.StatusBadRequest, "Action ID is required")
	}

	// Get the action
	action, err := h.storage.GetScheduledAction(actionID)
	if err != nil {
		return c.String(http.StatusNotFound, "Action not found")
	}

	// Toggle enabled state
	action.Enabled = !action.Enabled
	action.UpdatedAt = time.Now()

	// Update in storage
	if err := h.storage.UpdateScheduledAction(action); err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to update action: %v", err))
	}

	// Return updated table
	actions, err := h.storage.ListScheduledActions(nil)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to load actions: %v", err))
	}

	return Render(c, ActionsTable(actions))
}

// DeleteActionHandler handles deleting an action
func (h *Handler) DeleteActionHandler(c echo.Context) error {
	actionID := c.Param("id")
	if actionID == "" {
		return c.String(http.StatusBadRequest, "Action ID is required")
	}

	// Verify action exists
	_, err := h.storage.GetScheduledAction(actionID)
	if err != nil {
		return c.String(http.StatusNotFound, "Action not found")
	}

	// Call API to delete action
	apiURL := fmt.Sprintf("http://localhost:%d/api/v1/actions/%s", h.config.Server.Port, actionID)
	req, err := http.NewRequest("DELETE", apiURL, nil)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to create request: %v", err))
	}

	// Add auth token from context if available
	token := c.Request().Header.Get("Authorization")
	if token != "" {
		req.Header.Set("Authorization", token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to delete action: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return c.String(resp.StatusCode, fmt.Sprintf("Failed to delete action: %s", string(body)))
	}

	// Return updated table
	actions, err := h.storage.ListScheduledActions(nil)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to load actions: %v", err))
	}

	return Render(c, ActionsTable(actions))
}

// UpdateActionHandler handles updating an action
func (h *Handler) UpdateActionHandler(c echo.Context) error {
	actionID := c.Param("id")
	if actionID == "" {
		return c.String(http.StatusBadRequest, "Action ID is required")
	}

	// Get the existing action
	action, err := h.storage.GetScheduledAction(actionID)
	if err != nil {
		return c.String(http.StatusNotFound, "Action not found")
	}

	// Parse form data
	name := c.FormValue("name")
	description := c.FormValue("description")
	enabled := c.FormValue("enabled") == "on"

	// Update fields
	if name != "" {
		action.Name = name
	}
	action.Description = description
	action.Enabled = enabled
	action.UpdatedAt = time.Now()

	// Build update request
	updateData, err := json.Marshal(action)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to marshal action data")
	}

	// Call API to update action
	apiURL := fmt.Sprintf("http://localhost:%d/api/v1/actions/%s", h.config.Server.Port, actionID)
	req, err := http.NewRequest("PUT", apiURL, bytes.NewBuffer(updateData))
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to create request: %v", err))
	}

	req.Header.Set("Content-Type", "application/json")

	// Add auth token from context if available
	token := c.Request().Header.Get("Authorization")
	if token != "" {
		req.Header.Set("Authorization", token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to update action: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return c.String(resp.StatusCode, fmt.Sprintf("Failed to update action: %s", string(body)))
	}

	// Redirect to action detail page
	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/web/actions/%s", actionID))
}
