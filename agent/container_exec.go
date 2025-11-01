package agent

import (
	"context"
	"fmt"
	"io"
	"time"

	"evalgo.org/graphium/models"
	"github.com/docker/docker/api/types/container"
)

// executeContainerExec executes a command inside a running container
func (e *TaskExecutor) executeContainerExec(ctx context.Context, payload map[string]interface{}) (*models.TaskResult, error) {
	startTime := time.Now()

	// Extract container ID
	containerID, ok := payload["containerId"].(string)
	if !ok || containerID == "" {
		return nil, fmt.Errorf("missing or invalid 'containerId' field in payload")
	}

	// Extract command
	commandRaw, ok := payload["command"]
	if !ok {
		return nil, fmt.Errorf("missing 'command' field in payload")
	}

	// Convert command to string slice
	var cmd []string
	switch v := commandRaw.(type) {
	case []interface{}:
		cmd = make([]string, len(v))
		for i, c := range v {
			cmd[i] = fmt.Sprintf("%v", c)
		}
	case []string:
		cmd = v
	case string:
		// Single command string
		cmd = []string{"/bin/sh", "-c", v}
	default:
		return nil, fmt.Errorf("invalid command format: must be string or array")
	}

	// Extract optional working directory
	workDir, _ := payload["workDir"].(string)

	// Extract optional environment variables
	var env []string
	if envRaw, ok := payload["env"].(map[string]interface{}); ok {
		for k, v := range envRaw {
			env = append(env, fmt.Sprintf("%s=%v", k, v))
		}
	}

	// Extract optional user
	user, _ := payload["user"].(string)

	// Create exec configuration
	execConfig := container.ExecOptions{
		User:         user,
		Privileged:   false,
		Tty:          false,
		AttachStdin:  false,
		AttachStdout: true,
		AttachStderr: true,
		Detach:       false,
		DetachKeys:   "",
		Env:          env,
		WorkingDir:   workDir,
		Cmd:          cmd,
	}

	// Create the exec instance
	execResp, err := e.agent.docker.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return &models.TaskResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create exec instance: %v", err),
			Data: map[string]interface{}{
				"container_id": containerID,
				"command":      cmd,
				"error":        err.Error(),
			},
		}, nil
	}

	// Start and attach to the exec instance
	resp, err := e.agent.docker.ContainerExecAttach(ctx, execResp.ID, container.ExecStartOptions{})
	if err != nil {
		return &models.TaskResult{
			Success: false,
			Message: fmt.Sprintf("Failed to attach to exec instance: %v", err),
			Data: map[string]interface{}{
				"container_id": containerID,
				"command":      cmd,
				"exec_id":      execResp.ID,
				"error":        err.Error(),
			},
		}, nil
	}
	defer resp.Close()

	// Read the output
	output, err := io.ReadAll(resp.Reader)
	if err != nil {
		return &models.TaskResult{
			Success: false,
			Message: fmt.Sprintf("Failed to read exec output: %v", err),
			Data: map[string]interface{}{
				"container_id": containerID,
				"command":      cmd,
				"error":        err.Error(),
			},
		}, nil
	}

	// Inspect the exec instance to get the exit code
	inspect, err := e.agent.docker.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		return &models.TaskResult{
			Success: false,
			Message: fmt.Sprintf("Failed to inspect exec instance: %v", err),
			Data: map[string]interface{}{
				"container_id": containerID,
				"command":      cmd,
				"output":       string(output),
				"error":        err.Error(),
			},
		}, nil
	}

	duration := time.Since(startTime)
	exitCode := inspect.ExitCode

	// Determine success based on exit code
	success := exitCode == 0
	message := "Command executed successfully"
	if !success {
		message = fmt.Sprintf("Command failed with exit code %d", exitCode)
	}

	return &models.TaskResult{
		Success: success,
		Message: message,
		Data: map[string]interface{}{
			"container_id": containerID,
			"command":      cmd,
			"exit_code":    exitCode,
			"output":       string(output),
			"duration_ms":  duration.Milliseconds(),
			"exec_id":      execResp.ID,
		},
	}, nil
}
