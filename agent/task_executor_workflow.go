package agent

import (
	"context"

	"evalgo.org/graphium/models"
)

// executeWorkflow executes a composite workflow action
func (e *TaskExecutor) executeWorkflow(ctx context.Context, task *models.AgentTask) (*models.TaskResult, error) {
	// Create workflow executor
	workflowExec := NewWorkflowExecutor(e)

	// Execute the composite action
	return workflowExec.ExecuteCompositeAction(ctx, task)
}

// isCompositeAction checks if a task is a composite action
