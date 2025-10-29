package orchestration

import (
	"context"
	"fmt"
	"sync"

	"github.com/docker/docker/client"
)

// DockerClientManager manages Docker clients for multiple hosts.
// It maintains a pool of Docker clients, one per host, and handles
// connection lifecycle.
//
// Thread-safe for concurrent access.
type DockerClientManager struct {
	clients map[string]*client.Client
	mu      sync.RWMutex
}

// NewDockerClientManager creates a new client manager.
func NewDockerClientManager() *DockerClientManager {
	return &DockerClientManager{
		clients: make(map[string]*client.Client),
	}
}

// AddHost adds or updates a Docker client for a specific host.
//
// dockerHost format examples:
//   - unix:///var/run/docker.sock (local Unix socket)
//   - tcp://192.168.1.10:2375 (remote TCP)
//   - ssh://user@192.168.1.10 (SSH tunnel)
//
// Returns an error if the client cannot be created or if connection fails.
func (m *DockerClientManager) AddHost(hostID, dockerHost string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Close existing client if present
	if existingClient, ok := m.clients[hostID]; ok {
		existingClient.Close()
		delete(m.clients, hostID)
	}

	// Create new client
	cli, err := client.NewClientWithOpts(
		client.WithHost(dockerHost),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return fmt.Errorf("failed to create Docker client for host %s: %w", hostID, err)
	}

	// Verify connection by pinging Docker daemon
	ctx := context.Background()
	if _, err := cli.Ping(ctx); err != nil {
		cli.Close()
		return fmt.Errorf("failed to connect to Docker daemon on host %s: %w", hostID, err)
	}

	m.clients[hostID] = cli
	return nil
}

// GetClient returns the Docker client for a specific host.
// Returns an error if the host is not registered.
func (m *DockerClientManager) GetClient(hostID string) (*client.Client, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cli, ok := m.clients[hostID]
	if !ok {
		return nil, fmt.Errorf("no Docker client registered for host %s", hostID)
	}

	return cli, nil
}

// RemoveHost removes a host and closes its Docker client.
// Returns an error if the host is not found.
func (m *DockerClientManager) RemoveHost(hostID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cli, ok := m.clients[hostID]
	if !ok {
		return fmt.Errorf("host %s not found", hostID)
	}

	cli.Close()
	delete(m.clients, hostID)
	return nil
}

// ListHosts returns a list of all registered host IDs.
func (m *DockerClientManager) ListHosts() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	hosts := make([]string, 0, len(m.clients))
	for hostID := range m.clients {
		hosts = append(hosts, hostID)
	}
	return hosts
}

// Close closes all Docker clients and clears the manager.
func (m *DockerClientManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error
	for hostID, cli := range m.clients {
		if err := cli.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close client for host %s: %w", hostID, err))
		}
	}

	m.clients = make(map[string]*client.Client)

	if len(errs) > 0 {
		return fmt.Errorf("errors closing clients: %v", errs)
	}

	return nil
}

// HasHost checks if a host is registered.
func (m *DockerClientManager) HasHost(hostID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, ok := m.clients[hostID]
	return ok
}

// Count returns the number of registered hosts.
func (m *DockerClientManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.clients)
}
