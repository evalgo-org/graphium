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

// TaskWithResult wraps an AgentTask with its parsed result for template rendering
type TaskWithResult struct {
	Task   *models.AgentTask
	Result *models.TaskResult
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

	// Wrap tasks with parsed results
	tasksWithResults := make([]*TaskWithResult, len(tasks))
	for i, task := range tasks {
		result, _ := task.GetResult() // Ignore error, result will be nil if parsing fails
		tasksWithResults[i] = &TaskWithResult{
			Task:   task,
			Result: result,
		}
	}

	return Render(c, ActionDetailWithUser(action, tasksWithResults, user))
}

// ActionExecutionHistoryHandler returns just the execution history table rows for HTMX updates
func (h *Handler) ActionExecutionHistoryHandler(c echo.Context) error {
	actionID := c.Param("id")
	if actionID == "" {
		return c.String(http.StatusBadRequest, "Action ID is required")
	}

	// Fetch execution history (tasks created by this action)
	tasks, err := h.storage.GetTasksByScheduledAction(actionID)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to fetch tasks: %v", err))
	}

	// Wrap tasks with parsed results
	tasksWithResults := make([]*TaskWithResult, len(tasks))
	for i, task := range tasks {
		result, _ := task.GetResult()
		tasksWithResults[i] = &TaskWithResult{
			Task:   task,
			Result: result,
		}
	}

	return Render(c, ExecutionHistoryRows(tasksWithResults))
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

	// Check if JSON-LD input is provided
	actionJSON := c.FormValue("action_json")

	var action *models.ScheduledAction

	if actionJSON != "" {
		// Parse JSON-LD input
		action = &models.ScheduledAction{}
		if err := json.Unmarshal([]byte(actionJSON), action); err != nil {
			return Render(c, CreateActionFormWithUser(fmt.Sprintf("Invalid JSON: %v", err), user))
		}

		// Generate ID if not provided
		if action.ID == "" {
			action.ID = models.GenerateID("action")
		}

		// Set defaults
		if action.Context == "" {
			action.Context = "https://schema.org"
		}
		if action.ActionStatus == "" {
			action.ActionStatus = models.ActionStatusPotential
		}

		// Set timestamps
		action.CreatedAt = time.Now()
		action.UpdatedAt = time.Now()

		// Validate required fields
		if action.Name == "" {
			return Render(c, CreateActionFormWithUser("Action name is required in JSON", user))
		}
		if action.Agent == "" {
			return Render(c, CreateActionFormWithUser("Agent ID is required in JSON", user))
		}
		if action.Schedule == nil || action.Schedule.RepeatFrequency == "" {
			return Render(c, CreateActionFormWithUser("Schedule with repeat frequency is required in JSON", user))
		}
	} else {
		// Parse form data
		name := c.FormValue("name")
		description := c.FormValue("description")
		actionType := c.FormValue("action_type")
		agentID := c.FormValue("agent_id")
		repeatFrequency := c.FormValue("repeat_frequency")
		timezone := c.FormValue("timezone")
		enabled := c.FormValue("enabled") == "on" || c.FormValue("enabled") == "true"

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
		action = &models.ScheduledAction{
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

	// Get action details first
	action, err := h.storage.GetScheduledAction(actionID)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to get action: %v", err))
	}

	// Call API to execute action
	apiURL := fmt.Sprintf("http://localhost:%d/api/v1/actions/%s/execute", h.config.Server.Port, actionID)
	req, err := http.NewRequest("POST", apiURL, nil)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to create request: %v", err))
	}

	// Set Content-Type header for API
	req.Header.Set("Content-Type", "application/json")

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
		return c.HTML(http.StatusOK, fmt.Sprintf(`
			<div class="alert alert-error" style="position: relative; padding-right: 3rem;">
				<button onclick="document.getElementById('execution-response').innerHTML=''"
					style="position: absolute; top: 10px; right: 10px; background: none; border: none; font-size: 1.5em; cursor: pointer; color: inherit; opacity: 0.7; line-height: 1;"
					onmouseover="this.style.opacity='1'"
					onmouseout="this.style.opacity='0.7'"
				>&times;</button>
				<strong>✗ Execution Failed</strong><br>
				<strong>Action:</strong> %s<br>
				<div style="margin-top: 8px; padding: 8px; background: rgba(0,0,0,0.1); border-radius: 4px;">
					<small>%s</small>
				</div>
			</div>
		`, action.Name, string(body)))
	}

	// Parse response to get task ID
	var result map[string]interface{}
	taskID := ""
	if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
		if tid, ok := result["taskId"].(string); ok {
			taskID = tid
		}
	}

	// Build detailed execution information based on action type
	var detailsHTML string
	actionTypeDesc := "Action"

	switch action.Type {
	case "CheckAction":
		// Check if this is a TLS certificate check or HTTP health check
		if action.Instrument != nil {
			checkType, _ := action.Instrument["checkType"].(string)

			if checkType == "tls-certificate" {
				actionTypeDesc = "TLS Certificate Check"
				url, _ := action.Instrument["url"].(string)
				warnDays, _ := action.Instrument["warnDays"].(float64)

				if warnDays == 0 {
					warnDays = 30
				}

				detailsHTML = fmt.Sprintf(`
					<div style="margin-top: 12px; padding: 12px; background: rgba(0,0,0,0.05); border-radius: 4px; border-left: 3px solid #f59e0b;">
						<strong style="display: block; margin-bottom: 8px;">🔒 Execution Details:</strong>
						<div style="margin-left: 12px;">
							<div style="margin-bottom: 4px;">• <strong>Target URL:</strong> <code>%s</code></div>
							<div style="margin-bottom: 4px;">• <strong>Warning Threshold:</strong> %d days before expiration</div>
							<div style="margin-bottom: 4px;">• <strong>Schedule:</strong> %s</div>
						</div>
					</div>
					<div style="margin-top: 12px; padding: 12px; background: rgba(245, 158, 11, 0.1); border-radius: 4px;">
						<strong>✓ Success Criteria:</strong>
						<div style="margin-top: 6px; margin-left: 12px;">
							The agent will connect to <code>%s</code> and verify:
							<ul style="margin: 6px 0 0 20px;">
								<li>TLS certificate is valid and trusted</li>
								<li>Certificate is not expired</li>
								<li>Certificate expiration > %d days</li>
								<li>No connection errors occur</li>
							</ul>
						</div>
					</div>
				`, url, int(warnDays), formatSchedule(action.Schedule), url, int(warnDays))
			} else {
				// HTTP health check
				actionTypeDesc = "HTTP Health Check"
				url, _ := action.Instrument["url"].(string)
				method, _ := action.Instrument["method"].(string)
				expectedStatus, _ := action.Instrument["expectedStatusCode"].(float64)
				timeout, _ := action.Instrument["timeout"].(float64)

				if method == "" {
					method = "GET"
				}
				if expectedStatus == 0 {
					expectedStatus = 200
				}
				if timeout == 0 {
					timeout = 10
				}

				detailsHTML = fmt.Sprintf(`
					<div style="margin-top: 12px; padding: 12px; background: rgba(0,0,0,0.05); border-radius: 4px; border-left: 3px solid #10b981;">
						<strong style="display: block; margin-bottom: 8px;">📋 Execution Details:</strong>
						<div style="margin-left: 12px;">
							<div style="margin-bottom: 4px;">• <strong>Check URL:</strong> <code>%s %s</code></div>
							<div style="margin-bottom: 4px;">• <strong>Acceptance Criteria:</strong> HTTP status code = %d</div>
							<div style="margin-bottom: 4px;">• <strong>Timeout:</strong> %d seconds</div>
							<div style="margin-bottom: 4px;">• <strong>Schedule:</strong> %s</div>
						</div>
					</div>
					<div style="margin-top: 12px; padding: 12px; background: rgba(16, 185, 129, 0.1); border-radius: 4px;">
						<strong>✓ Success Criteria:</strong>
						<div style="margin-top: 6px; margin-left: 12px;">
							The agent will perform an HTTP %s request to <code>%s</code> and verify:
							<ul style="margin: 6px 0 0 20px;">
								<li>Response is received within %d seconds</li>
								<li>HTTP status code equals %d</li>
								<li>No connection errors occur</li>
							</ul>
						</div>
					</div>
				`, method, url, int(expectedStatus), int(timeout), formatSchedule(action.Schedule),
					method, url, int(timeout), int(expectedStatus))
			}
		}

	case "ControlAction":
		actionTypeDesc = "Container Control"
		if action.Instrument != nil {
			operation, _ := action.Instrument["action"].(string)
			containerID, _ := action.Instrument["containerId"].(string)
			timeout, _ := action.Instrument["timeout"].(float64)

			if timeout == 0 {
				timeout = 30
			}

			operationDesc := operation
			switch operation {
			case "restart":
				operationDesc = "Restart Container"
			case "stop":
				operationDesc = "Stop Container"
			case "start":
				operationDesc = "Start Container"
			case "pause":
				operationDesc = "Pause Container"
			case "unpause":
				operationDesc = "Unpause Container"
			}

			detailsHTML = fmt.Sprintf(`
				<div style="margin-top: 12px; padding: 12px; background: rgba(0,0,0,0.05); border-radius: 4px; border-left: 3px solid #3b82f6;">
					<strong style="display: block; margin-bottom: 8px;">⚙️ Execution Details:</strong>
					<div style="margin-left: 12px;">
						<div style="margin-bottom: 4px;">• <strong>Operation:</strong> <span style="text-transform: uppercase; font-weight: 600;">%s</span></div>
						<div style="margin-bottom: 4px;">• <strong>Target Container:</strong> <code>%s</code></div>
						<div style="margin-bottom: 4px;">• <strong>Timeout:</strong> %d seconds</div>
						<div style="margin-bottom: 4px;">• <strong>Schedule:</strong> %s</div>
					</div>
				</div>
				<div style="margin-top: 12px; padding: 12px; background: rgba(59, 130, 246, 0.1); border-radius: 4px;">
					<strong>✓ Success Criteria:</strong>
					<div style="margin-top: 6px; margin-left: 12px;">
						The agent will execute <strong>%s</strong> operation on container <code>%s</code> and verify:
						<ul style="margin: 6px 0 0 20px;">
							<li>Container operation completes within %d seconds</li>
							<li>Container transitions to expected state</li>
							<li>No Docker API errors occur</li>
						</ul>
					</div>
				</div>
			`, operation, containerID, int(timeout), formatSchedule(action.Schedule),
				operationDesc, containerID, int(timeout))
		}

	case "TransferAction":
		actionTypeDesc = "Log Collection"
		if action.Instrument != nil {
			actionName, _ := action.Instrument["action"].(string)
			containerID, _ := action.Instrument["containerId"].(string)
			lines, _ := action.Instrument["lines"].(float64)
			since, _ := action.Instrument["since"].(string)
			destination, _ := action.Instrument["destination"].(string)

			if lines == 0 {
				lines = 1000
			}
			if since == "" {
				since = "1h"
			}

			detailsHTML = fmt.Sprintf(`
				<div style="margin-top: 12px; padding: 12px; background: rgba(0,0,0,0.05); border-radius: 4px; border-left: 3px solid #8b5cf6;">
					<strong style="display: block; margin-bottom: 8px;">📦 Execution Details:</strong>
					<div style="margin-left: 12px;">
						<div style="margin-bottom: 4px;">• <strong>Operation:</strong> %s</div>
						<div style="margin-bottom: 4px;">• <strong>Source Container:</strong> <code>%s</code></div>
						<div style="margin-bottom: 4px;">• <strong>Lines to Collect:</strong> %d</div>
						<div style="margin-bottom: 4px;">• <strong>Time Range:</strong> Last %s</div>
						<div style="margin-bottom: 4px;">• <strong>Destination:</strong> <code>%s</code></div>
						<div style="margin-bottom: 4px;">• <strong>Schedule:</strong> %s</div>
					</div>
				</div>
				<div style="margin-top: 12px; padding: 12px; background: rgba(139, 92, 246, 0.1); border-radius: 4px;">
					<strong>✓ Success Criteria:</strong>
					<div style="margin-top: 6px; margin-left: 12px;">
						The agent will collect logs from container <code>%s</code> and verify:
						<ul style="margin: 6px 0 0 20px;">
							<li>Logs are successfully retrieved (up to %d lines)</li>
							<li>Logs are written to destination: <code>%s</code></li>
							<li>No container access errors occur</li>
							<li>Transfer completes successfully</li>
						</ul>
					</div>
				</div>
			`, actionName, containerID, int(lines), since, destination, formatSchedule(action.Schedule),
				containerID, int(lines), destination)
		}

	case "CreateAction":
		actionTypeDesc = "Create Action"
		detailsHTML = fmt.Sprintf(`
			<div style="margin-top: 12px; padding: 12px; background: rgba(0,0,0,0.05); border-radius: 4px; border-left: 3px solid #8b5cf6;">
				<strong style="display: block; margin-bottom: 8px;">📋 Execution Details:</strong>
				<div style="margin-left: 12px;">
					<div style="margin-bottom: 4px;">• <strong>Schedule:</strong> %s</div>
				</div>
			</div>
		`, formatSchedule(action.Schedule))

	default:
		detailsHTML = fmt.Sprintf(`
			<div style="margin-top: 12px; padding: 12px; background: rgba(0,0,0,0.05); border-radius: 4px;">
				<strong style="display: block; margin-bottom: 8px;">📋 Execution Details:</strong>
				<div style="margin-left: 12px;">
					<div style="margin-bottom: 4px;">• <strong>Schedule:</strong> %s</div>
				</div>
			</div>
		`, formatSchedule(action.Schedule))
	}

	// Add description if present
	descriptionHTML := ""
	if action.Description != "" {
		descriptionHTML = fmt.Sprintf(`
			<div style="margin-top: 8px; padding: 8px 12px; background: rgba(0,0,0,0.03); border-radius: 4px; font-style: italic; color: #666;">
				"%s"
			</div>
		`, action.Description)
	}

	// Add target object info if present
	targetHTML := ""
	if action.Object != nil && action.Object.ID != "" {
		targetHTML = fmt.Sprintf(`
			<div style="margin-top: 8px;">
				<strong>🎯 Target:</strong> <code>%s</code>
			</div>
		`, action.Object.ID)
	}

	// Return HTML for inline notification with detailed info
	html := fmt.Sprintf(`
		<div class="alert alert-success" style="position: relative; padding-right: 3rem;">
			<button onclick="document.getElementById('execution-response').innerHTML=''"
				style="position: absolute; top: 10px; right: 10px; background: none; border: none; font-size: 1.5em; cursor: pointer; color: inherit; opacity: 0.7; line-height: 1;"
				onmouseover="this.style.opacity='1'"
				onmouseout="this.style.opacity='0.7'"
			>&times;</button>
			<strong style="font-size: 1.1em;">✓ Task Created for Immediate Execution</strong>
			<div style="margin-top: 12px; display: grid; grid-template-columns: auto 1fr; gap: 8px 16px; align-items: start;">
				<strong>Action:</strong>
				<span>%s</span>

				<strong>Type:</strong>
				<span>%s</span>

				<strong>Agent:</strong>
				<span>%s</span>

				<strong>Task ID:</strong>
				<code style="background: rgba(0,0,0,0.1); padding: 2px 6px; border-radius: 3px; font-size: 0.9em;">%s</code>
			</div>
			%s
			%s
			%s
			<div style="margin-top: 12px; padding: 8px 12px; background: rgba(0,0,0,0.03); border-radius: 4px; font-size: 0.9em; color: #666;">
				<strong>ℹ️ Next Steps:</strong> The agent will pick up this task from the queue and execute it. Check the task history below for execution results.
			</div>
		</div>
	`, action.Name, actionTypeDesc, action.Agent, taskID, descriptionHTML, targetHTML, detailsHTML)

	return c.HTML(http.StatusOK, html)
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

	// Check if this is an HTMX request
	isHTMX := c.Request().Header.Get("HX-Request") == "true"

	if isHTMX {
		// For HTMX requests, use HX-Redirect to navigate to actions page
		c.Response().Header().Set("HX-Redirect", "/web/actions")
		return c.NoContent(http.StatusOK)
	}

	// For non-HTMX requests, do a regular redirect
	return c.Redirect(http.StatusSeeOther, "/web/actions")
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

// formatSchedule formats schedule information for display
func formatSchedule(schedule *models.Schedule) string {
	if schedule == nil {
		return "Not scheduled"
	}

	frequency := schedule.RepeatFrequency
	if frequency == "" {
		return "Not scheduled"
	}

	// Format ISO 8601 durations to human readable
	humanReadable := frequency
	switch frequency {
	case "PT1M":
		humanReadable = "Every minute"
	case "PT2M":
		humanReadable = "Every 2 minutes"
	case "PT5M":
		humanReadable = "Every 5 minutes"
	case "PT10M":
		humanReadable = "Every 10 minutes"
	case "PT15M":
		humanReadable = "Every 15 minutes"
	case "PT30M":
		humanReadable = "Every 30 minutes"
	case "PT1H":
		humanReadable = "Every hour"
	case "PT2H":
		humanReadable = "Every 2 hours"
	case "PT6H":
		humanReadable = "Every 6 hours"
	case "PT12H":
		humanReadable = "Every 12 hours"
	case "P1D":
		humanReadable = "Daily"
	case "P7D":
		humanReadable = "Weekly"
	case "P1M":
		humanReadable = "Monthly"
	}

	// Add timezone if not UTC
	if schedule.ScheduleTimezone != "" && schedule.ScheduleTimezone != "UTC" {
		humanReadable += fmt.Sprintf(" (%s)", schedule.ScheduleTimezone)
	}

	return humanReadable
}
