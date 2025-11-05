package storage

import (
	"fmt"
	"time"

	"eve.evalgo.org/db"
	"eve.evalgo.org/semantic"

	"evalgo.org/graphium/models"
)

// CreateTask creates a new agent task in the database.
func (s *Storage) CreateTask(task *models.AgentTask) error {
	// Set JSON-LD type if not set
	if task.Type == "" {
		task.Type = "AgentTask"
	}

	// Set created timestamp if not set
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}

	// Set default priority
	if task.Priority == 0 {
		task.Priority = 5
	}

	// Set default timeout
	if task.TimeoutSeconds == 0 {
		task.TimeoutSeconds = 300 // 5 minutes
	}

	// Set default max retries
	if task.MaxRetries == 0 {
		task.MaxRetries = 3
	}

	return s.SaveDocument(task)
}

// GetTask retrieves a task by ID.
func (s *Storage) GetTask(id string) (*models.AgentTask, error) {
	var task models.AgentTask
	if err := s.GetDocument(id, &task); err != nil {
		return nil, err
	}
	return &task, nil
}

// UpdateTask updates an existing task.
func (s *Storage) UpdateTask(task *models.AgentTask) error {
	return s.SaveDocument(task)
}

// DeleteTask deletes a task by ID.
func (s *Storage) DeleteTask(id string, rev string) error {
	return s.service.DeleteDocument(id, rev)
}

// ListTasks retrieves all tasks with optional filters.
func (s *Storage) ListTasks(filters map[string]interface{}) ([]*models.AgentTask, error) {
	// Build query - task documents have @type = "AgentTask"
	qb := db.NewQueryBuilder().
		Where("@type", "$eq", "AgentTask")

	// Add filters
	for field, value := range filters {
		qb = qb.And().Where(field, "$eq", value)
	}

	query := qb.Build()

	// Execute query
	tasks, err := db.FindTyped[models.AgentTask](s.service, query)
	if err != nil {
		return nil, err
	}

	// Convert to pointer slice
	result := make([]*models.AgentTask, len(tasks))
	for i := range tasks {
		result[i] = &tasks[i]
	}

	return result, nil
}

// GetTasksByAgent retrieves tasks for a specific agent with optional status filter.
// If status is empty, returns all tasks for the agent.
func (s *Storage) GetTasksByAgent(agentID string, status string) ([]*models.AgentTask, error) {
	filters := map[string]interface{}{
		"hostId": agentID,
	}

	if status != "" {
		filters["actionStatus"] = status
	}

	return s.ListTasks(filters)
}

// GetPendingTasksForAgent retrieves pending tasks for a specific agent,
// ordered by priority (highest first) and creation time (oldest first).
func (s *Storage) GetPendingTasksForAgent(agentID string) ([]*models.AgentTask, error) {
	// Query for pending tasks
	qbPending := db.NewQueryBuilder().
		Where("@type", "$eq", "AgentTask").
		And().
		Where("hostId", "$eq", agentID).
		And().
		Where("actionStatus", "$eq", models.TaskStatusPending)

	queryPending := qbPending.Build()
	pendingTasks, err := db.FindTyped[models.AgentTask](s.service, queryPending)
	if err != nil {
		return nil, err
	}

	// Query for assigned tasks
	qbAssigned := db.NewQueryBuilder().
		Where("@type", "$eq", "AgentTask").
		And().
		Where("hostId", "$eq", agentID).
		And().
		Where("actionStatus", "$eq", models.TaskStatusAssigned)

	queryAssigned := qbAssigned.Build()
	assignedTasks, err := db.FindTyped[models.AgentTask](s.service, queryAssigned)
	if err != nil {
		return nil, err
	}

	// Combine results
	allTasks := append(pendingTasks, assignedTasks...)

	// Sort by priority (desc) then by creation time (asc)
	// Note: EVE's query builder doesn't support OrderBy, so we sort in memory
	// For production, consider using CouchDB views with sorting
	for i := 0; i < len(allTasks); i++ {
		for j := i + 1; j < len(allTasks); j++ {
			// Sort by priority descending (higher first)
			if allTasks[i].Priority < allTasks[j].Priority {
				allTasks[i], allTasks[j] = allTasks[j], allTasks[i]
			} else if allTasks[i].Priority == allTasks[j].Priority {
				// If same priority, sort by creation time ascending (older first)
				if allTasks[i].CreatedAt.After(allTasks[j].CreatedAt) {
					allTasks[i], allTasks[j] = allTasks[j], allTasks[i]
				}
			}
		}
	}

	// Convert to pointer slice
	result := make([]*models.AgentTask, len(allTasks))
	for i := range allTasks {
		result[i] = &allTasks[i]
	}

	return result, nil
}

// GetTasksByStack retrieves all tasks for a specific stack.
func (s *Storage) GetTasksByStack(stackID string) ([]*models.AgentTask, error) {
	filters := map[string]interface{}{
		"stackId": stackID,
	}
	return s.ListTasks(filters)
}

// GetTasksByStatus retrieves all tasks with a specific status.
func (s *Storage) GetTasksByStatus(status string) ([]*models.AgentTask, error) {
	filters := map[string]interface{}{
		"actionStatus": status,
	}
	return s.ListTasks(filters)
}

// GetTasksByContainer retrieves all tasks for a specific container.
func (s *Storage) GetTasksByContainer(containerID string) ([]*models.AgentTask, error) {
	filters := map[string]interface{}{
		"containerId": containerID,
	}
	return s.ListTasks(filters)
}

// UpdateTaskStatus updates the status of a task and sets timestamps.
func (s *Storage) UpdateTaskStatus(taskID string, status string, errorMsg string) error {
	task, err := s.GetTask(taskID)
	if err != nil {
		return fmt.Errorf("failed to get task: %w", err)
	}

	now := time.Now()
	task.ActionStatus = status

	switch status {
	case "PotentialActionStatus": // pending/assigned
		// Pending/assigned - no timestamp needed yet

	case "ActiveActionStatus": // running
		if task.StartTime == nil {
			task.StartTime = &now
		}

	case "CompletedActionStatus": // completed
		if task.EndTime == nil {
			task.EndTime = &now
		}

	case "FailedActionStatus": // failed/cancelled
		if task.EndTime == nil {
			task.EndTime = &now
		}
	}

	if errorMsg != "" {
		if task.Error == nil {
			task.Error = &semantic.SemanticError{
				Type: "Error",
			}
		}
		task.Error.Message = errorMsg
	}

	return s.UpdateTask(task)
}

// MarkTaskAsRunning marks a task as running and sets the start time.
func (s *Storage) MarkTaskAsRunning(taskID string) error {
	task, err := s.GetTask(taskID)
	if err != nil {
		return err
	}

	now := time.Now()
	task.ActionStatus = models.TaskStatusRunning
	task.StartTime = &now

	return s.UpdateTask(task)
}

// CompleteTask marks a task as completed and stores the result.
func (s *Storage) CompleteTask(taskID string, result *models.TaskResult) error {
	task, err := s.GetTask(taskID)
	if err != nil {
		return err
	}

	now := time.Now()
	task.ActionStatus = models.TaskStatusCompleted
	task.EndTime = &now

	if result != nil {
		if err := task.SetResult(result); err != nil {
			return fmt.Errorf("failed to set task result: %w", err)
		}
	}

	return s.UpdateTask(task)
}

// FailTask marks a task as failed with an error message.
func (s *Storage) FailTask(taskID string, errorMsg string) error {
	task, err := s.GetTask(taskID)
	if err != nil {
		return err
	}

	now := time.Now()
	task.ActionStatus = models.TaskStatusFailed
	task.EndTime = &now

	if task.Error == nil {
		task.Error = &semantic.SemanticError{
			Type: "Error",
		}
	}
	task.Error.Message = errorMsg

	// Increment retry count
	task.RetryCount++

	return s.UpdateTask(task)
}

// RetryTask creates a new task based on a failed task.
func (s *Storage) RetryTask(taskID string) (*models.AgentTask, error) {
	originalTask, err := s.GetTask(taskID)
	if err != nil {
		return nil, err
	}

	if !originalTask.CanRetry() {
		return nil, fmt.Errorf("task has exceeded max retries (%d)", originalTask.MaxRetries)
	}

	// Create new task with same parameters
	newTask := &models.AgentTask{
		ID:             fmt.Sprintf("%s-retry-%d", originalTask.ID, originalTask.RetryCount+1),
		Context:        "https://schema.org",
		Type:           originalTask.Type,
		ActionStatus:   models.TaskStatusPending,
		Agent:          originalTask.Agent,
		Object:         originalTask.Object,
		Instrument:     originalTask.Instrument,
		HostID:         originalTask.HostID,
		StackID:        originalTask.StackID,
		ContainerID:    originalTask.ContainerID,
		Priority:       originalTask.Priority,
		CreatedAt:      time.Now(),
		CreatedBy:      originalTask.CreatedBy,
		RetryCount:     originalTask.RetryCount + 1,
		MaxRetries:     originalTask.MaxRetries,
		TimeoutSeconds: originalTask.TimeoutSeconds,
		DependsOn:      originalTask.DependsOn,
		Schedule:       originalTask.Schedule,
		Properties:     originalTask.Properties,
	}

	if err := s.CreateTask(newTask); err != nil {
		return nil, err
	}

	return newTask, nil
}

// CancelTask marks a task as cancelled.
func (s *Storage) CancelTask(taskID string) error {
	return s.UpdateTaskStatus(taskID, "cancelled", "")
}

// CleanupOldTasks deletes completed/failed tasks older than the specified duration.
func (s *Storage) CleanupOldTasks(olderThan time.Duration) (int, error) {
	cutoffTime := time.Now().Add(-olderThan)

	// Query for old completed tasks
	qbCompleted := db.NewQueryBuilder().
		Where("@type", "$eq", "AgentTask").
		And().
		Where("actionStatus", "$eq", models.TaskStatusCompleted).
		And().
		Where("endTime", "$lt", cutoffTime)

	queryCompleted := qbCompleted.Build()
	completedTasks, err := db.FindTyped[models.AgentTask](s.service, queryCompleted)
	if err != nil {
		return 0, err
	}

	// Query for old failed tasks
	qbFailed := db.NewQueryBuilder().
		Where("@type", "$eq", "AgentTask").
		And().
		Where("actionStatus", "$eq", models.TaskStatusFailed).
		And().
		Where("endTime", "$lt", cutoffTime)

	queryFailed := qbFailed.Build()
	failedTasks, err := db.FindTyped[models.AgentTask](s.service, queryFailed)
	if err != nil {
		return 0, err
	}

	// Combine and delete
	allOldTasks := append(completedTasks, failedTasks...)
	deletedCount := 0
	for _, task := range allOldTasks {
		if err := s.DeleteTask(task.ID, task.Rev); err != nil {
			s.debugLog("Warning: Failed to delete old task %s: %v\n", task.ID, err)
			continue
		}
		deletedCount++
	}

	return deletedCount, nil
}

// GetExpiredTasks retrieves tasks that have exceeded their timeout.
func (s *Storage) GetExpiredTasks() ([]*models.AgentTask, error) {
	// Get all running tasks
	runningTasks, err := s.GetTasksByStatus(models.TaskStatusRunning)
	if err != nil {
		return nil, err
	}

	// Filter expired tasks
	expiredTasks := make([]*models.AgentTask, 0)
	for _, task := range runningTasks {
		if task.IsExpired() {
			expiredTasks = append(expiredTasks, task)
		}
	}

	return expiredTasks, nil
}

// GetTaskStatistics returns statistics about tasks.
func (s *Storage) GetTaskStatistics() (map[string]int, error) {
	allTasks, err := s.ListTasks(nil)
	if err != nil {
		return nil, err
	}

	stats := map[string]int{
		"total":     len(allTasks),
		"pending":   0,
		"assigned":  0,
		"running":   0,
		"completed": 0,
		"failed":    0,
		"cancelled": 0,
	}

	for _, task := range allTasks {
		// Map semantic status to simple keys
		// Note: Since TaskStatusPending and TaskStatusAssigned both equal "PotentialActionStatus",
		// we count them all as "pending" for now. Same for failed/cancelled.
		switch task.ActionStatus {
		case "PotentialActionStatus":
			stats["pending"]++
		case "ActiveActionStatus":
			stats["running"]++
		case "CompletedActionStatus":
			stats["completed"]++
		case "FailedActionStatus":
			stats["failed"]++
		}
	}

	return stats, nil
}

// GetTaskDependencies returns all tasks that the given task depends on.
func (s *Storage) GetTaskDependencies(taskID string) ([]*models.AgentTask, error) {
	task, err := s.GetTask(taskID)
	if err != nil {
		return nil, err
	}

	if len(task.DependsOn) == 0 {
		return []*models.AgentTask{}, nil
	}

	dependencies := make([]*models.AgentTask, 0, len(task.DependsOn))
	for _, depID := range task.DependsOn {
		depTask, err := s.GetTask(depID)
		if err != nil {
			s.debugLog("Warning: Failed to get dependency task %s: %v\n", depID, err)
			continue
		}
		dependencies = append(dependencies, depTask)
	}

	return dependencies, nil
}

// AreTaskDependenciesMet checks if all dependencies of a task are completed.
func (s *Storage) AreTaskDependenciesMet(taskID string) (bool, error) {
	dependencies, err := s.GetTaskDependencies(taskID)
	if err != nil {
		return false, err
	}

	for _, dep := range dependencies {
		if dep.ActionStatus != models.TaskStatusCompleted {
			return false, nil
		}
	}

	return true, nil
}

// GetTasksByScheduledAction retrieves all tasks created by a specific scheduled action
func (s *Storage) GetTasksByScheduledAction(actionID string) ([]*models.AgentTask, error) {
	return s.ListTasks(map[string]interface{}{
		"scheduledBy": actionID,
	})
}
