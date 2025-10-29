package orchestration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"evalgo.org/graphium/models"
	"eve.evalgo.org/containers/stacks"
)

func createTestHosts() []*models.HostInfo {
	return []*models.HostInfo{
		{
			Host: &models.Host{
				ID:         "host1",
				Name:       "Host 1",
				IPAddress:  "192.168.1.10",
				CPU:        4,
				Memory:     8589934592, // 8GB
				Status:     "active",
				Datacenter: "dc1",
			},
			DockerSocket: "unix:///var/run/docker.sock",
			CurrentLoad: models.ResourceLoad{
				CPUUsage:       30.0,
				MemoryUsage:    4294967296, // 4GB used
				ContainerCount: 3,
			},
			AvailableResources: models.Resources{
				CPU:    4,
				Memory: 4294967296, // 4GB available
			},
		},
		{
			Host: &models.Host{
				ID:         "host2",
				Name:       "Host 2",
				IPAddress:  "192.168.1.11",
				CPU:        8,
				Memory:     17179869184, // 16GB
				Status:     "active",
				Datacenter: "dc1",
			},
			DockerSocket: "tcp://192.168.1.11:2375",
			CurrentLoad: models.ResourceLoad{
				CPUUsage:       20.0,
				MemoryUsage:    4294967296, // 4GB used
				ContainerCount: 2,
			},
			AvailableResources: models.Resources{
				CPU:    8,
				Memory: 12884901888, // 12GB available
			},
		},
		{
			Host: &models.Host{
				ID:         "host3",
				Name:       "Host 3",
				IPAddress:  "192.168.1.12",
				CPU:        8,
				Memory:     17179869184, // 16GB
				Status:     "active",
				Datacenter: "dc2",
			},
			DockerSocket: "tcp://192.168.1.12:2375",
			CurrentLoad: models.ResourceLoad{
				CPUUsage:       15.0,
				MemoryUsage:    2147483648, // 2GB used
				ContainerCount: 1,
			},
			AvailableResources: models.Resources{
				CPU:    8,
				Memory: 15032385536, // 14GB available
			},
		},
	}
}

func createTestStack() *models.Stack {
	return &models.Stack{
		Context:    "https://schema.org",
		Type:       "ItemList",
		ID:         "test-stack",
		Name:       "test-stack",
		Status:     "pending",
		Datacenter: "dc1",
		Deployment: models.DeploymentConfig{
			Mode:              "multi-host",
			PlacementStrategy: "auto",
			NetworkMode:       "host-port",
		},
	}
}

func createTestStackDefinition() *stacks.Stack {
	return &stacks.Stack{
		Context: "https://schema.org",
		Type:    "ItemList",
		Name:    "test-stack",
		ItemListElement: []stacks.StackItemElement{
			{
				Type:  "SoftwareApplication",
				Name:  "web",
				Image: "nginx:latest",
			},
			{
				Type:  "SoftwareApplication",
				Name:  "db",
				Image: "postgres:14",
			},
			{
				Type:  "SoftwareApplication",
				Name:  "cache",
				Image: "redis:7",
			},
		},
	}
}

func TestAutoPlacementStrategy(t *testing.T) {
	strategy := &AutoPlacementStrategy{}
	ctx := context.Background()

	stack := createTestStack()
	definition := createTestStackDefinition()
	hosts := createTestHosts()

	placement, err := strategy.PlaceContainers(ctx, stack, definition, hosts)
	require.NoError(t, err)

	// Should have placement for all containers
	assert.Len(t, placement, 3)
	assert.Contains(t, placement, "web")
	assert.Contains(t, placement, "db")
	assert.Contains(t, placement, "cache")

	// All placements should be to valid hosts
	for _, hostID := range placement {
		assert.Contains(t, []string{"host1", "host2", "host3"}, hostID)
	}
}

func TestAutoPlacementStrategy_NoHosts(t *testing.T) {
	strategy := &AutoPlacementStrategy{}
	ctx := context.Background()

	stack := createTestStack()
	definition := createTestStackDefinition()

	placement, err := strategy.PlaceContainers(ctx, stack, definition, []*models.HostInfo{})
	assert.Error(t, err)
	assert.Nil(t, placement)
}

func TestSpreadPlacementStrategy(t *testing.T) {
	strategy := &SpreadPlacementStrategy{}
	ctx := context.Background()

	stack := createTestStack()
	definition := createTestStackDefinition()
	hosts := createTestHosts()

	placement, err := strategy.PlaceContainers(ctx, stack, definition, hosts)
	require.NoError(t, err)

	// Should have placement for all containers
	assert.Len(t, placement, 3)

	// Count containers per host
	hostCounts := make(map[string]int)
	for _, hostID := range placement {
		hostCounts[hostID]++
	}

	// Should use multiple hosts (spread across at least 2 hosts)
	assert.GreaterOrEqual(t, len(hostCounts), 2, "Should spread across multiple hosts")

	// Maximum difference between host counts should be at most 1
	// (as even distribution as possible)
	minCount := 999
	maxCount := 0
	for _, count := range hostCounts {
		if count < minCount {
			minCount = count
		}
		if count > maxCount {
			maxCount = count
		}
	}
	assert.LessOrEqual(t, maxCount-minCount, 1, "Should spread evenly")
}

func TestManualPlacementStrategy(t *testing.T) {
	strategy := &ManualPlacementStrategy{}
	ctx := context.Background()

	stack := createTestStack()
	stack.Deployment.PlacementStrategy = "manual"
	stack.Deployment.HostConstraints = []models.HostConstraint{
		{
			ContainerName: "web",
			TargetHostID:  "host1",
		},
		{
			ContainerName: "db",
			TargetHostID:  "host2",
		},
		{
			ContainerName: "cache",
			TargetHostID:  "host3",
		},
	}

	definition := createTestStackDefinition()
	hosts := createTestHosts()

	placement, err := strategy.PlaceContainers(ctx, stack, definition, hosts)
	require.NoError(t, err)

	// Verify manual placement
	assert.Equal(t, "host1", placement["web"])
	assert.Equal(t, "host2", placement["db"])
	assert.Equal(t, "host3", placement["cache"])
}

func TestManualPlacementStrategy_MissingConstraint(t *testing.T) {
	strategy := &ManualPlacementStrategy{}
	ctx := context.Background()

	stack := createTestStack()
	stack.Deployment.PlacementStrategy = "manual"
	stack.Deployment.HostConstraints = []models.HostConstraint{
		{
			ContainerName: "web",
			TargetHostID:  "host1",
		},
		// Missing constraints for db and cache
	}

	definition := createTestStackDefinition()
	hosts := createTestHosts()

	placement, err := strategy.PlaceContainers(ctx, stack, definition, hosts)
	assert.Error(t, err)
	assert.Nil(t, placement)
}

func TestManualPlacementStrategy_InvalidHost(t *testing.T) {
	strategy := &ManualPlacementStrategy{}
	ctx := context.Background()

	stack := createTestStack()
	stack.Deployment.PlacementStrategy = "manual"
	stack.Deployment.HostConstraints = []models.HostConstraint{
		{
			ContainerName: "web",
			TargetHostID:  "nonexistent",
		},
		{
			ContainerName: "db",
			TargetHostID:  "host2",
		},
		{
			ContainerName: "cache",
			TargetHostID:  "host3",
		},
	}

	definition := createTestStackDefinition()
	hosts := createTestHosts()

	placement, err := strategy.PlaceContainers(ctx, stack, definition, hosts)
	assert.Error(t, err)
	assert.Nil(t, placement)
}

func TestDatacenterPlacementStrategy(t *testing.T) {
	strategy := &DatacenterPlacementStrategy{}
	ctx := context.Background()

	stack := createTestStack()
	stack.Datacenter = "dc1"
	definition := createTestStackDefinition()
	hosts := createTestHosts()

	placement, err := strategy.PlaceContainers(ctx, stack, definition, hosts)
	require.NoError(t, err)

	// All containers should be in dc1
	for _, hostID := range placement {
		// host1 and host2 are in dc1, host3 is in dc2
		assert.Contains(t, []string{"host1", "host2"}, hostID)
	}
}

func TestDatacenterPlacementStrategy_NoHostsInDatacenter(t *testing.T) {
	strategy := &DatacenterPlacementStrategy{}
	ctx := context.Background()

	stack := createTestStack()
	stack.Datacenter = "dc-nonexistent"
	definition := createTestStackDefinition()
	hosts := createTestHosts()

	placement, err := strategy.PlaceContainers(ctx, stack, definition, hosts)
	assert.Error(t, err)
	assert.Nil(t, placement)
}

func TestToEnvName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"postgres", "POSTGRES"},
		{"my-service", "MY_SERVICE"},
		{"cache.redis", "CACHE_REDIS"},
		{"web-api.v2", "WEB_API_V2"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toEnvName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMergeEnv(t *testing.T) {
	base := map[string]string{
		"VAR1": "value1",
		"VAR2": "value2",
	}

	overlay := map[string]string{
		"VAR2": "overridden",
		"VAR3": "value3",
	}

	result := mergeEnv(base, overlay)

	assert.Equal(t, "value1", result["VAR1"])
	assert.Equal(t, "overridden", result["VAR2"]) // Should be overridden
	assert.Equal(t, "value3", result["VAR3"])
}
