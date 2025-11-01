package api

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"evalgo.org/graphium/models"
)

// CreateScheduledAction handles POST /api/v1/actions
func (s *Server) CreateScheduledAction(c echo.Context) error {
	var action models.ScheduledAction

	if err := c.Bind(&action); err != nil {
		return BadRequestError("Invalid request body", err.Error())
	}

	// Validate required fields
	if action.Name == "" {
		return BadRequestError("Action name is required", "")
	}
	if action.Agent == "" {
		return BadRequestError("Agent (host ID) is required", "")
	}
	if action.Schedule == nil {
		return BadRequestError("Schedule is required", "")
	}
	if action.Schedule.RepeatFrequency == "" {
		return BadRequestError("Schedule repeat frequency is required", "")
	}

	// Set defaults
	now := time.Now()
	action.CreatedAt = now
	action.UpdatedAt = now

	if action.ActionStatus == "" {
		action.ActionStatus = models.ActionStatusPotential
	}
	if action.Enabled {
		// Default to enabled if not specified
		action.Enabled = true
	}

	// Create in database
	if err := s.storage.CreateScheduledAction(&action); err != nil {
		return InternalError("Failed to create scheduled action", err.Error())
	}

	return c.JSON(http.StatusCreated, action)
}

// GetScheduledAction handles GET /api/v1/actions/:id
func (s *Server) GetScheduledAction(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return BadRequestError("Action ID is required", "")
	}

	action, err := s.storage.GetScheduledAction(id)
	if err != nil {
		return NotFoundError("Scheduled action", id)
	}

	return c.JSON(http.StatusOK, action)
}

// ListScheduledActions handles GET /api/v1/actions
func (s *Server) ListScheduledActions(c echo.Context) error {
	// Parse query parameters for filtering
	filters := make(map[string]interface{})

	if actionType := c.QueryParam("type"); actionType != "" {
		filters["@type"] = actionType
	}
	if agent := c.QueryParam("agent"); agent != "" {
		filters["agent"] = agent
	}
	if status := c.QueryParam("status"); status != "" {
		filters["actionStatus"] = status
	}
	if enabled := c.QueryParam("enabled"); enabled != "" {
		filters["enabled"] = enabled == "true"
	}

	actions, err := s.storage.ListScheduledActions(filters)
	if err != nil {
		return InternalError("Failed to list scheduled actions", err.Error())
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"count":   len(actions),
		"actions": actions,
	})
}

// UpdateScheduledAction handles PUT /api/v1/actions/:id
func (s *Server) UpdateScheduledAction(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return BadRequestError("Action ID is required", "")
	}

	// Get existing action
	existing, err := s.storage.GetScheduledAction(id)
	if err != nil {
		return NotFoundError("Scheduled action", id)
	}

	var updates models.ScheduledAction
	if err := c.Bind(&updates); err != nil {
		return BadRequestError("Invalid request body", err.Error())
	}

	// Preserve system fields
	updates.ID = existing.ID
	updates.Rev = existing.Rev
	updates.CreatedAt = existing.CreatedAt
	updates.UpdatedAt = time.Now()

	// Set defaults for required fields if not provided
	if updates.Context == "" {
		updates.Context = existing.Context
	}
	if updates.Type == "" {
		updates.Type = existing.Type
	}

	// Update in database
	if err := s.storage.UpdateScheduledAction(&updates); err != nil {
		return InternalError("Failed to update scheduled action", err.Error())
	}

	return c.JSON(http.StatusOK, updates)
}

// DeleteScheduledAction handles DELETE /api/v1/actions/:id
func (s *Server) DeleteScheduledAction(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return BadRequestError("Action ID is required", "")
	}

	// Get existing action to get revision
	action, err := s.storage.GetScheduledAction(id)
	if err != nil {
		return NotFoundError("Scheduled action", id)
	}

	// Delete
	if err := s.storage.DeleteScheduledAction(id, action.Rev); err != nil {
		return InternalError("Failed to delete scheduled action", err.Error())
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Scheduled action deleted",
		"id":      id,
	})
}

// ExecuteScheduledAction handles POST /api/v1/actions/:id/execute
// Triggers an immediate execution of the scheduled action
func (s *Server) ExecuteScheduledAction(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return BadRequestError("Action ID is required", "")
	}

	// Get the action
	action, err := s.storage.GetScheduledAction(id)
	if err != nil {
		return NotFoundError("Scheduled action", id)
	}

	// Create a task from this action
	task := &models.AgentTask{
		Type:        "AgentTask",
		ID:          models.GenerateID("task"),
		HostID:      action.Agent,
		AgentID:     action.Agent,
		Status:      models.TaskStatusPending,
		ScheduledBy: action.ID,
		CreatedAt:   time.Now(),
	}

	// Check if this is a composite action (workflow)
	isCompositeAction := false
	if action.Instrument != nil {
		if compositeVal, ok := action.Instrument["compositeAction"]; ok {
			isCompositeAction, _ = compositeVal.(bool)
		}
	}

	// Set task type based on whether it's a composite action (workflow)
	if isCompositeAction {
		task.TaskType = "workflow"
	} else {
		// Map action type to task type
		switch action.Type {
		case models.ActionTypeCheck:
			task.TaskType = "check"
		case models.ActionTypeControl:
			task.TaskType = "control"
		case models.ActionTypeCreate:
			task.TaskType = "create"
		case models.ActionTypeUpdate:
			task.TaskType = "update"
		case models.ActionTypeTransfer:
			task.TaskType = "transfer"
		default:
			task.TaskType = "action"
		}
	}

	// Build payload from action instrument and object
	payload := make(map[string]interface{})

	// Copy parameters from action instrument
	if action.Instrument != nil {
		for k, v := range action.Instrument {
			payload[k] = v
		}
	}

	// Add object information to payload
	if action.Object != nil {
		payload["object"] = action.Object
	}

	// Set the payload
	if len(payload) > 0 {
		if err := task.SetPayload(payload); err != nil {
			return InternalError("Failed to set task payload", err.Error())
		}
	}

	// Create the task
	if err := s.storage.CreateTask(task); err != nil {
		return InternalError("Failed to create task", err.Error())
	}

	// Update action status
	action.MarkStarted()
	if err := s.storage.UpdateScheduledAction(action); err != nil {
		// Log but don't fail - task was created successfully
		s.debugLog("Warning: Failed to update action status: %v\n", err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Task created for immediate execution",
		"taskId":  task.ID,
		"actionId": action.ID,
	})
}

// GetScheduledActionHistory handles GET /api/v1/actions/:id/history
// Returns the execution history (tasks) for this action
func (s *Server) GetScheduledActionHistory(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return BadRequestError("Action ID is required", "")
	}

	// Verify action exists
	if _, err := s.storage.GetScheduledAction(id); err != nil {
		return NotFoundError("Scheduled action", id)
	}

	// Get tasks created by this action
	tasks, err := s.storage.GetTasksByScheduledAction(id)
	if err != nil {
		return InternalError("Failed to get action history", err.Error())
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"actionId": id,
		"count":    len(tasks),
		"tasks":    tasks,
	})
}

