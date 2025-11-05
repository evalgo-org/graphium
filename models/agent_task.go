package models

import (
	"encoding/json"
	"fmt"
	"time"

	"eve.evalgo.org/semantic"
)

// AgentTask represents a task that an agent should execute.
// WORKFLOW-COMPATIBLE: Uses canonical semantic types from eve.evalgo.org/semantic
// This enables full workflow integration with when's SemanticScheduledAction
//
// Task Flow:
//  1. Server creates AgentTask and stores in database
//  2. Agent polls for tasks via API
//  3. Agent executes task locally
//  4. Agent reports status back to server
//
// Semantic Mapping:
//  - taskType → @type (ActivateAction, DeactivateAction, DeleteAction, UpdateAction)
//  - status → actionStatus (PotentialActionStatus, ActiveActionStatus, CompletedActionStatus, FailedActionStatus)
//  - agentId → agent (semantic.SemanticAgent)
//  - payload → object (semantic.SemanticObject)
//
// Example JSON representation:
//
//	{
//	  "@context": "https://schema.org",
//	  "@id": "task:deploy-nginx-1",
//	  "@type": "ActivateAction",
//	  "actionStatus": "PotentialActionStatus",
//	  "agent": {"@type": "SoftwareApplication", "name": "agent:vm1"},
//	  "object": {...},
//	  "startTime": "2024-01-01T10:00:00Z",
//	  "endTime": "2024-01-01T10:05:00Z"
//	}
type AgentTask struct {
	// CouchDB fields
	ID  string `json:"@id" couchdb:"_id"`
	Rev string `json:"_rev,omitempty" couchdb:"_rev"`

	// JSON-LD semantic fields
	Context string `json:"@context" couchdb:"@context"`
	Type    string `json:"@type"`

	// Schema.org Action properties (CANONICAL SEMANTIC TYPES)
	Name           string                       `json:"name,omitempty"`           // Human-readable task name
	Description    string                       `json:"description,omitempty"`    // Task description
	ActionStatus   string                       `json:"actionStatus"`             // Canonical status (PotentialActionStatus, ActiveActionStatus, CompletedActionStatus, FailedActionStatus)
	Agent          *semantic.SemanticAgent      `json:"agent,omitempty"`          // Agent executing the task
	Object         *semantic.SemanticObject     `json:"object,omitempty"`         // Target object (container spec, etc.)
	Instrument     *semantic.SemanticInstrument `json:"instrument,omitempty"`     // Tool/method used
	SemanticResult *semantic.SemanticResult     `json:"semanticResult,omitempty"` // Semantic execution result
	Error          *semantic.SemanticError      `json:"error,omitempty"`          // Error if failed
	StartTime      *time.Time                   `json:"startTime,omitempty"`      // Execution start time
	EndTime        *time.Time                   `json:"endTime,omitempty"`        // Execution end time

	// Workflow integration fields
	DependsOn  []string                  `json:"requires,omitempty"` // Task dependencies (uses 'requires' for workflow compat)
	Schedule   *semantic.SemanticSchedule `json:"schedule,omitempty"` // Optional schedule for recurring tasks
	Properties map[string]interface{}    `json:"additionalProperty,omitempty"` // Additional metadata

	// Task-specific fields
	HostID         string    `json:"hostId" couchdb:"index"`                // Target host
	StackID        string    `json:"stackId,omitempty" couchdb:"index"`     // Stack association
	ContainerID    string    `json:"containerId,omitempty" couchdb:"index"` // Container ID
	Priority       int       `json:"priority,omitempty"`                    // Execution priority (0-10)
	CreatedAt      time.Time `json:"dateCreated" couchdb:"index"`           // Creation time
	CreatedBy      string    `json:"createdBy,omitempty"`                   // Creator user
	RetryCount     int       `json:"retryCount,omitempty"`                  // Retry attempts
	MaxRetries     int       `json:"maxRetries,omitempty"`                  // Max retry limit
	TimeoutSeconds int       `json:"timeoutSeconds,omitempty"`              // Execution timeout
	ScheduledBy    string    `json:"scheduledBy,omitempty" couchdb:"index"` // Source ScheduledAction ID
}

// Aliases for convenience and consistency with scheduler
type Task = AgentTask
type Params = map[string]interface{}

// Task status constants (Schema.org actionStatus values)
const (
	TaskStatusPending   = "PotentialActionStatus" // Task is pending/waiting
	TaskStatusAssigned  = "PotentialActionStatus" // Task is assigned but not started
	TaskStatusRunning   = "ActiveActionStatus"    // Task is actively running
	TaskStatusCompleted = "CompletedActionStatus" // Task completed successfully
	TaskStatusFailed    = "FailedActionStatus"    // Task failed
	TaskStatusCancelled = "FailedActionStatus"    // Task was cancelled
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
	if t.StartTime == nil {
		return false
	}

	timeout := t.TimeoutSeconds
	if timeout == 0 {
		timeout = 300 // Default 5 minutes
	}

	return time.Since(*t.StartTime) > time.Duration(timeout)*time.Second
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
	// Task must be pending or assigned (PotentialActionStatus) or active (ActiveActionStatus)
	if t.ActionStatus != TaskStatusPending && t.ActionStatus != TaskStatusRunning {
		return false
	}

	// Check agent assignment
	taskAgentID := ""
	if t.Agent != nil {
		taskAgentID = t.Agent.Name
	}

	// Task must be assigned to this agent (or unassigned)
	if taskAgentID != "" && taskAgentID != agentID {
		return false
	}

	return true
}

// GetPayloadAs unmarshals the task object/payload into the given struct.
// Looks for payload data in Object.Properties or Instrument map.
func (t *AgentTask) GetPayloadAs(v interface{}) error {
	// Try to get payload from Object.Properties first
	if t.Object != nil && t.Object.Properties != nil {
		data, err := json.Marshal(t.Object.Properties)
		if err != nil {
			return err
		}
		return json.Unmarshal(data, v)
	}

	// Fallback to Instrument if Object is not available
	if t.Instrument != nil {
		data, err := json.Marshal(t.Instrument)
		if err != nil {
			return err
		}
		return json.Unmarshal(data, v)
	}

	return fmt.Errorf("no payload data found in task")
}

// SetPayload marshals the given struct into the task Object.
func (t *AgentTask) SetPayload(v interface{}) error {
	// Marshal the payload
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	// Parse into a map
	var payload map[string]interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}

	// Store in Object.Properties
	if t.Object == nil {
		t.Object = &semantic.SemanticObject{
			Type: "Thing",
		}
	}
	t.Object.Properties = payload

	return nil
}

// SetResult sets the task result.
func (t *AgentTask) SetResult(result *TaskResult) error {
	if t.SemanticResult == nil {
		t.SemanticResult = &semantic.SemanticResult{
			Type: "Result",
		}
	}

	// Store result data as JSON string in Output
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}

	t.SemanticResult.Output = string(data)
	return nil
}

// GetResult gets the task result.
func (t *AgentTask) GetResult() (*TaskResult, error) {
	if t.SemanticResult == nil || t.SemanticResult.Output == "" {
		return nil, nil
	}

	// Parse Output JSON string back to TaskResult
	var result TaskResult
	if err := json.Unmarshal([]byte(t.SemanticResult.Output), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

