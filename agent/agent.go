// Package agent provides Docker container discovery and automatic synchronization
// with the Graphium API server.
//
// The agent runs on each Docker host and performs the following tasks:
//   - Discovers all running and stopped containers via Docker API
//   - Registers the host with the Graphium API server
//   - Monitors Docker events in real-time (start, stop, create, destroy, etc.)
//   - Automatically syncs container state changes to the API
//   - Implements rate limiting to respect API server constraints
//   - Handles authentication via Bearer tokens
//
// The agent uses a two-phase sync strategy:
//  1. Initial full sync on startup to discover all existing containers
//  2. Event-driven updates for real-time container lifecycle changes
//  3. Periodic full sync (every 30 seconds) to catch any missed events
//
// Rate Limiting:
//
// To prevent overwhelming the API server, the agent implements 100ms delays
// between container sync operations during bulk operations. This spreads the
// request load over time and stays well within the server's 100 req/s limit.
//
// Error Handling:
//
// The agent uses a smart sync logic that handles various HTTP response codes:
//   - 200 OK: Container exists, perform UPDATE (PUT)
//   - 404 Not Found: Container doesn't exist, perform CREATE (POST)
//   - 401 Unauthorized: Auth issue, perform CREATE (POST) to retry
//   - Other errors: Logged and retried on next sync cycle
//
// Example usage:
//
//	agent, err := agent.NewAgent(
//	    "http://localhost:8095",
//	    "host-01",
//	    "us-east",
//	    "/var/run/docker.sock",
//	    "agent-token",
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	ctx := context.Background()
//	if err := agent.Start(ctx); err != nil {
//	    log.Fatal(err)
//	}
package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	dockerclient "github.com/docker/docker/client"

	"eve.evalgo.org/network"

	"evalgo.org/graphium/models"
)

// Agent manages Docker container discovery and synchronization with the API server.
// It monitors the Docker daemon for container events and automatically syncs changes
// to the Graphium API, providing real-time visibility into container state across
// distributed hosts.
type Agent struct {
	apiURL           string
	hostID           string
	datacenter       string
	dockerSocket     string
	docker           *dockerclient.Client
	httpClient       *http.Client
	sshTunnel        *network.SSHTunnel
	syncInterval     time.Duration
	hostInfo         *models.Host
	authToken        string
	httpPort         int // HTTP server port (0 = disabled)
	startTime        time.Time
	syncCount        int64
	failedSyncs      int64
	eventsCount      int64
	lastSyncTime     time.Time
	lastSyncDuration time.Duration
}

// NewAgent creates a new agent instance.
func NewAgent(apiURL, hostID, datacenter, dockerSocket, agentToken string, httpPort int) (*Agent, error) {
	if apiURL == "" {
		return nil, fmt.Errorf("api URL is required")
	}
	if hostID == "" {
		return nil, fmt.Errorf("host ID is required")
	}

	// Use default Docker socket if not specified
	if dockerSocket == "" {
		dockerSocket = "/var/run/docker.sock"
	}

	var tunnel *network.SSHTunnel
	var dockerClient *dockerclient.Client

	// Check if using SSH connection
	if strings.HasPrefix(dockerSocket, "ssh://") {
		// Parse SSH URL: ssh://user@host:port
		u, err := url.Parse(dockerSocket)
		if err != nil {
			return nil, fmt.Errorf("failed to parse SSH URL: %w", err)
		}

		// Extract username from URL
		username := u.User.Username()
		if username == "" {
			return nil, fmt.Errorf("SSH URL must include username (e.g., ssh://user@host)")
		}

		// Extract host and port
		host := u.Hostname()
		port := u.Port()
		if port == "" {
			port = "22" // Default SSH port
		}
		sshAddress := net.JoinHostPort(host, port)

		// Get SSH key path from environment
		sshKeyPath := os.Getenv("DOCKER_SSH_IDENTITY")
		if sshKeyPath == "" {
			sshKeyPath = os.Getenv("HOME") + "/.ssh/id_rsa"
		}

		log.Printf("Creating SSH tunnel to %s@%s using key %s", username, sshAddress, sshKeyPath)

		// Create SSH tunnel
		tunnel, err = network.NewSSHTunnel(sshAddress, username, sshKeyPath, "")
		if err != nil {
			return nil, fmt.Errorf("failed to create SSH tunnel: %w", err)
		}

		log.Printf("✓ SSH tunnel established to %s", sshAddress)

		// Create custom HTTP client with tunnel
		customHTTPClient := &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					// For Docker over SSH, we connect to the remote Docker socket
					return tunnel.Dial("unix", "/var/run/docker.sock")
				},
			},
		}

		// Create Docker client with custom HTTP client
		dockerClient, err = dockerclient.NewClientWithOpts(
			dockerclient.WithHost("http://docker"),
			dockerclient.WithHTTPClient(customHTTPClient),
			dockerclient.WithAPIVersionNegotiation(),
		)
		if err != nil {
			tunnel.Close()
			return nil, fmt.Errorf("failed to create Docker client: %w", err)
		}
	} else {
		// Non-SSH connection (unix://, tcp://, or plain path)
		dockerHost := dockerSocket
		if !strings.Contains(dockerSocket, "://") {
			dockerHost = "unix://" + dockerSocket
		}

		// Set DOCKER_HOST for non-SSH connections
		os.Setenv("DOCKER_HOST", dockerHost)

		// Create standard Docker client
		var err error
		dockerClient, err = dockerclient.NewClientWithOpts(
			dockerclient.FromEnv,
			dockerclient.WithAPIVersionNegotiation(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create Docker client: %w", err)
		}
	}

	// Verify Docker connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := dockerClient.Ping(ctx)
	if err != nil {
		if tunnel != nil {
			tunnel.Close()
		}
		return nil, fmt.Errorf("failed to connect to Docker: %w", err)
	}

	return &Agent{
		apiURL:       strings.TrimSuffix(apiURL, "/"),
		hostID:       hostID,
		datacenter:   datacenter,
		dockerSocket: dockerSocket,
		docker:       dockerClient,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		sshTunnel:    tunnel,
		syncInterval: 30 * time.Second,
		authToken:    agentToken,
		httpPort:     httpPort,
	}, nil
}

// Close closes the agent and cleans up resources.
func (a *Agent) Close() error {
	if a.sshTunnel != nil {
		return a.sshTunnel.Close()
	}
	return nil
}

// Start starts the agent and begins monitoring Docker events.
func (a *Agent) Start(ctx context.Context) error {
	log.Printf("Agent started for host %s in datacenter %s", a.hostID, a.datacenter)
	log.Printf("Docker socket: %s", a.dockerSocket)
	log.Printf("API server: %s", a.apiURL)

	// Start HTTP server if configured
	if a.httpPort > 0 {
		if err := a.startHTTPServer(ctx, a.httpPort); err != nil {
			return fmt.Errorf("failed to start HTTP server: %w", err)
		}
	}

	// Verify authentication before proceeding
	if err := a.verifyAuthentication(ctx); err != nil {
		return fmt.Errorf("authentication failed: %w\n\nThe agent cannot operate without valid authentication.\nPlease check:\n  1. Agent token is configured in config.yaml\n  2. Token is valid and not expired\n  3. Server security.auth_enabled matches agent configuration", err)
	}

	// Register host with API server
	if err := a.registerHost(ctx); err != nil {
		log.Printf("Warning: Failed to register host: %v", err)
	}

	// Perform initial sync
	if err := a.syncContainers(ctx); err != nil {
		log.Printf("Warning: Initial sync failed: %v", err)
	}

	// Start periodic sync in background
	go a.periodicSync(ctx)

	// Start periodic metrics reporting
	go a.periodicMetricsReport(ctx)

	// Start task executor for deployment operations
	taskExecutor := NewTaskExecutor(a, 5*time.Second)
	go func() {
		if err := taskExecutor.Start(ctx); err != nil && err != context.Canceled {
			log.Printf("Task executor error: %v", err)
		}
	}()

	// Monitor Docker events
	return a.monitorEvents(ctx)
}

// verifyAuthentication tests if the agent can authenticate with the server.
// This prevents the agent from running if authentication will fail.
func (a *Agent) verifyAuthentication(ctx context.Context) error {
	// Test authentication with a simple API call
	url := fmt.Sprintf("%s/api/v1/stats", a.apiURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create auth test request: %w", err)
	}

	// Add auth token if configured
	if a.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+a.authToken)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to API server: %w", err)
	}
	defer resp.Body.Close()

	// Check for authentication errors
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		if a.authToken == "" {
			return fmt.Errorf("server requires authentication but no agent_token is configured")
		}
		return fmt.Errorf("authentication rejected (HTTP %d) - token may be invalid or expired", resp.StatusCode)
	}

	// Any 2xx response means we're authenticated
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Printf("✓ Authentication successful")
		return nil
	}

	// Other errors (5xx, etc.) - warn but don't fail
	log.Printf("Warning: API returned HTTP %d during auth check, but continuing anyway", resp.StatusCode)
	return nil
}

// registerHost registers this host with the API server.
func (a *Agent) registerHost(ctx context.Context) error {
	// Get Docker host info
	info, err := a.docker.Info(ctx)
	if err != nil {
		return fmt.Errorf("failed to get Docker info: %w", err)
	}

	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		hostname = a.hostID
	}

	// Create host model
	host := &models.Host{
		Context:    "https://schema.org",
		Type:       "ComputerSystem",
		ID:         a.hostID,
		Name:       hostname,
		IPAddress:  a.getHostIP(),
		CPU:        info.NCPU,
		Memory:     info.MemTotal,
		Status:     "active",
		Datacenter: a.datacenter,
	}

	a.hostInfo = host

	// Register with API
	url := fmt.Sprintf("%s/api/v1/hosts", a.apiURL)
	data, err := json.Marshal(host)
	if err != nil {
		return fmt.Errorf("failed to marshal host: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if a.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+a.authToken)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to register host: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to register host: %s - %s", resp.Status, string(body))
	}

	log.Printf("✓ Host registered: %s (%s)", hostname, a.hostID)
	return nil
}

// syncContainers discovers all containers and syncs them with the API.
func (a *Agent) syncContainers(ctx context.Context) error {
	// List all containers (including stopped ones)
	containers, err := a.docker.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	log.Printf("Discovered %d containers", len(containers))

	// Build set of container IDs that exist in Docker
	dockerContainerIDs := make(map[string]bool)
	for _, c := range containers {
		dockerContainerIDs[c.ID] = true
	}

	// Sync each container with rate limiting to avoid overwhelming the API
	for i, c := range containers {
		if err := a.syncContainer(ctx, c.ID); err != nil {
			log.Printf("Warning: Failed to sync container %s: %v", c.ID[:12], err)
		}

		// Add delay between syncs to respect rate limits (except for the last one)
		if i < len(containers)-1 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	// Clean up ignore list: remove entries for containers that no longer exist in Docker
	// This handles the edge case where the agent missed a "destroy" event
	a.cleanupIgnoreList(ctx, dockerContainerIDs)

	return nil
}

// syncContainer syncs a single container with the API server.
//
// This function implements a smart CREATE-or-UPDATE strategy:
//  1. First checks if container exists via GET request
//  2. If GET returns 200 OK: container exists, use PUT to UPDATE
//  3. If GET returns any other status (404, 401, etc.): use POST to CREATE
//
// This approach handles various scenarios:
//   - New containers that don't exist yet (404 → POST)
//   - Authentication issues on GET (401 → POST, which may succeed)
//   - Existing containers that need updates (200 → PUT)
//
// The function also handles the case where a container no longer exists in
// Docker (IsErrNotFound), which is normal when containers are removed.
//
// Rate limiting is handled by the caller (syncContainers) which adds delays
// between calls to this function.
func (a *Agent) syncContainer(ctx context.Context, containerID string) error {
	// Inspect container for full details
	inspect, err := a.docker.ContainerInspect(ctx, containerID)
	if err != nil {
		// Container no longer exists in Docker - this is normal when containers are removed
		if dockerclient.IsErrNotFound(err) {
			log.Printf("Container %s no longer exists in Docker, skipping sync", containerID[:12])
			return nil
		}
		return fmt.Errorf("failed to inspect container: %w", err)
	}

	// Convert to Graphium container model
	container := a.dockerToGraphium(inspect)

	// Check if this container is in the ignore list (user-deleted containers)
	ignoreURL := fmt.Sprintf("%s/api/v1/containers/%s/ignored", a.apiURL, container.ID)
	ignoreReq, err := http.NewRequestWithContext(ctx, "HEAD", ignoreURL, nil)
	if err != nil {
		log.Printf("Warning: Failed to create ignore check request for %s: %v", containerID[:12], err)
		// Continue with sync despite error (fail-open)
	} else {
		if a.authToken != "" {
			ignoreReq.Header.Set("Authorization", "Bearer "+a.authToken)
		}

		ignoreResp, err := a.httpClient.Do(ignoreReq)
		if err != nil {
			log.Printf("Warning: Failed to check ignore list for %s: %v", containerID[:12], err)
			// Continue with sync despite error (fail-open)
		} else {
			ignoreResp.Body.Close()

			// If container is ignored (200 OK), skip syncing
			if ignoreResp.StatusCode == http.StatusOK {
				log.Printf("Container %s is in ignore list, skipping sync", containerID[:12])
				return nil
			}
			// If 404 (not ignored), continue with normal sync below
		}
	}

	// Check if container already exists
	url := fmt.Sprintf("%s/api/v1/containers/%s", a.apiURL, container.ID)
	checkReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create check request: %w", err)
	}
	if a.authToken != "" {
		checkReq.Header.Set("Authorization", "Bearer "+a.authToken)
	}

	resp, err := a.httpClient.Do(checkReq)
	if err != nil {
		return fmt.Errorf("failed to check container: %w", err)
	}
	resp.Body.Close()

	var method string
	var endpoint string

	// Only try to UPDATE if we got a 200 OK (container exists)
	// For 404 (not found) or any error status (401, 403, 500, etc.), try to CREATE
	if resp.StatusCode == http.StatusOK {
		// Update existing container
		method = "PUT"
		endpoint = url
	} else {
		// Create new container (covers 404, 401, and other error cases)
		method = "POST"
		endpoint = fmt.Sprintf("%s/api/v1/containers", a.apiURL)
	}

	// Send to API
	data, err := json.Marshal(container)
	if err != nil {
		return fmt.Errorf("failed to marshal container: %w", err)
	}

	req, err := http.NewRequest(method, endpoint, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if a.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+a.authToken)
	}

	resp, err = a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to sync container: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	log.Printf("✓ Synced container: %s (%s)", inspect.Name, container.Status)
	return nil
}

// monitorEvents monitors Docker events and syncs changes in real-time.
func (a *Agent) monitorEvents(ctx context.Context) error {
	// Subscribe to Docker events
	eventFilter := filters.NewArgs()
	eventFilter.Add("type", string(events.ContainerEventType))

	eventsChan, errChan := a.docker.Events(ctx, events.ListOptions{
		Filters: eventFilter,
	})

	log.Printf("✓ Monitoring Docker events...")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errChan:
			if err != nil {
				log.Printf("Event stream error: %v", err)
				// Reconnect after short delay
				time.Sleep(5 * time.Second)
				eventsChan, errChan = a.docker.Events(ctx, events.ListOptions{
					Filters: eventFilter,
				})
			}
		case event := <-eventsChan:
			if event.Type == events.ContainerEventType {
				a.handleContainerEvent(ctx, event)
			}
		}
	}
}

// handleContainerEvent handles a Docker container event.
func (a *Agent) handleContainerEvent(ctx context.Context, event events.Message) {
	containerID := event.Actor.ID

	log.Printf("Docker event: %s - %s", event.Action, containerID[:12])

	switch event.Action {
	case "create", "start", "restart", "unpause":
		// On create, remove from ignore list in case container was recreated
		if event.Action == "create" {
			a.removeFromIgnoreList(containerID)
		}

		// Sync container state
		if err := a.syncContainer(ctx, containerID); err != nil {
			log.Printf("Failed to sync container: %v", err)
		}

	case "stop", "pause", "die", "kill":
		// Update container status
		if err := a.syncContainer(ctx, containerID); err != nil {
			log.Printf("Failed to update container: %v", err)
		}

	case "destroy", "remove":
		// Remove container from API
		url := fmt.Sprintf("%s/api/v1/containers/%s", a.apiURL, containerID)
		req, err := http.NewRequest("DELETE", url, nil)
		if err != nil {
			log.Printf("Failed to create delete request: %v", err)
			return
		}
		if a.authToken != "" {
			req.Header.Set("Authorization", "Bearer "+a.authToken)
		}

		resp, err := a.httpClient.Do(req)
		if err != nil {
			log.Printf("Failed to delete container: %v", err)
			return
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound {
			log.Printf("✓ Container removed: %s", containerID[:12])
		}

		// Clean up: remove from ignore list (container no longer exists)
		a.removeFromIgnoreList(containerID)
	}
}

// removeFromIgnoreList removes a container from the ignore list.
// This is called when a container is recreated or destroyed.
func (a *Agent) removeFromIgnoreList(containerID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/api/v1/containers/%s/ignored", a.apiURL, containerID)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		log.Printf("Warning: Failed to create ignore list removal request for %s: %v", containerID[:12], err)
		return
	}

	if a.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+a.authToken)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		log.Printf("Warning: Failed to remove %s from ignore list: %v", containerID[:12], err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		log.Printf("✓ Container %s removed from ignore list", containerID[:12])
	}
	// Silently ignore 404 or other errors - container may not be in ignore list
}

// cleanupIgnoreList removes stale entries from the ignore list.
// An ignore list entry is considered stale if the container no longer exists in Docker.
// This handles the edge case where the agent missed a "destroy" event or was offline.
func (a *Agent) cleanupIgnoreList(ctx context.Context, dockerContainerIDs map[string]bool) {
	// Fetch current ignore list from API
	url := fmt.Sprintf("%s/api/v1/containers/ignored", a.apiURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.Printf("Warning: Failed to create ignore list fetch request: %v", err)
		return
	}

	if a.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+a.authToken)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		log.Printf("Warning: Failed to fetch ignore list: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Ignore list endpoint may not exist yet or auth issue
		return
	}

	// Parse ignore list response
	var ignoreList []struct {
		ContainerID string `json:"container_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ignoreList); err != nil {
		log.Printf("Warning: Failed to decode ignore list: %v", err)
		return
	}

	// Check each ignored container and remove if not in Docker
	cleanedCount := 0
	for _, entry := range ignoreList {
		if !dockerContainerIDs[entry.ContainerID] {
			// Container doesn't exist in Docker, remove from ignore list
			log.Printf("Cleaning up stale ignore list entry for %s (not in Docker)", entry.ContainerID[:12])
			a.removeFromIgnoreList(entry.ContainerID)
			cleanedCount++
		}
	}

	if cleanedCount > 0 {
		log.Printf("✓ Cleaned up %d stale ignore list entries", cleanedCount)
	}
}

// periodicSync performs periodic full synchronization.
func (a *Agent) periodicSync(ctx context.Context) {
	ticker := time.NewTicker(a.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log.Printf("Running periodic sync...")
			if err := a.syncContainers(ctx); err != nil {
				log.Printf("Periodic sync error: %v", err)
			}
		}
	}
}

// periodicMetricsReport collects and reports system metrics periodically.
func (a *Agent) periodicMetricsReport(ctx context.Context) {
	ticker := time.NewTicker(a.syncInterval)
	defer ticker.Stop()

	// Report immediately on start
	if err := a.reportMetrics(ctx); err != nil {
		log.Printf("Warning: Initial metrics report failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := a.reportMetrics(ctx); err != nil {
				log.Printf("Metrics report error: %v", err)
			}
		}
	}
}

// reportMetrics collects system metrics and sends them to the API server.
func (a *Agent) reportMetrics(ctx context.Context) error {
	// Collect system metrics
	metrics, err := GetSystemMetrics()
	if err != nil {
		return fmt.Errorf("failed to collect metrics: %w", err)
	}

	// Create metrics update payload
	update := map[string]interface{}{
		"cpuUsage":           metrics.CPUUsage,
		"memoryUsage":        metrics.MemoryUsage,
		"memoryUsagePercent": metrics.MemoryUsagePercent,
		"lastMetricsUpdate":  metrics.Timestamp.Format(time.RFC3339),
	}

	data, err := json.Marshal(update)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	// Send metrics to API
	url := fmt.Sprintf("%s/api/v1/hosts/%s/metrics", a.apiURL, a.hostID)
	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if a.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+a.authToken)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send metrics: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("metrics update failed: %s - %s", resp.Status, string(body))
	}

	log.Printf("✓ Metrics reported: CPU %.1f%%, Memory %.1f%% (%d MB)",
		metrics.CPUUsage,
		metrics.MemoryUsagePercent,
		metrics.MemoryUsage/(1024*1024))
	return nil
}

// dockerToGraphium converts a Docker container to Graphium container model.
func (a *Agent) dockerToGraphium(inspect types.ContainerJSON) *models.Container {
	// Map Docker state to Graphium status
	var status string
	if inspect.State.Running {
		status = "running"
	} else if inspect.State.Paused {
		status = "paused"
	} else if inspect.State.Restarting {
		status = "restarting"
	} else if inspect.State.Dead {
		status = "exited"
	} else {
		status = "stopped"
	}

	// Extract ports
	ports := []models.Port{}
	for port, bindings := range inspect.HostConfig.PortBindings {
		for _, binding := range bindings {
			var hostPort int
			if _, err := fmt.Sscanf(binding.HostPort, "%d", &hostPort); err != nil {
				continue // Skip invalid port format
			}

			var containerPort int
			var protocol string
			parts := strings.Split(string(port), "/")
			if len(parts) == 2 {
				if _, err := fmt.Sscanf(parts[0], "%d", &containerPort); err != nil {
					continue // Skip invalid port format
				}
				protocol = parts[1]
			}

			if hostPort > 0 && containerPort > 0 {
				ports = append(ports, models.Port{
					HostPort:      hostPort,
					ContainerPort: containerPort,
					Protocol:      protocol,
				})
			}
		}
	}

	// Extract environment variables
	env := make(map[string]string)
	for _, e := range inspect.Config.Env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}

	// Clean container name (remove leading /)
	name := strings.TrimPrefix(inspect.Name, "/")

	return &models.Container{
		Context:  "https://schema.org",
		Type:     "SoftwareApplication",
		ID:       inspect.ID,
		Name:     name,
		Image:    inspect.Config.Image,
		Status:   status,
		HostedOn: a.hostID,
		Ports:    ports,
		Env:      env,
		Created:  inspect.Created,
	}
}

// getHostIP attempts to determine the host's IP address.
// For local Docker socket connections, returns "localhost" to enable Unix socket usage.
// For remote connections, returns the first non-loopback IPv4 address.
func (a *Agent) getHostIP() string {
	// Check if using local Docker socket
	dockerSocket := a.dockerSocket
	if dockerSocket == "" {
		dockerSocket = "/var/run/docker.sock"
	}

	// If using Unix socket, this is localhost
	if strings.HasPrefix(dockerSocket, "unix://") || strings.HasPrefix(dockerSocket, "/") {
		return "localhost"
	}

	// Try to get actual IP address from network interfaces
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Printf("Warning: Failed to get network interfaces: %v", err)
		return "127.0.0.1"
	}

	// Find first non-loopback IPv4 address
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}

	// Fallback to localhost if no external IP found
	return "localhost"
}
