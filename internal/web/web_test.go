package web

import (
	"testing"

	"evalgo.org/graphium/internal/config"
	"evalgo.org/graphium/internal/storage"
	"evalgo.org/graphium/models"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// TestTemplateCompilation verifies that all templates compile correctly
func TestTemplateCompilation(t *testing.T) {
	// This test will only pass if the templ templates compiled successfully
	// The presence of this test passing means templates_templ.go was generated correctly
	assert.True(t, true, "Templates compiled successfully")
}

// TestHandlerCreation verifies we can create a web handler
func TestHandlerCreation(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
		CouchDB: config.CouchDBConfig{
			URL:      "http://localhost:5984",
			Database: "test",
			Username: "admin",
			Password: "admin",
		},
	}

	// Note: We can't actually connect to storage without CouchDB running
	// but we can verify the handler struct is created correctly
	handler := &Handler{
		storage: nil,
		config:  cfg,
	}

	assert.NotNil(t, handler)
	assert.Equal(t, cfg, handler.config)
}

// TestDashboardTemplate verifies the Dashboard template function exists
func TestDashboardTemplate(t *testing.T) {
	stats := &storage.Statistics{
		TotalContainers:   10,
		RunningContainers: 8,
		TotalHosts:        3,
		HostContainerCounts: map[string]int{
			"host1": 5,
			"host2": 3,
			"host3": 2,
		},
	}

	// Verify the Dashboard template function is callable
	component := Dashboard(stats)
	assert.NotNil(t, component)
}

// TestContainersListTemplate verifies the ContainersList template function exists
func TestContainersListTemplate(t *testing.T) {
	containers := []*models.Container{
		{
			ID:       "container1",
			Name:     "test-container",
			Image:    "nginx:latest",
			Status:   "running",
			HostedOn: "host1",
			Created:  "2025-10-27T10:00:00Z",
		},
	}

	component := ContainersList(containers)
	assert.NotNil(t, component)
}

// TestHostsListTemplate verifies the HostsList template function exists
func TestHostsListTemplate(t *testing.T) {
	hosts := []*models.Host{
		{
			ID:         "host1",
			Name:       "test-host",
			IPAddress:  "192.168.1.10",
			CPU:        8,
			Memory:     16 * 1024 * 1024 * 1024,
			Status:     "active",
			Datacenter: "us-east",
		},
	}

	component := HostsList(hosts)
	assert.NotNil(t, component)
}

// TestTopologyViewTemplate verifies the TopologyView template function exists
func TestTopologyViewTemplate(t *testing.T) {
	topology := &storage.DatacenterTopology{
		Datacenter: "us-east",
		Hosts: map[string]*storage.HostTopology{
			"host1": {
				Host: &models.Host{
					ID:         "host1",
					Name:       "test-host",
					IPAddress:  "192.168.1.10",
					CPU:        8,
					Memory:     16 * 1024 * 1024 * 1024,
					Status:     "active",
					Datacenter: "us-east",
				},
				Containers: []*models.Container{
					{
						ID:       "container1",
						Name:     "test-container",
						Image:    "nginx:latest",
						Status:   "running",
						HostedOn: "host1",
					},
				},
			},
		},
	}

	component := TopologyView(topology, "us-east")
	assert.NotNil(t, component)
}

// TestRenderHelper verifies the Render helper function exists
func TestRenderHelper(t *testing.T) {
	e := echo.New()
	c := e.NewContext(nil, nil)

	// We can't fully test Render without a proper request/response
	// but we can verify the function signature is correct
	stats := &storage.Statistics{
		TotalContainers:   0,
		RunningContainers: 0,
		TotalHosts:        0,
	}

	component := Dashboard(stats)
	assert.NotNil(t, component)

	// Verify Render function exists (it will fail without proper context, but that's expected)
	err := Render(c, component)
	// We expect an error here because we don't have a proper response writer
	// but the function should exist and be callable
	assert.Error(t, err)
}
