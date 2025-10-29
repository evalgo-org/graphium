//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"testing"

	"evalgo.org/graphium/internal/config"
	"evalgo.org/graphium/internal/storage"
	"evalgo.org/graphium/models"
	evetesting "eve.evalgo.org/containers/testing"
	"github.com/stretchr/testify/require"
)

// TestCouchDBIntegration tests storage operations with a real CouchDB container.
//
// This test demonstrates using EVE's container support to replace test mocks
// with real containerized services. The CouchDB container is automatically
// created, started, and cleaned up by testcontainers-go.
func TestCouchDBIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx := context.Background()

	// Start CouchDB container using EVE
	couchURL, cleanup, err := evetesting.SetupCouchDB(ctx, t, nil)
	require.NoError(t, err, "Failed to start CouchDB container")
	defer cleanup()

	t.Logf("CouchDB container started at: %s", couchURL)

	// Configure Graphium to use containerized CouchDB
	cfg := &config.Config{
		CouchDB: config.CouchDBConfig{
			URL:      couchURL,
			Database: "graphium_test",
			Username: "admin",
			Password: "password",
		},
	}

	// Initialize storage with real CouchDB
	store, err := storage.New(cfg)
	require.NoError(t, err, "Failed to initialize storage")
	defer store.Close()

	t.Run("Host CRUD Operations", func(t *testing.T) {
		// Test CREATE
		host := &models.Host{
			Context:    "https://schema.org",
			Type:       "ComputerSystem",
			ID:         "test-host-001",
			Name:       "integration-test-server",
			IPAddress:  "192.168.1.100",
			Status:     "active",
			CPU:        8,
			Memory:     16777216,
			Datacenter: "us-west-2",
		}

		err := store.SaveHost(host)
		require.NoError(t, err, "Failed to save host")

		// Test READ
		retrieved, err := store.GetHost(host.ID)
		require.NoError(t, err, "Failed to retrieve host")
		require.Equal(t, host.ID, retrieved.ID)
		require.Equal(t, host.Name, retrieved.Name)
		require.Equal(t, host.IPAddress, retrieved.IPAddress)
		require.Equal(t, host.Status, retrieved.Status)

		// Test UPDATE
		retrieved.Status = "inactive"
		err = store.SaveHost(retrieved)
		require.NoError(t, err, "Failed to update host")

		updated, err := store.GetHost(host.ID)
		require.NoError(t, err, "Failed to retrieve updated host")
		require.Equal(t, "inactive", updated.Status)

		// Test LIST
		hosts, err := store.ListHosts(map[string]interface{}{
			"@type": "ComputerSystem",
		})
		require.NoError(t, err, "Failed to list hosts")
		require.GreaterOrEqual(t, len(hosts), 1)

		// Test DELETE
		err = store.DeleteHost(updated.ID, updated.Rev)
		require.NoError(t, err, "Failed to delete host")

		// Verify deletion
		_, err = store.GetHost(host.ID)
		require.Error(t, err, "Expected error when retrieving deleted host")
	})

	t.Run("Container CRUD Operations", func(t *testing.T) {
		// First create a host
		host := &models.Host{
			Context:   "https://schema.org",
			Type:      "ComputerSystem",
			ID:        "test-host-002",
			Name:      "container-test-host",
			IPAddress: "192.168.1.101",
			Status:    "active",
		}
		err := store.SaveHost(host)
		require.NoError(t, err)

		// Test CREATE container
		container := &models.Container{
			Context:  "https://schema.org",
			Type:     "SoftwareApplication",
			ID:       "test-container-001",
			Name:     "test-nginx",
			Image:    "nginx:latest",
			Status:   "running",
			HostedOn: host.ID,
			Ports: []models.Port{
				{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
			},
		}

		err = store.SaveContainer(container)
		require.NoError(t, err, "Failed to save container")

		// Test READ
		retrieved, err := store.GetContainer(container.ID)
		require.NoError(t, err, "Failed to retrieve container")
		require.Equal(t, container.ID, retrieved.ID)
		require.Equal(t, container.Name, retrieved.Name)
		require.Equal(t, container.Image, retrieved.Image)

		// Test UPDATE
		retrieved.Status = "stopped"
		err = store.SaveContainer(retrieved)
		require.NoError(t, err, "Failed to update container")

		updated, err := store.GetContainer(container.ID)
		require.NoError(t, err, "Failed to retrieve updated container")
		require.Equal(t, "stopped", updated.Status)

		// Test LIST
		containers, err := store.ListContainers(map[string]interface{}{
			"@type": "SoftwareApplication",
		})
		require.NoError(t, err, "Failed to list containers")
		require.GreaterOrEqual(t, len(containers), 1)

		// Test DELETE
		err = store.DeleteContainer(updated.ID, updated.Rev)
		require.NoError(t, err, "Failed to delete container")

		// Cleanup host
		host, _ = store.GetHost(host.ID)
		err = store.DeleteHost(host.ID, host.Rev)
		require.NoError(t, err)
	})

	t.Run("Concurrent Operations", func(t *testing.T) {
		// Test concurrent writes to ensure CouchDB handles them correctly
		const numGoroutines = 5

		done := make(chan bool, numGoroutines)
		for i := 0; i < numGoroutines; i++ {
			go func(index int) {
				host := &models.Host{
					Context:   "https://schema.org",
					Type:      "ComputerSystem",
					ID:        fmt.Sprintf("concurrent-host-%03d", index),
					Name:      fmt.Sprintf("concurrent-test-%d", index),
					IPAddress: fmt.Sprintf("192.168.2.%d", index+1),
					Status:    "active",
				}
				err := store.SaveHost(host)
				require.NoError(t, err)
				done <- true
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < numGoroutines; i++ {
			<-done
		}

		// Verify all hosts were created
		hosts, err := store.ListHosts(map[string]interface{}{
			"@type": "ComputerSystem",
		})
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(hosts), numGoroutines)

		// Cleanup
		for i := 0; i < numGoroutines; i++ {
			hostID := fmt.Sprintf("concurrent-host-%03d", i)
			if h, err := store.GetHost(hostID); err == nil {
				_ = store.DeleteHost(h.ID, h.Rev)
			}
		}
	})

	t.Logf("✅ All CouchDB integration tests passed with real container")
}
