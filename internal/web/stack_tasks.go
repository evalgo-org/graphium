package web

import (
	"fmt"
	"time"

	"evalgo.org/graphium/models"
)

// CreateDeploymentTasksForStack creates agent tasks for deploying all containers in a stack.
// Returns the list of created tasks.
func (h *Handler) CreateDeploymentTasksForStack(stackID string, containerSpecs []models.ContainerSpec, username string) ([]*models.AgentTask, error) {
	tasks := make([]*models.AgentTask, 0, len(containerSpecs))

	for i, spec := range containerSpecs {
		// Determine target host
		var hostID string
		if spec.LocatedInHost != nil {
			hostID = spec.LocatedInHost.ID
		} else {
			return nil, fmt.Errorf("container %s has no host assignment", spec.Name)
		}

		// Remove host: prefix if present
		if len(hostID) > 5 && hostID[:5] == "host:" {
			hostID = hostID[5:]
		}

		// Create deployment payload
		payload := models.DeployContainerPayload{
			ContainerSpec: spec,
			PullPolicy:    "if-not-present",
			Labels: map[string]string{
				"graphium.stack":   stackID,
				"graphium.managed": "true",
			},
		}

		// Create task
		task := &models.AgentTask{
			ID:             fmt.Sprintf("task-deploy-%s-%s-%d", stackID, spec.Name, time.Now().Unix()),
			Type:           "AgentTask",
			TaskType:       "deploy",
			Status:         "pending",
			AgentID:        hostID,
			HostID:         hostID,
			StackID:        stackID,
			Priority:       5,
			CreatedAt:      time.Now(),
			CreatedBy:      username,
			TimeoutSeconds: 300,
			MaxRetries:     3,
		}

		// Set payload
		if err := task.SetPayload(&payload); err != nil {
			return nil, fmt.Errorf("failed to set payload for container %s: %w", spec.Name, err)
		}

		// Handle dependencies
		if len(spec.DependsOn) > 0 {
			// Convert dependency container names to task IDs
			dependencyTaskIDs := make([]string, 0, len(spec.DependsOn))
			for _, depName := range spec.DependsOn {
				// Find the task for this dependency
				for j := 0; j < i; j++ {
					if containerSpecs[j].Name == depName {
						depTaskID := fmt.Sprintf("task-deploy-%s-%s-%d", stackID, depName, time.Now().Unix())
						dependencyTaskIDs = append(dependencyTaskIDs, depTaskID)
						break
					}
				}
			}
			task.DependsOn = dependencyTaskIDs
		}

		// Save task to database
		if err := h.storage.CreateTask(task); err != nil {
			return nil, fmt.Errorf("failed to create task for container %s: %w", spec.Name, err)
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}

// CreateDeletionTasksForStack creates agent tasks for deleting all containers in a stack.
// Returns the list of created tasks.
func (h *Handler) CreateDeletionTasksForStack(stackID string, deploymentState *models.DeploymentState, username string) ([]*models.AgentTask, error) {
	if deploymentState == nil || deploymentState.Placements == nil {
		return nil, fmt.Errorf("deployment state is nil or has no placements")
	}

	tasks := make([]*models.AgentTask, 0, len(deploymentState.Placements))

	for containerName, placement := range deploymentState.Placements {
		if placement == nil {
			continue
		}

		// Create deletion payload
		payload := models.DeleteContainerPayload{
			ContainerID:   placement.ContainerID,
			ContainerName: containerName,
			Force:         false,
			RemoveVolumes: false,
			StopTimeout:   10,
		}

		// Create task
		task := &models.AgentTask{
			ID:             fmt.Sprintf("task-delete-%s-%s-%d", stackID, containerName, time.Now().Unix()),
			Type:           "AgentTask",
			TaskType:       "delete",
			Status:         "pending",
			AgentID:        placement.HostID,
			HostID:         placement.HostID,
			StackID:        stackID,
			ContainerID:    placement.ContainerID,
			Priority:       7, // Higher priority for deletion
			CreatedAt:      time.Now(),
			CreatedBy:      username,
			TimeoutSeconds: 120,
			MaxRetries:     3,
		}

		// Set payload
		if err := task.SetPayload(&payload); err != nil {
			return nil, fmt.Errorf("failed to set payload for container %s: %w", containerName, err)
		}

		// Save task to database
		if err := h.storage.CreateTask(task); err != nil {
			return nil, fmt.Errorf("failed to create deletion task for container %s: %w", containerName, err)
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}

// CreateStopTasksForStack creates agent tasks for stopping all containers in a stack.
func (h *Handler) CreateStopTasksForStack(stackID string, deploymentState *models.DeploymentState, username string) ([]*models.AgentTask, error) {
	if deploymentState == nil || deploymentState.Placements == nil {
		return nil, fmt.Errorf("deployment state is nil or has no placements")
	}

	tasks := make([]*models.AgentTask, 0, len(deploymentState.Placements))

	for containerName, placement := range deploymentState.Placements {
		if placement == nil {
			continue
		}

		// Create control payload
		payload := models.ControlContainerPayload{
			ContainerID:   placement.ContainerID,
			ContainerName: containerName,
			Timeout:       10,
		}

		// Create task
		task := &models.AgentTask{
			ID:             fmt.Sprintf("task-stop-%s-%s-%d", stackID, containerName, time.Now().Unix()),
			Type:           "AgentTask",
			TaskType:       "stop",
			Status:         "pending",
			AgentID:        placement.HostID,
			HostID:         placement.HostID,
			StackID:        stackID,
			ContainerID:    placement.ContainerID,
			Priority:       6,
			CreatedAt:      time.Now(),
			CreatedBy:      username,
			TimeoutSeconds: 60,
			MaxRetries:     2,
		}

		// Set payload
		if err := task.SetPayload(&payload); err != nil {
			return nil, fmt.Errorf("failed to set payload for container %s: %w", containerName, err)
		}

		// Save task to database
		if err := h.storage.CreateTask(task); err != nil {
			return nil, fmt.Errorf("failed to create stop task for container %s: %w", containerName, err)
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}

// WaitForTasksCompletion waits for all tasks to reach a terminal state (completed, failed, cancelled).
// This is a simplified version - in production, you'd want to use WebSocket notifications.
func (h *Handler) WaitForTasksCompletion(tasks []*models.AgentTask, maxWaitTime time.Duration) error {
	start := time.Now()
	taskIDs := make([]string, len(tasks))
	for i, task := range tasks {
		taskIDs[i] = task.ID
	}

	for {
		if time.Since(start) > maxWaitTime {
			return fmt.Errorf("timeout waiting for tasks to complete")
		}

		allComplete := true
		for _, taskID := range taskIDs {
			task, err := h.storage.GetTask(taskID)
			if err != nil {
				return fmt.Errorf("failed to get task %s: %w", taskID, err)
			}

			if task.Status != "completed" && task.Status != "failed" && task.Status != "cancelled" {
				allComplete = false
				break
			}
		}

		if allComplete {
			return nil
		}

		time.Sleep(2 * time.Second)
	}
}

// GetDeploymentStatus checks the status of deployment tasks for a stack.
func (h *Handler) GetDeploymentStatus(stackID string) (*DeploymentStatus, error) {
	tasks, err := h.storage.GetTasksByStack(stackID)
	if err != nil {
		return nil, err
	}

	status := &DeploymentStatus{
		StackID:    stackID,
		TotalTasks: len(tasks),
		TasksByStatus: map[string]int{
			"pending":   0,
			"assigned":  0,
			"running":   0,
			"completed": 0,
			"failed":    0,
			"cancelled": 0,
		},
		Tasks: tasks,
	}

	for _, task := range tasks {
		status.TasksByStatus[task.Status]++
	}

	// Determine overall status
	if status.TasksByStatus["failed"] > 0 {
		status.OverallStatus = "failed"
	} else if status.TasksByStatus["running"] > 0 || status.TasksByStatus["pending"] > 0 || status.TasksByStatus["assigned"] > 0 {
		status.OverallStatus = "deploying"
	} else if status.TotalTasks > 0 && status.TasksByStatus["completed"] == status.TotalTasks {
		status.OverallStatus = "completed"
	} else {
		status.OverallStatus = "unknown"
	}

	// Calculate progress percentage
	if status.TotalTasks > 0 {
		status.Progress = (status.TasksByStatus["completed"] * 100) / status.TotalTasks
	}

	return status, nil
}

// DeploymentStatus represents the status of a stack deployment.
type DeploymentStatus struct {
	StackID       string
	TotalTasks    int
	OverallStatus string
	Progress      int
	TasksByStatus map[string]int
	Tasks         []*models.AgentTask
}

