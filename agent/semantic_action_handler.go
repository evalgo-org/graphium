package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"eve.evalgo.org/semantic"

	"evalgo.org/graphium/models"
)

// handleSemanticAction handles semantic action execution requests
// Endpoint: POST /v1/api/semantic/action
// This makes the agent a first-class semantic service in the EVE ecosystem
func (a *Agent) handleSemanticAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse incoming semantic action
	var action semantic.SemanticScheduledAction
	if err := json.NewDecoder(r.Body).Decode(&action); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	log.Printf("Received semantic action: type=%s, name=%s", action.Type, action.Name)

	// Execute action based on type
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	result, err := a.executeSemanticAction(ctx, &action)
	if err != nil {
		log.Printf("Semantic action execution failed: %v", err)

		// Return semantic error response
		errorResponse := map[string]interface{}{
			"@context": "https://schema.org",
			"@type":    "Action",
			"actionStatus": "FailedActionStatus",
			"error": map[string]interface{}{
				"@type":       "Thing",
				"name":        "ExecutionError",
				"description": err.Error(),
			},
		}

		w.Header().Set("Content-Type", "application/ld+json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	// Return semantic success response
	w.Header().Set("Content-Type", "application/ld+json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(result); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// executeSemanticAction executes a semantic action and returns the result
func (a *Agent) executeSemanticAction(ctx context.Context, action *semantic.SemanticScheduledAction) (map[string]interface{}, error) {
	startTime := time.Now()

	switch action.Type {
	case "ControlAction":
		return a.executeControlAction(ctx, action, startTime)

	case "CreateAction", "DeployAction":
		return a.executeDeployAction(ctx, action, startTime)

	case "DeleteAction":
		return a.executeDeleteAction(ctx, action, startTime)

	case "CheckAction":
		return a.executeCheckAction(ctx, action, startTime)

	default:
		return nil, fmt.Errorf("unsupported action type: %s", action.Type)
	}
}

// executeControlAction handles start/stop/restart operations
func (a *Agent) executeControlAction(ctx context.Context, action *semantic.SemanticScheduledAction, startTime time.Time) (map[string]interface{}, error) {
	// Extract container ID from object
	containerID := ""
	if action.Object != nil {
		containerID = action.Object.Identifier
	}

	if containerID == "" {
		return nil, fmt.Errorf("container ID required in action.object.identifier")
	}

	// Determine operation from instrument or properties
	operation := "restart" // default
	if action.Instrument != nil {
		operation = action.Instrument.Name
	} else if props, ok := action.Properties["operation"].(string); ok {
		operation = props
	}

	deployer := NewDeployer(a.docker, a.hostID, a.hostID)
	payload := &models.ControlContainerPayload{
		ContainerID: containerID,
	}

	var taskResult *models.TaskResult
	var err error

	switch operation {
	case "start":
		taskResult, err = deployer.StartContainer(ctx, payload)
	case "stop":
		taskResult, err = deployer.StopContainer(ctx, payload)
	case "restart":
		taskResult, err = deployer.RestartContainer(ctx, payload)
	default:
		return nil, fmt.Errorf("unsupported control operation: %s", operation)
	}

	if err != nil {
		return nil, err
	}

	// Convert to semantic result
	duration := time.Since(startTime)
	return map[string]interface{}{
		"@context":     "https://schema.org",
		"@type":        "Action",
		"actionStatus": "CompletedActionStatus",
		"result": map[string]interface{}{
			"@type":       "Thing",
			"name":        fmt.Sprintf("Container %s completed", operation),
			"description": taskResult.Message,
			"value":       taskResult.Data,
		},
		"startTime": startTime.Format(time.RFC3339),
		"endTime":   time.Now().Format(time.RFC3339),
		"duration":  fmt.Sprintf("PT%dS", int(duration.Seconds())),
	}, nil
}

// executeDeployAction handles container deployment
func (a *Agent) executeDeployAction(ctx context.Context, action *semantic.SemanticScheduledAction, startTime time.Time) (map[string]interface{}, error) {
	// Parse deployment configuration from properties
	var deployPayload models.DeployContainerPayload

	// Convert properties to deployment payload
	if propsBytes, err := json.Marshal(action.Properties); err == nil {
		if err := json.Unmarshal(propsBytes, &deployPayload); err != nil {
			return nil, fmt.Errorf("invalid deploy configuration: %w", err)
		}
	}

	// Ensure required fields
	if deployPayload.ContainerSpec.Image == "" {
		return nil, fmt.Errorf("image required in action properties")
	}

	deployer := NewDeployer(a.docker, a.hostID, a.hostID)
	taskResult, err := deployer.DeployContainer(ctx, &deployPayload)
	if err != nil {
		return nil, err
	}

	duration := time.Since(startTime)
	return map[string]interface{}{
		"@context":     "https://schema.org",
		"@type":        "CreateAction",
		"actionStatus": "CompletedActionStatus",
		"result": map[string]interface{}{
			"@type":       "SoftwareApplication",
			"identifier":  taskResult.Data["container_id"],
			"name":        deployPayload.ContainerSpec.Image,
			"description": taskResult.Message,
			"value":       taskResult.Data,
		},
		"startTime": startTime.Format(time.RFC3339),
		"endTime":   time.Now().Format(time.RFC3339),
		"duration":  fmt.Sprintf("PT%dS", int(duration.Seconds())),
	}, nil
}

// executeDeleteAction handles container deletion
func (a *Agent) executeDeleteAction(ctx context.Context, action *semantic.SemanticScheduledAction, startTime time.Time) (map[string]interface{}, error) {
	// Extract container ID
	containerID := ""
	if action.Object != nil {
		containerID = action.Object.Identifier
	}

	if containerID == "" {
		return nil, fmt.Errorf("container ID required in action.object.identifier")
	}

	deployer := NewDeployer(a.docker, a.hostID, a.hostID)
	payload := &models.DeleteContainerPayload{
		ContainerID: containerID,
	}

	taskResult, err := deployer.DeleteContainer(ctx, payload)
	if err != nil {
		return nil, err
	}

	duration := time.Since(startTime)
	return map[string]interface{}{
		"@context":     "https://schema.org",
		"@type":        "DeleteAction",
		"actionStatus": "CompletedActionStatus",
		"result": map[string]interface{}{
			"@type":       "Thing",
			"name":        "Container deleted",
			"description": taskResult.Message,
			"identifier":  containerID,
		},
		"startTime": startTime.Format(time.RFC3339),
		"endTime":   time.Now().Format(time.RFC3339),
		"duration":  fmt.Sprintf("PT%dS", int(duration.Seconds())),
	}, nil
}

// executeCheckAction handles health checks
func (a *Agent) executeCheckAction(ctx context.Context, action *semantic.SemanticScheduledAction, startTime time.Time) (map[string]interface{}, error) {
	// Extract container ID
	containerID := ""
	if action.Object != nil {
		containerID = action.Object.Identifier
	}

	if containerID == "" {
		return nil, fmt.Errorf("container ID required in action.object.identifier")
	}

	// Inspect container to check health
	containerJSON, err := a.docker.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	// Check container state
	isHealthy := containerJSON.State.Running
	healthStatus := "Healthy"
	if !isHealthy {
		healthStatus = "Unhealthy"
	}

	duration := time.Since(startTime)
	return map[string]interface{}{
		"@context":     "https://schema.org",
		"@type":        "CheckAction",
		"actionStatus": "CompletedActionStatus",
		"result": map[string]interface{}{
			"@type":       "Thing",
			"name":        healthStatus,
			"description": fmt.Sprintf("Container %s status: %s", containerID, containerJSON.State.Status),
			"value": map[string]interface{}{
				"container_id": containerID,
				"running":      containerJSON.State.Running,
				"status":       containerJSON.State.Status,
				"health":       containerJSON.State.Health,
			},
		},
		"startTime": startTime.Format(time.RFC3339),
		"endTime":   time.Now().Format(time.RFC3339),
		"duration":  fmt.Sprintf("PT%dS", int(duration.Seconds())),
	}, nil
}
