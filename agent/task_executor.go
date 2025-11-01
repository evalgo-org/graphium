package agent

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"

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
			if reportErr := e.reportTaskStatus(task.ID, "failed", err.Error(), nil); reportErr != nil {
				log.Printf("Failed to report task failure status: %v", reportErr)
			}
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

	case "check":
		result, err = e.executeCheck(ctx, task)

	case "control":
		result, err = e.executeControl(ctx, task)

	case "transfer":
		result, err = e.executeTransfer(ctx, task)

	case "workflow":
		result, err = e.executeWorkflow(ctx, task)

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

// executeCheck executes a health check task.
func (e *TaskExecutor) executeCheck(ctx context.Context, task *models.AgentTask) (*models.TaskResult, error) {
	// First, check if this is a TLS certificate check
	var rawPayload map[string]interface{}
	if err := task.GetPayloadAs(&rawPayload); err != nil {
		return nil, fmt.Errorf("invalid check payload: %w", err)
	}

	// Route to TLS certificate check if specified
	if checkType, ok := rawPayload["checkType"].(string); ok && checkType == "tls-certificate" {
		return e.executeTLSCertificateCheck(ctx, rawPayload)
	}

	// Otherwise, execute HTTP health check
	var payload models.CheckHealthPayload
	if err := task.GetPayloadAs(&payload); err != nil {
		return nil, fmt.Errorf("invalid check payload: %w", err)
	}

	// Set defaults
	if payload.Method == "" {
		payload.Method = "GET"
	}
	if payload.ExpectedStatusCode == 0 {
		payload.ExpectedStatusCode = 200
	}
	if payload.Timeout == 0 {
		payload.Timeout = 5
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, payload.Method, payload.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	for key, value := range payload.Headers {
		req.Header.Set(key, value)
	}

	// Set body if provided
	if payload.Body != "" {
		req.Body = io.NopCloser(bytes.NewBufferString(payload.Body))
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: time.Duration(payload.Timeout) * time.Second,
	}

	// Execute request
	startTime := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(startTime)

	if err != nil {
		return &models.TaskResult{
			Success: false,
			Message: fmt.Sprintf("Health check failed: %v", err),
			Data: map[string]interface{}{
				"url":          payload.URL,
				"error":        err.Error(),
				"duration_ms":  duration.Milliseconds(),
				"container_id": payload.ContainerID,
			},
		}, nil
	}
	defer resp.Body.Close()

	// Read response body (limited to 1KB for logging)
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	bodyPreview := string(bodyBytes)

	// Check status code
	success := resp.StatusCode == payload.ExpectedStatusCode
	message := fmt.Sprintf("Health check %s", map[bool]string{true: "passed", false: "failed"}[success])

	return &models.TaskResult{
		Success: success,
		Message: message,
		Data: map[string]interface{}{
			"url":              payload.URL,
			"method":           payload.Method,
			"status_code":      resp.StatusCode,
			"expected_status":  payload.ExpectedStatusCode,
			"duration_ms":      duration.Milliseconds(),
			"response_preview": bodyPreview,
			"container_id":     payload.ContainerID,
			"content_length":   resp.ContentLength,
			"content_type":     resp.Header.Get("Content-Type"),
		},
	}, nil
}

// executeControl executes a control task (dispatches to specific operations).
func (e *TaskExecutor) executeControl(ctx context.Context, task *models.AgentTask) (*models.TaskResult, error) {
	// Parse payload to get control parameters
	var payload map[string]interface{}
	if err := task.GetPayloadAs(&payload); err != nil {
		return nil, fmt.Errorf("invalid control payload: %w", err)
	}

	// Get action type from payload
	action, ok := payload["action"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'action' field in payload")
	}

	containerID, ok := payload["containerId"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'containerId' field in payload")
	}

	timeout := 30.0 // default timeout
	if timeoutVal, ok := payload["timeout"].(float64); ok {
		timeout = timeoutVal
	}

	// Create control payload
	controlPayload := &models.ControlContainerPayload{
		ContainerID: containerID,
		Timeout:     int(timeout),
	}

	// Execute based on action
	switch action {
	case "restart":
		return e.deployer.RestartContainer(ctx, controlPayload)
	case "stop":
		return e.deployer.StopContainer(ctx, controlPayload)
	case "start":
		return e.deployer.StartContainer(ctx, controlPayload)
	case "pause":
		// Pause operation would be similar to stop
		return e.deployer.StopContainer(ctx, controlPayload)
	case "unpause":
		// Unpause operation would be similar to start
		return e.deployer.StartContainer(ctx, controlPayload)
	default:
		return nil, fmt.Errorf("unsupported control action: %s", action)
	}
}

// executeTransfer executes a transfer task (log collection).
func (e *TaskExecutor) executeTransfer(ctx context.Context, task *models.AgentTask) (*models.TaskResult, error) {
	// Parse payload to get transfer parameters
	var payload map[string]interface{}
	if err := task.GetPayloadAs(&payload); err != nil {
		return nil, fmt.Errorf("invalid transfer payload: %w", err)
	}

	action, _ := payload["action"].(string)
	containerID, ok := payload["containerId"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'containerId' field in payload")
	}

	// Route to log collection if specified
	if action == "collect-logs" {
		return e.executeLogCollection(ctx, payload)
	}

	// For other transfer actions, return not implemented
	return &models.TaskResult{
		Success: false,
		Message: fmt.Sprintf("Transfer action '%s' not yet implemented", action),
		Data: map[string]interface{}{
			"action":       action,
			"container_id": containerID,
		},
	}, nil
}

// executeLogCollection collects logs from a container.
func (e *TaskExecutor) executeLogCollection(ctx context.Context, payload map[string]interface{}) (*models.TaskResult, error) {
	// Extract parameters
	containerID, ok := payload["containerId"].(string)
	if !ok || containerID == "" {
		return nil, fmt.Errorf("missing or invalid 'containerId' field in payload")
	}

	// Extract optional parameters with defaults
	lines := int64(100) // default to 100 lines
	if linesVal, ok := payload["lines"].(float64); ok {
		lines = int64(linesVal)
	}

	since := ""
	if sinceVal, ok := payload["since"].(string); ok {
		since = sinceVal
	}

	destination := "/tmp/graphium-logs" // default destination
	if destVal, ok := payload["destination"].(string); ok {
		destination = destVal
	}

	// Parse destination to get file path
	// Expected format: file:///path/to/destination/
	destinationPath := strings.TrimPrefix(destination, "file://")
	if destinationPath == "" {
		destinationPath = "/tmp/graphium-logs"
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(destinationPath, 0750); err != nil {
		return &models.TaskResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create destination directory: %v", err),
			Data: map[string]interface{}{
				"container_id": containerID,
				"destination":  destinationPath,
				"error":        err.Error(),
			},
		}, nil
	}

	// Get container logs using Docker API
	startTime := time.Now()

	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       fmt.Sprintf("%d", lines),
		Timestamps: true,
	}

	// Add time filter if specified
	if since != "" {
		options.Since = since
	}

	logReader, err := e.agent.docker.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return &models.TaskResult{
			Success: false,
			Message: fmt.Sprintf("Failed to fetch container logs: %v", err),
			Data: map[string]interface{}{
				"container_id": containerID,
				"lines":        lines,
				"since":        since,
				"error":        err.Error(),
			},
		}, nil
	}
	defer logReader.Close()

	// Read all logs
	logBytes, err := io.ReadAll(logReader)
	if err != nil {
		return &models.TaskResult{
			Success: false,
			Message: fmt.Sprintf("Failed to read container logs: %v", err),
			Data: map[string]interface{}{
				"container_id": containerID,
				"error":        err.Error(),
			},
		}, nil
	}

	duration := time.Since(startTime)

	// Create log file with timestamp
	timestamp := time.Now().Format("20060102-150405")
	logFileName := fmt.Sprintf("%s-%s.log", containerID, timestamp)
	logFilePath := fmt.Sprintf("%s/%s", destinationPath, logFileName)

	// Write logs to file
	if err := os.WriteFile(logFilePath, logBytes, 0600); err != nil {
		return &models.TaskResult{
			Success: false,
			Message: fmt.Sprintf("Failed to write logs to file: %v", err),
			Data: map[string]interface{}{
				"container_id": containerID,
				"destination":  logFilePath,
				"error":        err.Error(),
			},
		}, nil
	}

	logSize := len(logBytes)
	logLines := bytes.Count(logBytes, []byte("\n"))

	return &models.TaskResult{
		Success: true,
		Message: fmt.Sprintf("Successfully collected %d lines (%d bytes) of logs from container %s", logLines, logSize, containerID),
		Data: map[string]interface{}{
			"container_id":    containerID,
			"log_file":        logFilePath,
			"log_size":        logSize,
			"log_lines":       logLines,
			"duration_ms":     duration.Milliseconds(),
			"lines_requested": lines,
			"since":           since,
		},
	}, nil
}

// executeTLSCertificateCheck executes a TLS certificate expiration check.
func (e *TaskExecutor) executeTLSCertificateCheck(ctx context.Context, payload map[string]interface{}) (*models.TaskResult, error) {
	// Extract parameters
	urlStr, ok := payload["url"].(string)
	if !ok || urlStr == "" {
		return nil, fmt.Errorf("missing or invalid 'url' field in payload")
	}

	warnDays := 30.0 // default warning threshold
	if warnVal, ok := payload["warnDays"].(float64); ok {
		warnDays = warnVal
	}

	// Parse URL to extract host and port
	// Remove protocol if present
	host := strings.TrimPrefix(urlStr, "https://")
	host = strings.TrimPrefix(host, "http://")

	// If no port specified, use 443
	if !strings.Contains(host, ":") {
		host = host + ":443"
	}

	// Create TLS dialer with secure config
	dialer := &tls.Dialer{
		Config: &tls.Config{
			InsecureSkipVerify: false,            // Verify certificate chain
			MinVersion:         tls.VersionTLS12, // Minimum TLS 1.2
		},
	}

	// Connect and get certificate
	startTime := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", host)
	duration := time.Since(startTime)

	if err != nil {
		return &models.TaskResult{
			Success: false,
			Message: fmt.Sprintf("TLS certificate check failed: %v", err),
			Data: map[string]interface{}{
				"url":         urlStr,
				"host":        host,
				"error":       err.Error(),
				"duration_ms": duration.Milliseconds(),
			},
		}, nil
	}
	defer conn.Close()

	// Get TLS connection state
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return nil, fmt.Errorf("failed to get TLS connection")
	}

	// Get certificate chain
	state := tlsConn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return &models.TaskResult{
			Success: false,
			Message: "No certificates found in TLS handshake",
			Data: map[string]interface{}{
				"url":         urlStr,
				"host":        host,
				"duration_ms": duration.Milliseconds(),
			},
		}, nil
	}

	// Get the leaf certificate (first in chain)
	cert := state.PeerCertificates[0]

	// Calculate days until expiration
	now := time.Now()
	daysUntilExpiry := int(cert.NotAfter.Sub(now).Hours() / 24)

	// Check if certificate is expired
	if now.After(cert.NotAfter) {
		return &models.TaskResult{
			Success: false,
			Message: fmt.Sprintf("Certificate EXPIRED on %s (%d days ago)", cert.NotAfter.Format("2006-01-02 15:04:05 MST"), -daysUntilExpiry),
			Data: map[string]interface{}{
				"url":                 urlStr,
				"host":                host,
				"subject":             cert.Subject.CommonName,
				"issuer":              cert.Issuer.CommonName,
				"not_before":          cert.NotBefore.Format("2006-01-02 15:04:05 MST"),
				"not_after":           cert.NotAfter.Format("2006-01-02 15:04:05 MST"),
				"expires_in_days":     daysUntilExpiry,
				"is_expired":          true,
				"dns_names":           cert.DNSNames,
				"serial_number":       cert.SerialNumber.String(),
				"duration_ms":         duration.Milliseconds(),
				"signature_algorithm": cert.SignatureAlgorithm.String(),
			},
		}, nil
	}

	// Check if certificate is not yet valid
	if now.Before(cert.NotBefore) {
		return &models.TaskResult{
			Success: false,
			Message: fmt.Sprintf("Certificate not yet valid (valid from %s)", cert.NotBefore.Format("2006-01-02 15:04:05 MST")),
			Data: map[string]interface{}{
				"url":             urlStr,
				"host":            host,
				"subject":         cert.Subject.CommonName,
				"issuer":          cert.Issuer.CommonName,
				"not_before":      cert.NotBefore.Format("2006-01-02 15:04:05 MST"),
				"not_after":       cert.NotAfter.Format("2006-01-02 15:04:05 MST"),
				"expires_in_days": daysUntilExpiry,
				"dns_names":       cert.DNSNames,
				"serial_number":   cert.SerialNumber.String(),
				"duration_ms":     duration.Milliseconds(),
			},
		}, nil
	}

	// Check warning threshold
	warningThreshold := int(warnDays)
	success := daysUntilExpiry > warningThreshold

	var message string
	if success {
		message = fmt.Sprintf("Certificate valid - expires on %s (in %d days)", cert.NotAfter.Format("2006-01-02 15:04:05 MST"), daysUntilExpiry)
	} else {
		message = fmt.Sprintf("WARNING: Certificate expires soon on %s (in %d days, threshold: %d days)", cert.NotAfter.Format("2006-01-02 15:04:05 MST"), daysUntilExpiry, warningThreshold)
	}

	// Verify certificate chain
	opts := x509.VerifyOptions{
		DNSName: strings.Split(host, ":")[0], // Remove port for DNS verification
	}
	_, err = cert.Verify(opts)
	chainValid := err == nil

	return &models.TaskResult{
		Success: success,
		Message: message,
		Data: map[string]interface{}{
			"url":                 urlStr,
			"host":                host,
			"subject":             cert.Subject.CommonName,
			"issuer":              cert.Issuer.CommonName,
			"not_before":          cert.NotBefore.Format("2006-01-02 15:04:05 MST"),
			"not_after":           cert.NotAfter.Format("2006-01-02 15:04:05 MST"),
			"expires_in_days":     daysUntilExpiry,
			"warning_threshold":   warningThreshold,
			"is_expired":          false,
			"is_valid":            success,
			"chain_valid":         chainValid,
			"dns_names":           cert.DNSNames,
			"serial_number":       cert.SerialNumber.String(),
			"signature_algorithm": cert.SignatureAlgorithm.String(),
			"duration_ms":         duration.Milliseconds(),
			"cert_version":        cert.Version,
		},
	}, nil
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
