package integration

import (
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"evalgo.org/graphium/internal/config"
	"evalgo.org/graphium/internal/orchestration"
	"evalgo.org/graphium/internal/storage"
	"evalgo.org/graphium/models"
	evetesting "eve.evalgo.org/containers/testing"
	"eve.evalgo.org/containers/stacks"
)

// TestDistributedStackDeployment tests complete distributed stack deployment.
// This test uses a local Docker daemon but simulates multi-host deployment.
func TestDistributedStackDeployment(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// Setup CouchDB for storage
	couchURL, cleanup, err := evetesting.SetupCouchDB(ctx, t, nil)
	require.NoError(t, err, "Failed to start CouchDB container")
	defer cleanup()

	// Wait for CouchDB to be ready
	time.Sleep(2 * time.Second)

	// Create config
	cfg := &config.Config{
		CouchDB: config.CouchDBConfig{
			URL:      couchURL,
			Database: "graphium_distributed_test",
			Username: "admin",
			Password: "password",
		},
	}

	// Initialize storage
	store, err := storage.New(cfg)
	require.NoError(t, err, "Failed to initialize storage")
	defer store.Close()

	// Create test hosts (simulating multiple hosts with same Docker daemon)
	hosts := []*models.Host{
		{
			Context:    "https://schema.org",
			Type:       "ComputerSystem",
			ID:         "test-host-1",
			Name:       "Test Host 1",
			IPAddress:  "127.0.0.1",
			CPU:        4,
			Memory:     8589934592, // 8GB
			Status:     "active",
			Datacenter: "test-dc",
		},
		{
			Context:    "https://schema.org",
			Type:       "ComputerSystem",
			ID:         "test-host-2",
			Name:       "Test Host 2",
			IPAddress:  "127.0.0.1",
			CPU:        8,
			Memory:     17179869184, // 16GB
			Status:     "active",
			Datacenter: "test-dc",
		},
	}

	// Save hosts to storage
	for _, host := range hosts {
		err := store.SaveHost(host)
		require.NoError(t, err, "Failed to save host")
	}

	// Create test stack definition
	stackDef := &stacks.Stack{
		Context: "https://schema.org",
		Type:    "ItemList",
		Name:    "test-stack",
		ItemListElement: []stacks.StackItemElement{
			{
				Type:  "SoftwareApplication",
				Name:  "redis",
				Image: "redis:7-alpine",
				Ports: []stacks.PortMapping{
					{ContainerPort: 6379, HostPort: 6379},
				},
			},
			{
				Type:  "SoftwareApplication",
				Name:  "nginx",
				Image: "nginx:alpine",
				Ports: []stacks.PortMapping{
					{ContainerPort: 80, HostPort: 8080},
				},
			},
		},
	}

	// Create stack model
	stack := &models.Stack{
		Context:     "https://schema.org",
		Type:        "ItemList",
		ID:          "test-stack-1",
		Name:        "test-stack",
		Description: "Integration test stack",
		Status:      "pending",
		Datacenter:  "test-dc",
		Deployment: models.DeploymentConfig{
			Mode:              "multi-host",
			PlacementStrategy: "spread",
			NetworkMode:       "host-port",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Owner:     "test",
	}

	// Save stack
	err = store.SaveStack(stack)
	require.NoError(t, err, "Failed to save stack")

	// Create orchestrator
	orch := orchestration.NewDistributedStackOrchestrator(store)
	defer orch.Close()

	// Prepare host info (using local Docker daemon for all "hosts")
	hostInfos := []*models.HostInfo{
		{
			Host:         hosts[0],
			DockerSocket: "unix:///var/run/docker.sock",
			CurrentLoad: models.ResourceLoad{
				CPUUsage:       20.0,
				MemoryUsage:    2147483648, // 2GB
				ContainerCount: 2,
			},
			AvailableResources: models.Resources{
				CPU:    4,
				Memory: 8589934592,
			},
			Labels: make(map[string]string),
		},
		{
			Host:         hosts[1],
			DockerSocket: "unix:///var/run/docker.sock",
			CurrentLoad: models.ResourceLoad{
				CPUUsage:       15.0,
				MemoryUsage:    3221225472, // 3GB
				ContainerCount: 1,
			},
			AvailableResources: models.Resources{
				CPU:    8,
				Memory: 17179869184,
			},
			Labels: make(map[string]string),
		},
	}

	// Register hosts
	for _, hostInfo := range hostInfos {
		err := orch.RegisterHost(hostInfo.Host, hostInfo.DockerSocket)
		require.NoError(t, err, "Failed to register host %s", hostInfo.Host.ID)
	}

	// Pull required images
	t.Log("Pulling required images...")
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	require.NoError(t, err, "Failed to create Docker client")
	defer cli.Close()

	images := []string{"redis:7-alpine", "nginx:alpine"}
	for _, img := range images {
		t.Logf("Pulling image: %s", img)
		reader, err := cli.ImagePull(ctx, img, image.PullOptions{})
		require.NoError(t, err, "Failed to pull image %s", img)
		// Drain the reader to complete the pull
		_, _ = io.ReadAll(reader)
		reader.Close()
	}

	// Deploy stack
	t.Log("Deploying stack...")
	deployment, err := orch.DeployStack(ctx, stack, stackDef, hostInfos)
	require.NoError(t, err, "Failed to deploy stack")

	// Verify deployment
	assert.Equal(t, "test-stack-1", deployment.StackID)
	assert.Equal(t, "completed", deployment.Status)
	assert.Len(t, deployment.Placements, 2, "Should have 2 container placements")

	// Verify placements
	t.Log("Verifying placements...")
	for name, placement := range deployment.Placements {
		t.Logf("Container %s: host=%s, containerID=%s, status=%s",
			name, placement.HostID, placement.ContainerID, placement.Status)

		assert.NotEmpty(t, placement.ContainerID, "Container ID should not be empty")
		assert.Contains(t, []string{"test-host-1", "test-host-2"}, placement.HostID)
		assert.Equal(t, "127.0.0.1", placement.IPAddress)
	}

	// Verify network config
	assert.Equal(t, "host-port", deployment.NetworkConfig.Mode)
	assert.Len(t, deployment.NetworkConfig.ServiceEndpoints, 2, "Should have 2 service endpoints")

	// Verify service endpoints
	redisEndpoint, ok := deployment.NetworkConfig.ServiceEndpoints["redis"]
	assert.True(t, ok, "Should have redis endpoint")
	assert.Contains(t, redisEndpoint, "127.0.0.1:6379")

	nginxEndpoint, ok := deployment.NetworkConfig.ServiceEndpoints["nginx"]
	assert.True(t, ok, "Should have nginx endpoint")
	assert.Contains(t, nginxEndpoint, "127.0.0.1:8080")

	// Verify stack status was updated
	updatedStack, err := store.GetStack(stack.ID)
	require.NoError(t, err, "Failed to get updated stack")
	assert.Equal(t, "running", updatedStack.Status)
	assert.NotNil(t, updatedStack.DeployedAt)
	assert.Empty(t, updatedStack.ErrorMessage)

	// Stop stack
	t.Log("Stopping stack...")
	err = orch.StopStack(ctx, stack.ID)
	require.NoError(t, err, "Failed to stop stack")

	// Verify stack status
	stoppedStack, err := store.GetStack(stack.ID)
	require.NoError(t, err, "Failed to get stopped stack")
	assert.Equal(t, "stopped", stoppedStack.Status)

	// Remove stack
	t.Log("Removing stack...")
	err = orch.RemoveStack(ctx, stack.ID, false)
	require.NoError(t, err, "Failed to remove stack")

	// Verify stack was deleted
	_, err = store.GetStack(stack.ID)
	assert.Error(t, err, "Stack should be deleted")

	t.Log("✓ Distributed deployment test completed successfully")
}

// TestPlacementStrategies tests different placement strategies.
func TestPlacementStrategies(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	testCases := []struct {
		name     string
		strategy string
	}{
		{"Auto Placement", "auto"},
		{"Spread Placement", "spread"},
		{"Datacenter Placement", "datacenter"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			// Setup CouchDB
			couchURL, cleanup, err := evetesting.SetupCouchDB(ctx, t, nil)
			require.NoError(t, err)
			defer cleanup()

			time.Sleep(2 * time.Second)

			// Create config
			cfg := &config.Config{
				CouchDB: config.CouchDBConfig{
					URL:      couchURL,
					Database: "graphium_placement_test",
					Username: "admin",
					Password: "password",
				},
			}

			// Initialize storage
			store, err := storage.New(cfg)
			require.NoError(t, err)
			defer store.Close()

			// Create test hosts
			hosts := []*models.HostInfo{
				{
					Host: &models.Host{
						ID:         "host1",
						Name:       "Host 1",
						IPAddress:  "127.0.0.1",
						CPU:        4,
						Memory:     8589934592,
						Status:     "active",
						Datacenter: "test-dc",
					},
					DockerSocket: "unix:///var/run/docker.sock",
					CurrentLoad: models.ResourceLoad{
						CPUUsage:       20.0,
						MemoryUsage:    2147483648,
						ContainerCount: 2,
					},
					AvailableResources: models.Resources{
						CPU:    4,
						Memory: 8589934592,
					},
				},
				{
					Host: &models.Host{
						ID:         "host2",
						Name:       "Host 2",
						IPAddress:  "127.0.0.1",
						CPU:        8,
						Memory:     17179869184,
						Status:     "active",
						Datacenter: "test-dc",
					},
					DockerSocket: "unix:///var/run/docker.sock",
					CurrentLoad: models.ResourceLoad{
						CPUUsage:       15.0,
						MemoryUsage:    3221225472,
						ContainerCount: 1,
					},
					AvailableResources: models.Resources{
						CPU:    8,
						Memory: 17179869184,
					},
				},
			}

			// Create stack definition
			stackDef := &stacks.Stack{
				Context: "https://schema.org",
				Type:    "ItemList",
				Name:    "test-stack",
				ItemListElement: []stacks.StackItemElement{
					{
						Type:  "SoftwareApplication",
						Name:  "web",
						Image: "nginx:alpine",
					},
				},
			}

			// Create stack model
			stack := &models.Stack{
				Context:    "https://schema.org",
				Type:       "ItemList",
				ID:         "test-stack-placement",
				Name:       "test-stack",
				Status:     "pending",
				Datacenter: "test-dc",
				Deployment: models.DeploymentConfig{
					Mode:              "multi-host",
					PlacementStrategy: tc.strategy,
					NetworkMode:       "host-port",
				},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			// Get placement strategy
			var strategy orchestration.PlacementStrategy
			switch tc.strategy {
			case "auto":
				strategy = &orchestration.AutoPlacementStrategy{}
			case "spread":
				strategy = &orchestration.SpreadPlacementStrategy{}
			case "datacenter":
				strategy = &orchestration.DatacenterPlacementStrategy{}
			default:
				t.Fatalf("Unknown strategy: %s", tc.strategy)
			}

			// Test placement
			placements, err := strategy.PlaceContainers(ctx, stack, stackDef, hosts)
			require.NoError(t, err, "Placement should succeed")

			// Verify placement
			assert.Len(t, placements, 1, "Should place 1 container")
			assert.Contains(t, []string{"host1", "host2"}, placements["web"])

			t.Logf("✓ %s strategy placed container on %s", tc.strategy, placements["web"])
		})
	}
}

// TestMain sets up test environment
func TestMain(m *testing.M) {
	// Run tests
	code := m.Run()
	os.Exit(code)
}
