package orchestration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDockerClientManager_AddHost(t *testing.T) {
	manager := NewDockerClientManager()
	defer manager.Close()

	// Test adding a local host
	// Note: This requires Docker to be running locally
	err := manager.AddHost("local", "unix:///var/run/docker.sock")
	if err != nil {
		t.Skip("Docker not available locally, skipping test")
	}

	// Verify host was added
	assert.True(t, manager.HasHost("local"))
	assert.Equal(t, 1, manager.Count())

	// Get client
	cli, err := manager.GetClient("local")
	require.NoError(t, err)
	assert.NotNil(t, cli)
}

func TestDockerClientManager_AddHostInvalidSocket(t *testing.T) {
	manager := NewDockerClientManager()
	defer manager.Close()

	// Test adding a host with invalid socket
	err := manager.AddHost("invalid", "tcp://invalid-host:2375")
	assert.Error(t, err)

	// Verify host was not added
	assert.False(t, manager.HasHost("invalid"))
	assert.Equal(t, 0, manager.Count())
}

func TestDockerClientManager_RemoveHost(t *testing.T) {
	manager := NewDockerClientManager()
	defer manager.Close()

	// Add a host
	err := manager.AddHost("local", "unix:///var/run/docker.sock")
	if err != nil {
		t.Skip("Docker not available locally, skipping test")
	}

	// Remove the host
	err = manager.RemoveHost("local")
	require.NoError(t, err)

	// Verify host was removed
	assert.False(t, manager.HasHost("local"))
	assert.Equal(t, 0, manager.Count())
}

func TestDockerClientManager_RemoveHostNotFound(t *testing.T) {
	manager := NewDockerClientManager()
	defer manager.Close()

	// Try to remove non-existent host
	err := manager.RemoveHost("nonexistent")
	assert.Error(t, err)
}

func TestDockerClientManager_GetClientNotFound(t *testing.T) {
	manager := NewDockerClientManager()
	defer manager.Close()

	// Try to get non-existent client
	cli, err := manager.GetClient("nonexistent")
	assert.Error(t, err)
	assert.Nil(t, cli)
}

func TestDockerClientManager_ListHosts(t *testing.T) {
	manager := NewDockerClientManager()
	defer manager.Close()

	// Initially empty
	hosts := manager.ListHosts()
	assert.Empty(t, hosts)

	// Add a host
	err := manager.AddHost("local", "unix:///var/run/docker.sock")
	if err != nil {
		t.Skip("Docker not available locally, skipping test")
	}

	// List should contain the host
	hosts = manager.ListHosts()
	assert.Len(t, hosts, 1)
	assert.Contains(t, hosts, "local")
}

func TestDockerClientManager_Close(t *testing.T) {
	manager := NewDockerClientManager()

	// Add a host
	err := manager.AddHost("local", "unix:///var/run/docker.sock")
	if err != nil {
		t.Skip("Docker not available locally, skipping test")
	}

	// Close manager
	err = manager.Close()
	require.NoError(t, err)

	// Verify all clients are closed
	assert.Equal(t, 0, manager.Count())
}
