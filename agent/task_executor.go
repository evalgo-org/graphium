package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"evalgo.org/graphium/models"
)

// TaskExecutor polls the server for tasks and executes them.
type TaskExecutor struct {
	agent        *Agent
	deployer     *AgentDeployer
	pollInterval time.Duration
	running      bool
	stopChan     chan struct{}
}

// NewTaskExecutor creates a new task executor.
func NewTaskExecutor(agent *Agent, pollInterval time.Duration) *TaskExecutor {
	if pollInterval == 0 {
		pollInterval = 5 * time.Second // Default: poll every 5 seconds
	}

	return &TaskExecutor{
		agent:        agent,
		deployer:     NewDeployer(agent.docker, agent.hostID, agent.hostID),
		pollInterval: pollInterval,
		stopChan:     make(chan struct{}),
	}
}

// Start begins polling for tasks and executing them.
func (e *TaskExecutor) Start(ctx context.Context) error {
	if e.running {
		return fmt.Errorf("task executor is already running")
	}

	e.running = true
	log.Printf("Task executor started (polling every %v)", e.pollInterval)

	ticker := time.NewTicker(e.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := e.pollAndExecuteTasks(ctx); err != nil {
				log.Printf("Error polling/executing tasks: %v", err)
			}

		case <-e.stopChan:
			log.Println("Task executor stopped")
			e.running = false
			return nil

		case <-ctx.Done():
			log.Println("Task executor context cancelled")
			e.running = false
			return ctx.Err()
		}
	}
}

// Stop stops the task executor.
func (e *TaskExecutor) Stop() {
	if e.running {
		close(e.stopChan)
	}
}

// pollAndExecuteTasks fetches pending tasks and executes them.
func (e *TaskExecutor) pollAndExecuteTasks(ctx context.Context) error {
	// Fetch pending tasks from server
	tasks, err := e.fetchPendingTasks()
	if err != nil {
		return fmt.Errorf("failed to fetch tasks: %w", err)
	}

	if len(tasks) == 0 {
		return nil // No tasks to execute
	}

	log.Printf("Fetched %d pending task(s)", len(tasks))

	// Execute each task
	for _, task := range tasks {
		if err := e.executeTask(ctx, task); err != nil {
			log.Printf("Failed to execute task %s: %v", task.ID, err)
			// Report failure
			e.reportTaskStatus(task.ID, "failed", err.Error(), nil)
		}
	}

	return nil
}

// fetchPendingTasks fetches pending tasks for this agent from the server.
func (e *TaskExecutor) fetchPendingTasks() ([]*models.AgentTask, error) {
	// Build API URL
	url := fmt.Sprintf("%s/api/v1/agents/%s/tasks?status=pending&limit=10", e.agent.apiURL, e.agent.hostID)

	// Create request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Add auth token
	if e.agent.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+e.agent.authToken)
	}

	// Execute request
	resp, err := e.agent.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var tasks []*models.AgentTask
	if err := json.NewDecoder(resp.Body).Decode(&tasks); err != nil {
		return nil, err
	}

	return tasks, nil
}

// executeTask executes a single task.
func (e *TaskExecutor) executeTask(ctx context.Context, task *models.AgentTask) error {
	log.Printf("Executing task %s (type: %s)", task.ID, task.TaskType)

	// Mark task as running
	if err := e.reportTaskStatus(task.ID, "running", "", nil); err != nil {
		log.Printf("Warning: Failed to mark task as running: %v", err)
	}

	// Execute based on task type
	var result *models.TaskResult
	var err error

	switch task.TaskType {
	case "deploy":
		result, err = e.executeDeploy(ctx, task)

	case "delete":
		result, err = e.executeDelete(ctx, task)

	case "stop":
		result, err = e.executeStop(ctx, task)

	case "start":
		result, err = e.executeStart(ctx, task)

	case "restart":
		result, err = e.executeRestart(ctx, task)

	default:
		err = fmt.Errorf("unsupported task type: %s", task.TaskType)
	}

	// Report result
	if err != nil {
		log.Printf("Task %s failed: %v", task.ID, err)
		return e.reportTaskStatus(task.ID, "failed", err.Error(), nil)
	}

	log.Printf("Task %s completed successfully", task.ID)
	return e.reportTaskStatus(task.ID, "completed", "", result)
}

// executeDeploy executes a deploy task.
func (e *TaskExecutor) executeDeploy(ctx context.Context, task *models.AgentTask) (*models.TaskResult, error) {
	var payload models.DeployContainerPayload
	if err := task.GetPayloadAs(&payload); err != nil {
		return nil, fmt.Errorf("invalid deploy payload: %w", err)
	}

	return e.deployer.DeployContainer(ctx, &payload)
}

// executeDelete executes a delete task.
func (e *TaskExecutor) executeDelete(ctx context.Context, task *models.AgentTask) (*models.TaskResult, error) {
	var payload models.DeleteContainerPayload
	if err := task.GetPayloadAs(&payload); err != nil {
		return nil, fmt.Errorf("invalid delete payload: %w", err)
	}

	return e.deployer.DeleteContainer(ctx, &payload)
}

// executeStop executes a stop task.
func (e *TaskExecutor) executeStop(ctx context.Context, task *models.AgentTask) (*models.TaskResult, error) {
	var payload models.ControlContainerPayload
	if err := task.GetPayloadAs(&payload); err != nil {
		return nil, fmt.Errorf("invalid stop payload: %w", err)
	}

	return e.deployer.StopContainer(ctx, &payload)
}

// executeStart executes a start task.
func (e *TaskExecutor) executeStart(ctx context.Context, task *models.AgentTask) (*models.TaskResult, error) {
	var payload models.ControlContainerPayload
	if err := task.GetPayloadAs(&payload); err != nil {
		return nil, fmt.Errorf("invalid start payload: %w", err)
	}

	return e.deployer.StartContainer(ctx, &payload)
}

// executeRestart executes a restart task.
func (e *TaskExecutor) executeRestart(ctx context.Context, task *models.AgentTask) (*models.TaskResult, error) {
	var payload models.ControlContainerPayload
	if err := task.GetPayloadAs(&payload); err != nil {
		return nil, fmt.Errorf("invalid restart payload: %w", err)
	}

	return e.deployer.RestartContainer(ctx, &payload)
}

// reportTaskStatus reports task status back to the server.
func (e *TaskExecutor) reportTaskStatus(taskID string, status string, errorMsg string, result *models.TaskResult) error {
	// Build API URL
	url := fmt.Sprintf("%s/api/v1/tasks/%s/status", e.agent.apiURL, taskID)

	// Create status update
	update := map[string]interface{}{
		"status": status,
	}

	if errorMsg != "" {
		update["error"] = errorMsg
	}

	if result != nil {
		update["result"] = result
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(update)
	if err != nil {
		return err
	}

	// Create request
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	// Add auth token
	if e.agent.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+e.agent.authToken)
	}

	// Execute request
	resp, err := e.agent.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
