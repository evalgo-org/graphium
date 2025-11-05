package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"evalgo.org/graphium/models"
)

// @Summary Get agent tasks
// @Description Get pending tasks for a specific agent
// @Tags Agent Tasks
// @Accept json
// @Produce json
// @Param id path string true "Agent ID"
// @Param status query string false "Filter by status (pending, assigned, running, completed, failed)"
// @Param limit query int false "Maximum number of tasks to return (default: 10)"
// @Success 200 {array} models.AgentTask "List of tasks"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse
// @Router /agents/{id}/tasks [get]
func (s *Server) getAgentTasks(c echo.Context) error {
	agentID := c.Param("id")
	if agentID == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "agent ID is required",
		})
	}

	// Get status filter from query params
	status := c.QueryParam("status")

	// Get tasks for agent
	var tasks []*models.AgentTask
	var err error

	if status == "" {
		// Get pending tasks by default (most common use case)
		tasks, err = s.storage.GetPendingTasksForAgent(agentID)
	} else {
		tasks, err = s.storage.GetTasksByAgent(agentID, status)
	}

	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to get agent tasks",
			Details: err.Error(),
		})
	}

	// Apply limit if specified
	if limitStr := c.QueryParam("limit"); limitStr != "" {
		var limit int
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err == nil && limit > 0 && limit < len(tasks) {
			tasks = tasks[:limit]
		}
	}

	return c.JSON(http.StatusOK, tasks)
}

// @Summary Update task status
// @Description Update the status of a task (called by agents)
// @Tags Agent Tasks
// @Accept json
// @Produce json
// @Param id path string true "Task ID"
// @Param body body TaskStatusUpdate true "Task status update"
// @Success 200 {object} models.AgentTask "Updated task"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 404 {object} ErrorResponse "Task not found"
// @Failure 500 {object} ErrorResponse
// @Router /tasks/{id}/status [put]
func (s *Server) updateTaskStatus(c echo.Context) error {
	taskID := c.Param("id")
	if taskID == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "task ID is required",
		})
	}

	// Parse request body
	var update TaskStatusUpdate
	if err := c.Bind(&update); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid request body",
			Details: err.Error(),
		})
	}

	// Validate status
	validStatuses := map[string]bool{
		"pending":   true,
		"assigned":  true,
		"running":   true,
		"completed": true,
		"failed":    true,
		"cancelled": true,
	}

	if !validStatuses[update.Status] {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid status",
			Details: "status must be one of: pending, assigned, running, completed, failed, cancelled",
		})
	}

	// Get task to verify it exists and get current state
	task, err := s.storage.GetTask(taskID)
	if err != nil {
		return c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "task not found",
			Details: err.Error(),
		})
	}

	// Update task status based on the new status
	now := time.Now()

	switch update.Status {
	case "assigned":
		if task.AssignedAt == nil {
			task.AssignedAt = &now
		}

	case "running":
		if task.StartedAt == nil {
			task.StartedAt = &now
		}

	case "completed":
		if task.CompletedAt == nil {
			task.CompletedAt = &now
		}
		// Store result if provided
		if update.Result != nil {
			if err := task.SetResult(update.Result); err != nil {
				return c.JSON(http.StatusInternalServerError, ErrorResponse{
					Error:   "failed to set task result",
					Details: err.Error(),
				})
			}
		}

	case "failed":
		if task.CompletedAt == nil {
			task.CompletedAt = &now
		}
		task.ErrorMsg = update.Error
		task.RetryCount++

	case "cancelled":
		if task.CompletedAt == nil {
			task.CompletedAt = &now
		}
		task.ErrorMsg = update.Error
	}

	task.Status = update.Status

	// Update task in database
	if err := s.storage.UpdateTask(task); err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to update task",
			Details: err.Error(),
		})
	}

	// Broadcast WebSocket event for real-time updates
	s.BroadcastGraphEvent("task_updated", map[string]interface{}{
		"taskId":  task.ID,
		"status":  task.Status,
		"agentId": task.AgentID,
		"stackId": task.StackID,
	})

	return c.JSON(http.StatusOK, task)
}

// @Summary Get task details
// @Description Get details of a specific task
// @Tags Agent Tasks
// @Accept json
// @Produce json
// @Param id path string true "Task ID"
// @Success 200 {object} models.AgentTask "Task details"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse "Task not found"
// @Failure 500 {object} ErrorResponse
// @Router /tasks/{id} [get]
func (s *Server) getTask(c echo.Context) error {
	taskID := c.Param("id")
	if taskID == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "task ID is required",
		})
	}

	task, err := s.storage.GetTask(taskID)
	if err != nil {
		return c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "task not found",
			Details: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, task)
}

// @Summary List all tasks
// @Description List all tasks with optional filters
// @Tags Agent Tasks
// @Accept json
// @Produce json
// @Param status query string false "Filter by status"
// @Param stackId query string false "Filter by stack ID"
// @Param agentId query string false "Filter by agent ID"
// @Param containerId query string false "Filter by container ID"
// @Success 200 {array} models.AgentTask "List of tasks"
// @Failure 500 {object} ErrorResponse
// @Router /tasks [get]
func (s *Server) listTasks(c echo.Context) error {
	// Build filters from query params
	filters := make(map[string]interface{})

	if status := c.QueryParam("status"); status != "" {
		filters["status"] = status
	}
	if stackID := c.QueryParam("stackId"); stackID != "" {
		filters["stackId"] = stackID
	}
	if agentID := c.QueryParam("agentId"); agentID != "" {
		filters["agentId"] = agentID
	}
	if containerID := c.QueryParam("containerId"); containerID != "" {
		filters["containerId"] = containerID
	}

	tasks, err := s.storage.ListTasks(filters)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to list tasks",
			Details: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, tasks)
}

// @Summary Get task statistics
// @Description Get statistics about tasks (total, by status, etc.)
// @Tags Agent Tasks
// @Accept json
// @Produce json
// @Success 200 {object} map[string]int "Task statistics"
// @Failure 500 {object} ErrorResponse
// @Router /tasks/stats [get]
func (s *Server) getTaskStatistics(c echo.Context) error {
	stats, err := s.storage.GetTaskStatistics()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to get task statistics",
			Details: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, stats)
}

// @Summary Retry a failed task
// @Description Create a new task based on a failed task
// @Tags Agent Tasks
// @Accept json
// @Produce json
// @Param id path string true "Task ID"
// @Success 200 {object} models.AgentTask "New retry task"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse "Task not found"
// @Failure 500 {object} ErrorResponse
// @Router /tasks/{id}/retry [post]
func (s *Server) retryTask(c echo.Context) error {
	taskID := c.Param("id")
	if taskID == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "task ID is required",
		})
	}

	newTask, err := s.storage.RetryTask(taskID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "failed to retry task",
			Details: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, newTask)
}

// @Summary Cancel a task
// @Description Cancel a pending or running task
// @Tags Agent Tasks
// @Accept json
// @Produce json
// @Param id path string true "Task ID"
// @Success 200 {object} models.AgentTask "Cancelled task"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse "Task not found"
// @Failure 500 {object} ErrorResponse
// @Router /tasks/{id}/cancel [post]
func (s *Server) cancelTask(c echo.Context) error {
	taskID := c.Param("id")
	if taskID == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "task ID is required",
		})
	}

	if err := s.storage.CancelTask(taskID); err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to cancel task",
			Details: err.Error(),
		})
	}

	task, err := s.storage.GetTask(taskID)
	if err != nil {
		return c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "task not found after cancel",
			Details: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, task)
}

// @Summary Create a new task
// @Description Create a new agent task (admin/write access required)
// @Tags Agent Tasks
// @Accept json
// @Produce json
// @Param body body models.AgentTask true "Task to create"
// @Success 201 {object} models.AgentTask "Created task"
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /tasks [post]
func (s *Server) createTask(c echo.Context) error {
	var task models.AgentTask
	if err := c.Bind(&task); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid request body",
			Details: err.Error(),
		})
	}

	// Normalize: populate semantic fields from legacy fields (or vice versa)
	// This ensures backward compatibility regardless of which format was used
	task.Normalize()

	// Set defaults
	if task.Type == "" {
		task.Type = "AgentTask"
	}
	if task.Status == "" {
		task.Status = "pending"
	}
	if task.Priority == 0 {
		task.Priority = 5
	}

	// Create task in database
	if err := s.storage.CreateTask(&task); err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to create task",
			Details: err.Error(),
		})
	}

	// Broadcast WebSocket event
	s.BroadcastGraphEvent("task_created", map[string]interface{}{
		"taskId":   task.ID,
		"taskType": task.TaskType,
		"agentId":  task.AgentID,
		"stackId":  task.StackID,
	})

	return c.JSON(http.StatusCreated, task)
}

// TaskStatusUpdate represents a task status update request.
type TaskStatusUpdate struct {
	Status string             `json:"status"`
	Error  string             `json:"error,omitempty"`
	Result *models.TaskResult `json:"result,omitempty"`
}
