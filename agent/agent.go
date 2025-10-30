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
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	dockerclient "github.com/docker/docker/client"

	"evalgo.org/graphium/models"
)

// Agent manages Docker container discovery and synchronization with the API server.
// It monitors the Docker daemon for container events and automatically syncs changes
// to the Graphium API, providing real-time visibility into container state across
// distributed hosts.
type Agent struct {
	apiURL       string
	hostID       string
	datacenter   string
	dockerSocket string
	docker       *dockerclient.Client
	httpClient   *http.Client
	syncInterval time.Duration
	hostInfo     *models.Host
	authToken    string
}

// NewAgent creates a new agent instance.
func NewAgent(apiURL, hostID, datacenter, dockerSocket, agentToken string) (*Agent, error) {
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

	// Create Docker client
	dockerClient, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithHost("unix://"+dockerSocket),
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	// Verify Docker connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = dockerClient.Ping(ctx)
	if err != nil {
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
		syncInterval: 30 * time.Second,
		authToken:    agentToken,
	}, nil
}

// Start starts the agent and begins monitoring Docker events.
func (a *Agent) Start(ctx context.Context) error {
	log.Printf("Agent started for host %s in datacenter %s", a.hostID, a.datacenter)
	log.Printf("Docker socket: %s", a.dockerSocket)
	log.Printf("API server: %s", a.apiURL)

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

	// Monitor Docker events
	return a.monitorEvents(ctx)
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
//  - New containers that don't exist yet (404 → POST)
//  - Authentication issues on GET (401 → POST, which may succeed)
//  - Existing containers that need updates (200 → PUT)
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
func (a *Agent) getHostIP() string {
	// Try to get IP from hostname
	hostname, err := os.Hostname()
	if err != nil {
		return "127.0.0.1"
	}

	// For now, just return a placeholder
	// In production, you might want to query network interfaces
	return fmt.Sprintf("host-%s", hostname)
}
