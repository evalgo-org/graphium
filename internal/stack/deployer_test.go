package stack

import (
	"context"
	"errors"
	"testing"
	"time"

	"eve.evalgo.org/common"

	"evalgo.org/graphium/models"
)

// MockDatabase is a test implementation of the Database interface
type MockDatabase struct {
	documents map[string]interface{}
	createErr error
	updateErr error
}

func (m *MockDatabase) Create(ctx context.Context, doc interface{}) error {
	if m.createErr != nil {
		return m.createErr
	}
	if m.documents == nil {
		m.documents = make(map[string]interface{})
	}
	// Store by ID if it has one
	if state, ok := doc.(*models.DeploymentState); ok {
		m.documents[state.ID] = doc
	}
	return nil
}

func (m *MockDatabase) Update(ctx context.Context, doc interface{}) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if m.documents == nil {
		m.documents = make(map[string]interface{})
	}
	if state, ok := doc.(*models.DeploymentState); ok {
		m.documents[state.ID] = doc
	}
	return nil
}

// MockDockerClientFactory creates mock Docker clients using EVE's mock
type MockDockerClientFactory struct {
	clients       map[string]*common.MockDockerClient
	getErr        error
	defaultClient *common.MockDockerClient
}

func (f *MockDockerClientFactory) GetClient(ctx context.Context, hostID string) (common.DockerClient, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}

	if f.clients == nil {
		f.clients = make(map[string]*common.MockDockerClient)
	}

	// Return existing client for host or create new one
	if client, ok := f.clients[hostID]; ok {
		return client, nil
	}

	// Use default client if provided
	if f.defaultClient != nil {
		return f.defaultClient, nil
	}

	// Create new mock client using EVE
	client := common.NewMockDockerClient()
	f.clients[hostID] = client

	return client, nil
}

func TestDeployer_DeploySimpleStack(t *testing.T) {
	db := &MockDatabase{documents: make(map[string]interface{})}
	resolver := &MockHostResolver{
		hosts: map[string]*models.HostInfo{
			"host1": {
				Host: &models.Host{
					ID:        "host1",
					Name:      "test-host",
					IPAddress: "192.168.1.10",
				},
			},
		},
	}

	mockClient := common.NewMockDockerClient()

	clientFactory := &MockDockerClientFactory{
		defaultClient: mockClient,
	}

	deployer := NewDeployer(db, resolver, clientFactory)

	plan := &models.DeploymentPlan{
		StackNode: &models.GraphNode{
			ID:   "stack1",
			Name: "test-stack",
		},
		ContainerSpecs: []models.ContainerSpec{
			{
				ID:    "container1",
				Name:  "web",
				Image: "nginx:latest",
				Ports: []models.PortMapping{
					{ContainerPort: 80, HostPort: 8080, Protocol: "tcp"},
				},
			},
		},
		HostMap: map[string]string{
			"container1": "host1",
		},
		DependencyGraph: [][]string{
			{"web"},
		},
	}

	opts := DeployOptions{
		Timeout:         5 * time.Minute,
		RollbackOnError: false,
		StackName:       "test-stack",
		PullImages:      false,
	}

	ctx := context.Background()
	state, err := deployer.Deploy(ctx, plan, opts)

	if err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}

	if state == nil {
		t.Fatal("Expected deployment state, got nil")
	}

	if state.Status != "deploying" && state.Status != "deployed" && state.Status != "running" {
		t.Errorf("Expected status 'deploying', 'deployed', or 'running', got '%s'", state.Status)
	}

	if len(state.Placements) == 0 {
		t.Error("Expected at least one container placement")
	}

	// Verify container was created
	if !mockClient.ContainerCreateCalled {
		t.Error("Expected ContainerCreate to be called")
	}

	expectedName := "test-stack-web"
	if mockClient.LastContainerName != expectedName {
		t.Errorf("Expected container name '%s', got '%s'", expectedName, mockClient.LastContainerName)
	}
}

func TestDeployer_DeployWithNetwork(t *testing.T) {
	db := &MockDatabase{documents: make(map[string]interface{})}
	resolver := &MockHostResolver{
		hosts: map[string]*models.HostInfo{
			"host1": {
				Host: &models.Host{
					ID:        "host1",
					Name:      "test-host",
					IPAddress: "192.168.1.10",
				},
			},
		},
	}

	mockClient := common.NewMockDockerClient()

	clientFactory := &MockDockerClientFactory{
		defaultClient: mockClient,
	}

	deployer := NewDeployer(db, resolver, clientFactory)

	plan := &models.DeploymentPlan{
		StackNode: &models.GraphNode{
			ID:   "stack1",
			Name: "test-stack",
		},
		ContainerSpecs: []models.ContainerSpec{
			{
				ID:    "container1",
				Name:  "web",
				Image: "nginx:latest",
			},
		},
		HostMap: map[string]string{
			"container1": "host1",
		},
		Network: &models.NetworkSpec{
			Name:              "app-network",
			Driver:            "bridge",
			CreateIfNotExists: true,
		},
		DependencyGraph: [][]string{
			{"web"},
		},
	}

	opts := DeployOptions{
		Timeout:         5 * time.Minute,
		RollbackOnError: false,
		StackName:       "test-stack",
		PullImages:      false,
	}

	ctx := context.Background()
	state, err := deployer.Deploy(ctx, plan, opts)

	if err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}

	// Verify network was created
	if !mockClient.NetworkCreateCalled {
		t.Error("Expected NetworkCreate to be called")
	}

	// Verify network info in state
	if state.NetworkInfo == nil {
		t.Error("Expected network info in deployment state")
	}
}

func TestDeployer_DeployWithDependencies(t *testing.T) {
	db := &MockDatabase{documents: make(map[string]interface{})}
	resolver := &MockHostResolver{
		hosts: map[string]*models.HostInfo{
			"host1": {
				Host: &models.Host{
					ID:        "host1",
					Name:      "test-host",
					IPAddress: "192.168.1.10",
				},
			},
		},
	}

	mockClient := common.NewMockDockerClient()

	clientFactory := &MockDockerClientFactory{
		defaultClient: mockClient,
	}

	deployer := NewDeployer(db, resolver, clientFactory)

	plan := &models.DeploymentPlan{
		StackNode: &models.GraphNode{
			ID:   "stack1",
			Name: "test-stack",
		},
		ContainerSpecs: []models.ContainerSpec{
			{
				ID:    "db",
				Name:  "db",
				Image: "postgres:15",
			},
			{
				ID:    "api",
				Name:  "api",
				Image: "api:latest",
			},
		},
		HostMap: map[string]string{
			"db":  "host1",
			"api": "host1",
		},
		DependencyGraph: [][]string{
			{"db"},  // Wave 1: db
			{"api"}, // Wave 2: api (depends on db)
		},
	}

	opts := DeployOptions{
		Timeout:         5 * time.Minute,
		RollbackOnError: false,
		StackName:       "test-stack",
		PullImages:      false,
	}

	ctx := context.Background()
	state, err := deployer.Deploy(ctx, plan, opts)

	if err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}

	// Verify containers were created
	if !mockClient.ContainerCreateCalled {
		t.Error("Expected ContainerCreate to be called")
	}

	// Verify both placements in state
	if len(state.Placements) != 2 {
		t.Errorf("Expected 2 placements, got %d", len(state.Placements))
	}
}

func TestDeployer_RollbackOnError(t *testing.T) {
	db := &MockDatabase{documents: make(map[string]interface{})}
	resolver := &MockHostResolver{
		hosts: map[string]*models.HostInfo{
			"host1": {
				Host: &models.Host{
					ID:        "host1",
					Name:      "test-host",
					IPAddress: "192.168.1.10",
				},
			},
		},
	}

	mockClient := common.NewMockDockerClient()
	// Make the container start fail
	mockClient.Err = errors.New("failed to start container")

	clientFactory := &MockDockerClientFactory{
		defaultClient: mockClient,
	}

	deployer := NewDeployer(db, resolver, clientFactory)

	plan := &models.DeploymentPlan{
		StackNode: &models.GraphNode{
			ID:   "stack1",
			Name: "test-stack",
		},
		ContainerSpecs: []models.ContainerSpec{
			{
				ID:    "container1",
				Name:  "web",
				Image: "nginx:latest",
			},
		},
		HostMap: map[string]string{
			"container1": "host1",
		},
		DependencyGraph: [][]string{
			{"web"},
		},
	}

	opts := DeployOptions{
		Timeout:         5 * time.Minute,
		RollbackOnError: true,
		StackName:       "test-stack",
		PullImages:      false,
	}

	ctx := context.Background()
	state, err := deployer.Deploy(ctx, plan, opts)

	// Deploy should fail
	if err == nil {
		t.Error("Expected deploy to fail, but it succeeded")
	}

	// State should indicate failure
	if state.Status != "failed" {
		t.Errorf("Expected status 'failed', got '%s'", state.Status)
	}

	// Should have rollback state
	if state.RollbackState == nil {
		t.Error("Expected rollback state, got nil")
	}
}

func TestDeployer_HostNotFound(t *testing.T) {
	db := &MockDatabase{documents: make(map[string]interface{})}
	resolver := &MockHostResolver{
		hosts: map[string]*models.HostInfo{}, // No hosts
	}

	clientFactory := &MockDockerClientFactory{}

	deployer := NewDeployer(db, resolver, clientFactory)

	plan := &models.DeploymentPlan{
		StackNode: &models.GraphNode{
			ID:   "stack1",
			Name: "test-stack",
		},
		ContainerSpecs: []models.ContainerSpec{
			{
				ID:    "container1",
				Name:  "web",
				Image: "nginx:latest",
			},
		},
		HostMap: map[string]string{
			"container1": "nonexistent-host",
		},
		DependencyGraph: [][]string{
			{"web"},
		},
	}

	opts := DeployOptions{
		Timeout:         5 * time.Minute,
		RollbackOnError: false,
		StackName:       "test-stack",
		PullImages:      false,
	}

	ctx := context.Background()
	_, err := deployer.Deploy(ctx, plan, opts)

	// Should fail with host not found error
	if err == nil {
		t.Error("Expected deploy to fail with host not found, but it succeeded")
	}
}

func TestDeployer_DockerClientError(t *testing.T) {
	db := &MockDatabase{documents: make(map[string]interface{})}
	resolver := &MockHostResolver{
		hosts: map[string]*models.HostInfo{
			"host1": {
				Host: &models.Host{
					ID:        "host1",
					Name:      "test-host",
					IPAddress: "192.168.1.10",
				},
			},
		},
	}

	clientFactory := &MockDockerClientFactory{
		getErr: errors.New("failed to connect to Docker"),
	}

	deployer := NewDeployer(db, resolver, clientFactory)

	plan := &models.DeploymentPlan{
		StackNode: &models.GraphNode{
			ID:   "stack1",
			Name: "test-stack",
		},
		ContainerSpecs: []models.ContainerSpec{
			{
				ID:    "container1",
				Name:  "web",
				Image: "nginx:latest",
			},
		},
		HostMap: map[string]string{
			"container1": "host1",
		},
		DependencyGraph: [][]string{
			{"web"},
		},
	}

	opts := DeployOptions{
		Timeout:         5 * time.Minute,
		RollbackOnError: false,
		StackName:       "test-stack",
		PullImages:      false,
	}

	ctx := context.Background()
	_, err := deployer.Deploy(ctx, plan, opts)

	// Should fail with Docker client error
	if err == nil {
		t.Error("Expected deploy to fail with Docker client error, but it succeeded")
	}
}

func TestDeployer_BuildContainerConfig(t *testing.T) {
	db := &MockDatabase{}
	resolver := &MockHostResolver{}
	clientFactory := &MockDockerClientFactory{}

	deployer := NewDeployer(db, resolver, clientFactory)

	spec := &models.ContainerSpec{
		Name:  "web",
		Image: "nginx:latest",
		Environment: []models.EnvironmentVariable{
			{Name: "ENV", Value: "production"},
		},
		Command:    []string{"/bin/sh"},
		Args:       []string{"-c", "nginx -g 'daemon off;'"},
		WorkingDir: "/app",
		User:       "nginx",
		Labels: map[string]string{
			"app": "web",
		},
	}

	config := deployer.buildContainerConfig(spec)

	if config.Image != "nginx:latest" {
		t.Errorf("Expected image 'nginx:latest', got '%s'", config.Image)
	}

	if len(config.Env) != 1 {
		t.Errorf("Expected 1 environment variable, got %d", len(config.Env))
	}

	if config.WorkingDir != "/app" {
		t.Errorf("Expected working dir '/app', got '%s'", config.WorkingDir)
	}

	if config.User != "nginx" {
		t.Errorf("Expected user 'nginx', got '%s'", config.User)
	}
}

func TestDeployer_EventLogging(t *testing.T) {
	db := &MockDatabase{documents: make(map[string]interface{})}
	resolver := &MockHostResolver{
		hosts: map[string]*models.HostInfo{
			"host1": {
				Host: &models.Host{
					ID:        "host1",
					Name:      "test-host",
					IPAddress: "192.168.1.10",
				},
			},
		},
	}

	mockClient := common.NewMockDockerClient()

	clientFactory := &MockDockerClientFactory{
		defaultClient: mockClient,
	}

	deployer := NewDeployer(db, resolver, clientFactory)

	plan := &models.DeploymentPlan{
		StackNode: &models.GraphNode{
			ID:   "stack1",
			Name: "test-stack",
		},
		ContainerSpecs: []models.ContainerSpec{
			{
				ID:    "container1",
				Name:  "web",
				Image: "nginx:latest",
			},
		},
		HostMap: map[string]string{
			"container1": "host1",
		},
		DependencyGraph: [][]string{
			{"web"},
		},
	}

	opts := DeployOptions{
		Timeout:         5 * time.Minute,
		RollbackOnError: false,
		StackName:       "test-stack",
		PullImages:      false,
	}

	ctx := context.Background()
	state, err := deployer.Deploy(ctx, plan, opts)

	if err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}

	// Verify events were logged
	if len(state.Events) == 0 {
		t.Error("Expected deployment events, got none")
	}

	// Should have at least initialization and container deployment events
	hasInitEvent := false
	hasDeployEvent := false

	for _, event := range state.Events {
		if event.Phase == "initialization" {
			hasInitEvent = true
		}
		if event.Phase == "container-deployment" {
			hasDeployEvent = true
		}
	}

	if !hasInitEvent {
		t.Error("Expected initialization event")
	}

	if !hasDeployEvent {
		t.Error("Expected container deployment event")
	}
}
