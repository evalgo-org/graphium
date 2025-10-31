package models

import (
	"encoding/json"
	"time"
)

// AgentTask represents a task that an agent should execute.
// This enables the server to delegate container lifecycle operations to agents
// running on remote hosts, using a pull-based task queue model.
//
// Task Flow:
//  1. Server creates AgentTask and stores in database
//  2. Agent polls for tasks via API
//  3. Agent executes task locally
//  4. Agent reports status back to server
//
// Example JSON representation:
//
//	{
//	  "@id": "task:deploy-nginx-1",
//	  "@type": "AgentTask",
//	  "taskType": "deploy",
//	  "status": "pending",
//	  "agentId": "agent:vm1",
//	  "hostId": "host:vm1",
//	  "stackId": "stack:nginx-multihost",
//	  "priority": 5,
//	  "payload": {...}
//	}
type AgentTask struct {
	// ID is the unique task identifier (maps to CouchDB _id)
	ID string `json:"@id" couchdb:"_id"`

	// Rev is the CouchDB document revision
	Rev string `json:"_rev,omitempty" couchdb:"_rev"`

	// Type is the JSON-LD @type
	Type string `json:"@type"`

	// TaskType defines what operation to perform
	// Values: "deploy", "delete", "stop", "start", "restart", "update"
	TaskType string `json:"taskType" couchdb:"index"`

	// Status tracks the task lifecycle
	// Values: "pending", "assigned", "running", "completed", "failed", "cancelled"
	Status string `json:"status" couchdb:"index"`

	// AgentID identifies which agent should execute this task
	AgentID string `json:"agentId" couchdb:"index"`

	// HostID identifies the target host
	HostID string `json:"hostId" couchdb:"index"`

	// StackID is the stack this task belongs to (optional)
	StackID string `json:"stackId,omitempty" couchdb:"index"`

	// ContainerID is the target container for single-container operations (optional)
	ContainerID string `json:"containerId,omitempty" couchdb:"index"`

	// Priority determines execution order (higher = more urgent)
	// Range: 0-10, default: 5
	// Example: delete tasks might have higher priority than deploy
	Priority int `json:"priority,omitempty"`

	// Payload contains task-specific data (JSON-encoded)
	// Content depends on TaskType:
	//  - deploy: DeployContainerPayload
	//  - delete: DeleteContainerPayload
	//  - stop/start/restart: ControlContainerPayload
	Payload json.RawMessage `json:"payload"`

	// CreatedAt is when the task was created
	CreatedAt time.Time `json:"dateCreated" couchdb:"index"`

	// CreatedBy is the user who created this task
	CreatedBy string `json:"createdBy,omitempty"`

	// AssignedAt is when the task was assigned to an agent
	AssignedAt *time.Time `json:"assignedAt,omitempty"`

	// StartedAt is when the agent started executing the task
	StartedAt *time.Time `json:"startedAt,omitempty"`

	// CompletedAt is when the task finished (success or failure)
	CompletedAt *time.Time `json:"completedAt,omitempty"`

	// Error contains error details if status is "failed"
	Error string `json:"error,omitempty"`

	// Result contains task execution results (optional, JSON-encoded)
	Result json.RawMessage `json:"result,omitempty"`

	// RetryCount tracks how many times this task has been retried
	RetryCount int `json:"retryCount,omitempty"`

	// MaxRetries is the maximum number of retry attempts (default: 3)
	MaxRetries int `json:"maxRetries,omitempty"`

	// TimeoutSeconds is how long the agent has to complete the task (default: 300)
	TimeoutSeconds int `json:"timeoutSeconds,omitempty"`

	// DependsOn lists task IDs that must complete before this task
	DependsOn []string `json:"dependsOn,omitempty"`

	// ScheduledBy is the ID of the ScheduledAction that created this task (optional)
	// Links tasks created by the scheduler back to their source action
	ScheduledBy string `json:"scheduledBy,omitempty" couchdb:"index"`
}

// Aliases for convenience and consistency with scheduler
type Task = AgentTask
type Params = map[string]interface{}

// Task status constants
const (
	TaskStatusPending   = "pending"
	TaskStatusAssigned  = "assigned"
	TaskStatusRunning   = "running"
	TaskStatusCompleted = "completed"
	TaskStatusFailed    = "failed"
	TaskStatusCancelled = "cancelled"
)

// DeployContainerPayload contains data for deploying a container.
type DeployContainerPayload struct {
	// ContainerSpec is the container specification to deploy
	ContainerSpec ContainerSpec `json:"containerSpec"`

	// NetworkConfig is optional network configuration
	NetworkConfig *NetworkSpec `json:"networkConfig,omitempty"`

	// Labels are custom labels to apply to the container
	Labels map[string]string `json:"labels,omitempty"`

	// PullPolicy determines when to pull the image
	// Values: "always", "if-not-present", "never"
	PullPolicy string `json:"pullPolicy,omitempty"`
}

// DeleteContainerPayload contains data for deleting a container.
type DeleteContainerPayload struct {
	// ContainerID is the Docker container ID to delete
	ContainerID string `json:"containerId"`

	// ContainerName is the container name (for logging)
	ContainerName string `json:"containerName,omitempty"`

	// Force forces removal even if container is running
	Force bool `json:"force,omitempty"`

	// RemoveVolumes removes associated volumes
	RemoveVolumes bool `json:"removeVolumes,omitempty"`

	// StopTimeout is the timeout in seconds before force-killing (default: 10)
	StopTimeout int `json:"stopTimeout,omitempty"`
}

// ControlContainerPayload contains data for start/stop/restart operations.
type ControlContainerPayload struct {
	// ContainerID is the Docker container ID
	ContainerID string `json:"containerId"`

	// ContainerName is the container name (for logging)
	ContainerName string `json:"containerName,omitempty"`

	// Timeout is the timeout in seconds for stop/restart (default: 10)
	Timeout int `json:"timeout,omitempty"`
}

// UpdateContainerPayload contains data for updating a running container.
type UpdateContainerPayload struct {
	// ContainerID is the Docker container ID to update
	ContainerID string `json:"containerId"`

	// UpdateSpec defines what to update
	UpdateSpec ContainerUpdateSpec `json:"updateSpec"`
}

// ContainerUpdateSpec defines container update parameters.
type ContainerUpdateSpec struct {
	// Image is the new image to use (triggers recreation)
	Image string `json:"image,omitempty"`

	// Env is the new environment variables
	Env []string `json:"env,omitempty"`

	// RestartPolicy is the new restart policy
	RestartPolicy string `json:"restartPolicy,omitempty"`

	// Resources are updated resource constraints (uses ResourceConstraints from stack_deployment.go)
	Resources *ResourceConstraints `json:"resources,omitempty"`
}

// CheckHealthPayload contains data for health check operations.
type CheckHealthPayload struct {
	// URL is the health check endpoint
	URL string `json:"url"`

	// Method is the HTTP method (default: GET)
	Method string `json:"method,omitempty"`

	// ExpectedStatusCode is the expected HTTP response code (default: 200)
	ExpectedStatusCode int `json:"expectedStatusCode,omitempty"`

	// Timeout is the timeout in seconds (default: 5)
	Timeout int `json:"timeout,omitempty"`

	// Headers are optional HTTP headers to send
	Headers map[string]string `json:"headers,omitempty"`

	// Body is optional HTTP request body
	Body string `json:"body,omitempty"`

	// ContainerID is the container to check (optional, for logging)
	ContainerID string `json:"containerId,omitempty"`
}

// TaskResult contains the result of a task execution.
type TaskResult struct {
	// Success indicates if the task succeeded
	Success bool `json:"success"`

	// ContainerID is the resulting container ID (for deploy tasks)
	ContainerID string `json:"containerId,omitempty"`

	// Message is a human-readable result message
	Message string `json:"message,omitempty"`

	// Data contains additional result data
	Data map[string]interface{} `json:"data,omitempty"`
}

// IsExpired checks if the task has exceeded its timeout.
func (t *AgentTask) IsExpired() bool {
	if t.StartedAt == nil {
		return false
	}

	timeout := t.TimeoutSeconds
	if timeout == 0 {
		timeout = 300 // Default 5 minutes
	}

	return time.Since(*t.StartedAt) > time.Duration(timeout)*time.Second
}

// CanRetry checks if the task can be retried.
func (t *AgentTask) CanRetry() bool {
	maxRetries := t.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3 // Default 3 retries
	}

	return t.RetryCount < maxRetries
}

// ShouldExecute checks if the task is ready to be executed by an agent.
func (t *AgentTask) ShouldExecute(agentID string) bool {
	// Task must be pending or assigned to this agent
	if t.Status != "pending" && t.Status != "assigned" {
		return false
	}

	// Task must be assigned to this agent (or unassigned)
	if t.AgentID != "" && t.AgentID != agentID {
		return false
	}

	return true
}

// GetPayloadAs unmarshals the payload into the given struct.
func (t *AgentTask) GetPayloadAs(v interface{}) error {
	return json.Unmarshal(t.Payload, v)
}

// SetPayload marshals the given struct into the payload.
func (t *AgentTask) SetPayload(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	t.Payload = data
	return nil
}

// SetResult sets the task result.
func (t *AgentTask) SetResult(result *TaskResult) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	t.Result = data
	return nil
}

// GetResult gets the task result.
func (t *AgentTask) GetResult() (*TaskResult, error) {
	if len(t.Result) == 0 {
		return nil, nil
	}

	var result TaskResult
	if err := json.Unmarshal(t.Result, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
