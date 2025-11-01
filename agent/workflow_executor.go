package agent

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"evalgo.org/graphium/models"
)

// WorkflowExecutor handles composite action execution with variable substitution
type WorkflowExecutor struct {
	taskExecutor  *TaskExecutor
	actionOutputs map[string]map[string]interface{} // outputAs -> result data
}

// NewWorkflowExecutor creates a new workflow executor
func NewWorkflowExecutor(taskExecutor *TaskExecutor) *WorkflowExecutor {
	return &WorkflowExecutor{
		taskExecutor:  taskExecutor,
		actionOutputs: make(map[string]map[string]interface{}),
	}
}

// ExecuteCompositeAction executes a composite action with multiple steps
func (w *WorkflowExecutor) ExecuteCompositeAction(ctx context.Context, task *models.AgentTask) (*models.TaskResult, error) {
	// Parse composite action payload
	var payload map[string]interface{}
	if err := task.GetPayloadAs(&payload); err != nil {
		return nil, fmt.Errorf("invalid composite action payload: %w", err)
	}

	// Check if this is a composite action
	compositeAction, ok := payload["compositeAction"].(bool)
	if !ok || !compositeAction {
		return nil, fmt.Errorf("not a composite action")
	}

	// Get execution mode (sequential or parallel)
	executionMode, _ := payload["executionMode"].(string)
	if executionMode == "" {
		executionMode = "sequential"
	}

	// Get actions array
	actionsRaw, ok := payload["actions"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'actions' field in composite action")
	}

	// For now, only support sequential execution
	if executionMode != "sequential" {
		return nil, fmt.Errorf("only sequential execution mode is supported currently")
	}

	// Execute actions sequentially
	w.actionOutputs = make(map[string]map[string]interface{})
	var lastResult *models.TaskResult

	for i, actionRaw := range actionsRaw {
		actionData, ok := actionRaw.(map[string]interface{})
		if !ok {
			return &models.TaskResult{
				Success: false,
				Message: fmt.Sprintf("Invalid action at index %d", i),
			}, nil
		}

		// Substitute variables in the action
		substitutedAction := w.substituteVariables(actionData)

		// Execute the action
		result, err := w.executeSingleAction(ctx, substitutedAction)
		if err != nil {
			return &models.TaskResult{
				Success: false,
				Message: fmt.Sprintf("Failed to execute action %d: %v", i, err),
				Data: map[string]interface{}{
					"failed_step": i,
					"error":       err.Error(),
				},
			}, nil
		}

		if !result.Success {
			return &models.TaskResult{
				Success: false,
				Message: fmt.Sprintf("Action %d failed: %s", i, result.Message),
				Data: map[string]interface{}{
					"failed_step": i,
					"step_result": result,
				},
			}, nil
		}

		// Store output if outputAs is specified
		if outputAs, ok := actionData["outputAs"].(string); ok {
			w.actionOutputs[outputAs] = result.Data
		}

		lastResult = result
	}

	return &models.TaskResult{
		Success: true,
		Message: fmt.Sprintf("Successfully executed %d actions", len(actionsRaw)),
		Data: map[string]interface{}{
			"actions_count":  len(actionsRaw),
			"last_result":    lastResult,
			"action_outputs": w.actionOutputs,
		},
	}, nil
}

// executeSingleAction executes a single action within a workflow
func (w *WorkflowExecutor) executeSingleAction(ctx context.Context, actionData map[string]interface{}) (*models.TaskResult, error) {
	actionType, _ := actionData["type"].(string)

	switch actionType {
	case "container-exec":
		return w.taskExecutor.executeContainerExec(ctx, actionData)
	case "deploy-stack":
		return w.executeDeployStack(ctx, actionData)
	case "wait":
		return w.executeWait(ctx, actionData)
	default:
		return nil, fmt.Errorf("unsupported action type: %s", actionType)
	}
}

// substituteVariables replaces variable placeholders in action data
// Supports syntax: ${{outputName.field.subfield}}
func (w *WorkflowExecutor) substituteVariables(data map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for key, value := range data {
		switch v := value.(type) {
		case string:
			result[key] = w.substituteString(v)
		case map[string]interface{}:
			result[key] = w.substituteVariables(v)
		case []interface{}:
			result[key] = w.substituteArray(v)
		default:
			result[key] = value
		}
	}

	return result
}

// substituteString replaces variable references in a string
func (w *WorkflowExecutor) substituteString(s string) string {
	// Match pattern: ${{variable.path.here}}
	re := regexp.MustCompile(`\$\{\{([^}]+)\}\}`)

	return re.ReplaceAllStringFunc(s, func(match string) string {
		// Extract variable path (remove ${{ and }})
		varPath := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(match, "${{"), "}}"))

		// Split path into parts
		parts := strings.Split(varPath, ".")
		if len(parts) == 0 {
			return match // Return original if invalid
		}

		// Get output by name
		output, ok := w.actionOutputs[parts[0]]
		if !ok {
			return match // Return original if not found
		}

		// Navigate nested path
		current := output
		for i := 1; i < len(parts); i++ {
			if nested, ok := current[parts[i]]; ok {
				if nestedMap, ok := nested.(map[string]interface{}); ok {
					current = nestedMap
				} else {
					// If it's a final value, convert to string
					return fmt.Sprintf("%v", nested)
				}
			} else {
				return match // Return original if path not found
			}
		}

		// If we've traversed the whole path, return the value
		return fmt.Sprintf("%v", current)
	})
}

// substituteArray substitutes variables in an array
func (w *WorkflowExecutor) substituteArray(arr []interface{}) []interface{} {
	result := make([]interface{}, len(arr))
	for i, item := range arr {
		switch v := item.(type) {
		case string:
			result[i] = w.substituteString(v)
		case map[string]interface{}:
			result[i] = w.substituteVariables(v)
		case []interface{}:
			result[i] = w.substituteArray(v)
		default:
			result[i] = item
		}
	}
	return result
}

// executeDeployStack executes a stack deployment action
func (w *WorkflowExecutor) executeDeployStack(ctx context.Context, actionData map[string]interface{}) (*models.TaskResult, error) {
	stackID, ok := actionData["stackId"].(string)
	if !ok {
		return nil, fmt.Errorf("missing stackId in deploy-stack action")
	}

	// TODO: Implement stack deployment
	// For now, return a placeholder
	return &models.TaskResult{
		Success: true,
		Message: fmt.Sprintf("Stack %s deployment placeholder", stackID),
		Data: map[string]interface{}{
			"stack_id": stackID,
			"containers": map[string]string{
				"example": "container-id-placeholder",
			},
		},
	}, nil
}

// executeWait executes a wait action
func (w *WorkflowExecutor) executeWait(ctx context.Context, actionData map[string]interface{}) (*models.TaskResult, error) {
	// TODO: Implement wait logic (health checks, etc.)
	return &models.TaskResult{
		Success: true,
		Message: "Wait action placeholder",
	}, nil
}
