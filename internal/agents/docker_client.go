package agents

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"eve.evalgo.org/network"
	dockerclient "github.com/docker/docker/client"
)

// GetDockerClient returns a Docker client for the specified host.
//
// For remote hosts with running agents, it creates and caches a Docker client
// that connects via SSH tunnel. The SSH tunnel and client are cached and reused
// for subsequent requests to avoid the overhead of creating new connections.
//
// For local Docker or when no agent is configured, it returns a local Docker client.
//
// The returned client must NOT be closed by the caller - the manager handles cleanup.
func (m *Manager) GetDockerClient(hostID string) (*dockerclient.Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find agent by hostID (not by config ID)
	var agent *AgentProcess
	for _, a := range m.agents {
		if a.Config.HostID == hostID && a.State.Status == "running" {
			agent = a
			break
		}
	}

	// No running agent for this host - try local Docker
	if agent == nil {
		return m.getLocalDockerClient()
	}

	// Check if we already have a cached Docker client for this agent
	if agent.DockerClient != nil {
		return agent.DockerClient, nil
	}

	// Create new Docker client for this agent
	dockerSocket := agent.Config.DockerSocket
	if dockerSocket == "" {
		dockerSocket = "/var/run/docker.sock"
	}

	// Check if using SSH connection
	if strings.HasPrefix(dockerSocket, "ssh://") {
		client, tunnel, err := m.createSSHDockerClient(dockerSocket, agent.Config.SSHKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create SSH Docker client: %w", err)
		}

		// Cache the client and tunnel for reuse
		agent.DockerClient = client
		agent.SSHTunnel = tunnel

		return client, nil
	}

	// For non-SSH sockets, create standard client
	dockerHost := dockerSocket
	if !strings.Contains(dockerSocket, "://") {
		dockerHost = "unix://" + dockerSocket
	}

	client, err := dockerclient.NewClientWithOpts(
		dockerclient.WithHost(dockerHost),
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	// Cache the client
	agent.DockerClient = client

	return client, nil
}

// getLocalDockerClient returns a client for the local Docker daemon.
func (m *Manager) getLocalDockerClient() (*dockerclient.Client, error) {
	client, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create local Docker client: %w", err)
	}
	return client, nil
}

// createSSHDockerClient creates a Docker client that connects via SSH tunnel.
func (m *Manager) createSSHDockerClient(dockerSocket, sshKeyPath string) (*dockerclient.Client, *network.SSHTunnel, error) {
	// Parse SSH URL: ssh://user@host:port
	u, err := url.Parse(dockerSocket)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse SSH URL: %w", err)
	}

	username := u.User.Username()
	if username == "" {
		return nil, nil, fmt.Errorf("SSH URL must include username (e.g., ssh://user@host)")
	}

	host := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "22"
	}
	sshAddress := net.JoinHostPort(host, port)

	// Determine SSH key path
	if sshKeyPath == "" {
		sshKeyPath = os.Getenv("DOCKER_SSH_IDENTITY")
		if sshKeyPath == "" {
			sshKeyPath = os.Getenv("HOME") + "/.ssh/id_rsa"
		}
	}

	// Create SSH tunnel
	tunnel, err := network.NewSSHTunnel(sshAddress, username, sshKeyPath, "")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create SSH tunnel: %w", err)
	}

	// Create custom HTTP client with tunnel
	customHTTPClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return tunnel.Dial("unix", "/var/run/docker.sock")
			},
		},
	}

	// Create Docker client
	client, err := dockerclient.NewClientWithOpts(
		dockerclient.WithHost("http://docker"),
		dockerclient.WithHTTPClient(customHTTPClient),
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		_ = tunnel.Close()
		return nil, nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 10)
	defer cancel()
	if _, err := client.Ping(ctx); err != nil {
		_ = client.Close()
		_ = tunnel.Close()
		return nil, nil, fmt.Errorf("failed to connect to Docker via SSH: %w", err)
	}

	return client, tunnel, nil
}

// closeDockerClient closes the Docker client and SSH tunnel for an agent.
// This is called when an agent is stopped.
func (m *Manager) closeDockerClient(agent *AgentProcess) {
	if agent.DockerClient != nil {
		_ = agent.DockerClient.Close()
		agent.DockerClient = nil
	}
	if agent.SSHTunnel != nil {
		_ = agent.SSHTunnel.Close()
		agent.SSHTunnel = nil
	}
}
