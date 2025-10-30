package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestStackDefinition_JSONMarshaling(t *testing.T) {
	definition := &StackDefinition{
		Context: "https://schema.org",
		Graph: []GraphNode{
			{
				ID:          "https://example.com/stacks/test",
				Type:        []interface{}{"datacenter:Stack", "SoftwareApplication"},
				Name:        "test-stack",
				Description: "Test stack",
				LocatedInHost: &Reference{
					ID: "https://example.com/hosts/host1",
				},
				HasPart: []ContainerSpec{
					{
						ID:    "https://example.com/containers/web",
						Type:  []interface{}{"datacenter:Container"},
						Name:  "web",
						Image: "nginx:latest",
						Ports: []PortMapping{
							{ContainerPort: 80, HostPort: 8080, Protocol: "tcp"},
						},
					},
				},
			},
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(definition)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal back
	var decoded StackDefinition
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify
	if decoded.Context != "https://schema.org" {
		t.Errorf("Expected context 'https://schema.org', got '%v'", decoded.Context)
	}

	if len(decoded.Graph) != 1 {
		t.Errorf("Expected 1 graph node, got %d", len(decoded.Graph))
	}

	if decoded.Graph[0].Name != "test-stack" {
		t.Errorf("Expected stack name 'test-stack', got '%s'", decoded.Graph[0].Name)
	}

	if len(decoded.Graph[0].HasPart) != 1 {
		t.Errorf("Expected 1 container, got %d", len(decoded.Graph[0].HasPart))
	}
}

func TestContainerSpec_FullSpec(t *testing.T) {
	spec := ContainerSpec{
		ID:                  "container1",
		Type:                "Container",
		Name:                "web",
		ApplicationCategory: "WebServer",
		Image:               "nginx:latest",
		Environment: []EnvironmentVariable{
			{Name: "ENV", Value: "production"},
		},
		Ports: []PortMapping{
			{ContainerPort: 80, HostPort: 8080, Protocol: "tcp"},
			{ContainerPort: 443, HostPort: 8443, Protocol: "tcp"},
		},
		VolumeMounts: []VolumeMount{
			{
				Source:   "data",
				Target:   "/usr/share/nginx/html",
				Type:     "volume",
				ReadOnly: false,
			},
		},
		HealthCheck: &HealthCheck{
			Type:        "http",
			Path:        "/health",
			Port:        80,
			Interval:    30,
			Timeout:     10,
			Retries:     3,
			StartPeriod: 60,
		},
		Resources: &ResourceConstraints{
			Limits: &ResourceLimits{
				CPUs:   2.0,
				Memory: 1024 * 1024 * 1024, // 1GB
			},
			Reservations: &ResourceReservations{
				CPUs:   0.5,
				Memory: 512 * 1024 * 1024, // 512MB
			},
		},
		RestartPolicy: "unless-stopped",
		Command:       []string{"/bin/sh"},
		Args:          []string{"-c", "nginx -g 'daemon off;'"},
		WorkingDir:    "/app",
		User:          "nginx",
		Labels: map[string]string{
			"app": "web",
		},
	}

	// Marshal and unmarshal
	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded ContainerSpec
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify key fields
	if decoded.Name != "web" {
		t.Errorf("Expected name 'web', got '%s'", decoded.Name)
	}

	if decoded.Image != "nginx:latest" {
		t.Errorf("Expected image 'nginx:latest', got '%s'", decoded.Image)
	}

	if len(decoded.Ports) != 2 {
		t.Errorf("Expected 2 ports, got %d", len(decoded.Ports))
	}

	if decoded.HealthCheck == nil {
		t.Error("Expected health check, got nil")
	} else if decoded.HealthCheck.Path != "/health" {
		t.Errorf("Expected health check path '/health', got '%s'", decoded.HealthCheck.Path)
	}

	if decoded.Resources == nil {
		t.Error("Expected resources, got nil")
	} else {
		if decoded.Resources.Limits.CPUs != 2.0 {
			t.Errorf("Expected CPU limit 2.0, got %f", decoded.Resources.Limits.CPUs)
		}
	}
}

func TestNetworkSpec_JSONMarshaling(t *testing.T) {
	network := NetworkSpec{
		Name:              "app-network",
		Driver:            "bridge",
		CreateIfNotExists: true,
		Subnet:            "172.20.0.0/16",
		Gateway:           "172.20.0.1",
		IPRange:           "172.20.10.0/24",
		Options: map[string]string{
			"com.docker.network.bridge.name": "app-br0",
		},
		Labels: map[string]string{
			"environment": "production",
		},
	}

	data, err := json.Marshal(network)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded NetworkSpec
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded.Name != "app-network" {
		t.Errorf("Expected name 'app-network', got '%s'", decoded.Name)
	}

	if decoded.Driver != "bridge" {
		t.Errorf("Expected driver 'bridge', got '%s'", decoded.Driver)
	}

	if decoded.Subnet != "172.20.0.0/16" {
		t.Errorf("Expected subnet '172.20.0.0/16', got '%s'", decoded.Subnet)
	}
}

func TestDeploymentState_JSONMarshaling(t *testing.T) {
	now := time.Now()
	state := DeploymentState{
		ID:       "deployment-1",
		StackID:  "stack-1",
		Status:   "deploying",
		Phase:    "container-deployment",
		Progress: 50,
		Placements: map[string]*ContainerPlacement{
			"web": {
				ContainerID:   "abc123",
				ContainerName: "stack-web",
				HostID:        "host1",
				IPAddress:     "192.168.1.10",
				Ports: map[int]int{
					80: 8080,
				},
				Status:    "running",
				StartedAt: &now,
			},
		},
		NetworkInfo: &DeployedNetworkInfo{
			NetworkID:   "net123",
			NetworkName: "stack-network",
			Driver:      "bridge",
			Subnet:      "172.20.0.0/16",
			Gateway:     "172.20.0.1",
			Scope:       "local",
		},
		Events: []DeploymentEvent{
			{
				Timestamp: now,
				Type:      "info",
				Phase:     "initialization",
				Message:   "Starting deployment",
			},
		},
		StartedAt: now,
	}

	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded DeploymentState
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded.StackID != "stack-1" {
		t.Errorf("Expected stack ID 'stack-1', got '%s'", decoded.StackID)
	}

	if decoded.Status != "deploying" {
		t.Errorf("Expected status 'deploying', got '%s'", decoded.Status)
	}

	if decoded.Progress != 50 {
		t.Errorf("Expected progress 50, got %d", decoded.Progress)
	}

	if len(decoded.Placements) != 1 {
		t.Errorf("Expected 1 placement, got %d", len(decoded.Placements))
	}

	if len(decoded.Events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(decoded.Events))
	}
}

func TestPortMapping_Validation(t *testing.T) {
	tests := []struct {
		name    string
		port    PortMapping
		wantErr bool
	}{
		{
			name: "valid TCP port",
			port: PortMapping{
				ContainerPort: 80,
				HostPort:      8080,
				Protocol:      "tcp",
			},
			wantErr: false,
		},
		{
			name: "valid UDP port",
			port: PortMapping{
				ContainerPort: 53,
				HostPort:      5353,
				Protocol:      "udp",
			},
			wantErr: false,
		},
		{
			name: "dynamic host port",
			port: PortMapping{
				ContainerPort: 80,
				HostPort:      0, // Dynamic allocation
				Protocol:      "tcp",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.port)
			if err != nil {
				t.Fatalf("Failed to marshal: %v", err)
			}

			var decoded PortMapping
			err = json.Unmarshal(data, &decoded)
			if err != nil {
				t.Fatalf("Failed to unmarshal: %v", err)
			}

			if decoded.ContainerPort != tt.port.ContainerPort {
				t.Errorf("Container port mismatch: got %d, want %d",
					decoded.ContainerPort, tt.port.ContainerPort)
			}
		})
	}
}

func TestVolumeMount_Types(t *testing.T) {
	tests := []struct {
		name   string
		mount  VolumeMount
		verify func(*testing.T, VolumeMount)
	}{
		{
			name: "volume mount",
			mount: VolumeMount{
				Source:   "data-volume",
				Target:   "/data",
				Type:     "volume",
				ReadOnly: false,
			},
			verify: func(t *testing.T, m VolumeMount) {
				if m.Type != "volume" {
					t.Errorf("Expected type 'volume', got '%s'", m.Type)
				}
			},
		},
		{
			name: "bind mount",
			mount: VolumeMount{
				Source:   "/host/path",
				Target:   "/container/path",
				Type:     "bind",
				ReadOnly: true,
				BindOptions: &BindOptions{
					Propagation: "rprivate",
				},
			},
			verify: func(t *testing.T, m VolumeMount) {
				if m.Type != "bind" {
					t.Errorf("Expected type 'bind', got '%s'", m.Type)
				}
				if !m.ReadOnly {
					t.Error("Expected read-only mount")
				}
				if m.BindOptions == nil {
					t.Fatal("Expected bind options")
				}
				if m.BindOptions.Propagation != "rprivate" {
					t.Errorf("Expected propagation 'rprivate', got '%s'",
						m.BindOptions.Propagation)
				}
			},
		},
		{
			name: "tmpfs mount",
			mount: VolumeMount{
				Source: "",
				Target: "/tmp",
				Type:   "tmpfs",
			},
			verify: func(t *testing.T, m VolumeMount) {
				if m.Type != "tmpfs" {
					t.Errorf("Expected type 'tmpfs', got '%s'", m.Type)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.mount)
			if err != nil {
				t.Fatalf("Failed to marshal: %v", err)
			}

			var decoded VolumeMount
			err = json.Unmarshal(data, &decoded)
			if err != nil {
				t.Fatalf("Failed to unmarshal: %v", err)
			}

			tt.verify(t, decoded)
		})
	}
}

func TestHealthCheck_Types(t *testing.T) {
	tests := []struct {
		name  string
		check HealthCheck
	}{
		{
			name: "HTTP health check",
			check: HealthCheck{
				Type:        "http",
				Path:        "/healthz",
				Port:        8080,
				Interval:    30,
				Timeout:     10,
				Retries:     3,
				StartPeriod: 60,
				Headers: map[string]string{
					"Authorization": "Bearer token",
				},
			},
		},
		{
			name: "TCP health check",
			check: HealthCheck{
				Type:        "tcp",
				Port:        5432,
				Interval:    30,
				Timeout:     5,
				Retries:     3,
				StartPeriod: 10,
			},
		},
		{
			name: "exec health check",
			check: HealthCheck{
				Type:        "exec",
				Command:     []string{"curl", "-f", "http://localhost/health"},
				Interval:    30,
				Timeout:     10,
				Retries:     3,
				StartPeriod: 30,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.check)
			if err != nil {
				t.Fatalf("Failed to marshal: %v", err)
			}

			var decoded HealthCheck
			err = json.Unmarshal(data, &decoded)
			if err != nil {
				t.Fatalf("Failed to unmarshal: %v", err)
			}

			if decoded.Type != tt.check.Type {
				t.Errorf("Expected type '%s', got '%s'", tt.check.Type, decoded.Type)
			}

			if decoded.Interval != tt.check.Interval {
				t.Errorf("Expected interval %d, got %d", tt.check.Interval, decoded.Interval)
			}
		})
	}
}

func TestDeploymentPlan_Structure(t *testing.T) {
	plan := &DeploymentPlan{
		StackNode: &GraphNode{
			ID:   "stack1",
			Name: "test-stack",
		},
		ContainerSpecs: []ContainerSpec{
			{Name: "db", Image: "postgres:15"},
			{Name: "api", Image: "api:latest"},
			{Name: "web", Image: "nginx:latest"},
		},
		HostMap: map[string]string{
			"db":  "host1",
			"api": "host1",
			"web": "host2",
		},
		Network: &NetworkSpec{
			Name:   "app-network",
			Driver: "bridge",
		},
		DependencyGraph: [][]string{
			{"db"},
			{"api"},
			{"web"},
		},
	}

	// Verify structure
	if plan.StackNode.Name != "test-stack" {
		t.Errorf("Expected stack name 'test-stack', got '%s'", plan.StackNode.Name)
	}

	if len(plan.ContainerSpecs) != 3 {
		t.Errorf("Expected 3 containers, got %d", len(plan.ContainerSpecs))
	}

	if len(plan.HostMap) != 3 {
		t.Errorf("Expected 3 host mappings, got %d", len(plan.HostMap))
	}

	if len(plan.DependencyGraph) != 3 {
		t.Errorf("Expected 3 deployment waves, got %d", len(plan.DependencyGraph))
	}

	if plan.Network == nil {
		t.Error("Expected network spec, got nil")
	}
}

func TestRollbackState_JSONMarshaling(t *testing.T) {
	now := time.Now()
	completed := now.Add(5 * time.Minute)

	rollback := RollbackState{
		Status:            "rolled-back",
		StartedAt:         now,
		CompletedAt:       &completed,
		RemovedContainers: []string{"web", "api", "db"},
	}

	data, err := json.Marshal(rollback)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded RollbackState
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded.Status != "rolled-back" {
		t.Errorf("Expected status 'rolled-back', got '%s'", decoded.Status)
	}

	if len(decoded.RemovedContainers) != 3 {
		t.Errorf("Expected 3 removed containers, got %d", len(decoded.RemovedContainers))
	}
}

// TestEnvironmentVariable_ArrayFormat tests the new environment variable array format
func TestEnvironmentVariable_ArrayFormat(t *testing.T) {
	envVars := []EnvironmentVariable{
		{
			Type:  "datacenter:EnvironmentVariable",
			Name:  "DATABASE_URL",
			Value: "postgres://localhost:5432/mydb",
		},
		{
			Type:  "datacenter:EnvironmentVariable",
			Name:  "API_KEY",
			Value: "secret-key-123",
		},
		{
			Name:  "ENV",
			Value: "production",
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(envVars)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal back
	var decoded []EnvironmentVariable
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify
	if len(decoded) != 3 {
		t.Errorf("Expected 3 environment variables, got %d", len(decoded))
	}

	if decoded[0].Name != "DATABASE_URL" {
		t.Errorf("Expected name 'DATABASE_URL', got '%s'", decoded[0].Name)
	}

	if decoded[0].Value != "postgres://localhost:5432/mydb" {
		t.Errorf("Expected specific database URL, got '%s'", decoded[0].Value)
	}

	if decoded[0].Type != "datacenter:EnvironmentVariable" {
		t.Errorf("Expected type 'datacenter:EnvironmentVariable', got '%s'", decoded[0].Type)
	}

	// Third entry has no @type
	if decoded[2].Type != "" {
		t.Errorf("Expected empty type for third entry, got '%s'", decoded[2].Type)
	}
}

// TestContainerSpec_WithEnvironmentArray tests ContainerSpec with the new environment array
func TestContainerSpec_WithEnvironmentArray(t *testing.T) {
	spec := ContainerSpec{
		ID:    "container1",
		Name:  "web",
		Image: "nginx:latest",
		Environment: []EnvironmentVariable{
			{
				Type:  "datacenter:EnvironmentVariable",
				Name:  "NGINX_HOST",
				Value: "localhost",
			},
			{
				Type:  "datacenter:EnvironmentVariable",
				Name:  "NGINX_PORT",
				Value: "80",
			},
		},
		Ports: []PortMapping{
			{ContainerPort: 80, HostPort: 8080, Protocol: "tcp"},
		},
	}

	// Marshal and unmarshal
	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded ContainerSpec
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify environment variables
	if len(decoded.Environment) != 2 {
		t.Errorf("Expected 2 environment variables, got %d", len(decoded.Environment))
	}

	if decoded.Environment[0].Name != "NGINX_HOST" {
		t.Errorf("Expected first env name 'NGINX_HOST', got '%s'", decoded.Environment[0].Name)
	}

	if decoded.Environment[1].Value != "80" {
		t.Errorf("Expected second env value '80', got '%s'", decoded.Environment[1].Value)
	}
}

// TestResourceRequirements_JSONMarshaling tests the new ResourceRequirements struct
func TestResourceRequirements_JSONMarshaling(t *testing.T) {
	requirements := ResourceRequirements{
		Type:                "datacenter:ResourceRequirements",
		MinCPU:              2,
		MaxCPU:              4,
		MinMemory:           2147483648, // 2GB
		MaxMemory:           4294967296, // 4GB
		RequiredLabels:      map[string]string{"environment": "production"},
		PreferredDatacenter: "dc1",
		Description:         "Database server requires high resources",
	}

	data, err := json.Marshal(requirements)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded ResourceRequirements
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded.MinCPU != 2 {
		t.Errorf("Expected MinCPU 2, got %d", decoded.MinCPU)
	}

	if decoded.MaxMemory != 4294967296 {
		t.Errorf("Expected MaxMemory 4294967296, got %d", decoded.MaxMemory)
	}

	if decoded.PreferredDatacenter != "dc1" {
		t.Errorf("Expected PreferredDatacenter 'dc1', got '%s'", decoded.PreferredDatacenter)
	}

	if decoded.Description != "Database server requires high resources" {
		t.Errorf("Expected specific description, got '%s'", decoded.Description)
	}
}

// TestContainerSpec_WithResourceRequirements tests ContainerSpec with resource requirements
func TestContainerSpec_WithResourceRequirements(t *testing.T) {
	spec := ContainerSpec{
		Name:  "database",
		Image: "postgres:15",
		ResourceRequirements: &ResourceRequirements{
			Type:      "datacenter:ResourceRequirements",
			MinCPU:    2,
			MaxCPU:    4,
			MinMemory: 2147483648,
			MaxMemory: 4294967296,
		},
		Environment: []EnvironmentVariable{
			{Name: "POSTGRES_PASSWORD", Value: "secret"},
		},
	}

	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded ContainerSpec
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded.ResourceRequirements == nil {
		t.Fatal("Expected resource requirements, got nil")
	}

	if decoded.ResourceRequirements.MinCPU != 2 {
		t.Errorf("Expected MinCPU 2, got %d", decoded.ResourceRequirements.MinCPU)
	}

	if decoded.ResourceRequirements.MaxMemory != 4294967296 {
		t.Errorf("Expected MaxMemory 4294967296, got %d", decoded.ResourceRequirements.MaxMemory)
	}
}

// TestFullStack_WithNewStructures tests a complete stack with all new features
func TestFullStack_WithNewStructures(t *testing.T) {
	definition := StackDefinition{
		Context: map[string]interface{}{
			"@vocab":     "https://graphium.evalgo.org/schema/",
			"schema":     "https://schema.org/",
			"datacenter": "https://graphium.evalgo.org/schema/datacenter/",
		},
		Graph: []GraphNode{
			{
				ID:   "https://example.com/stacks/mystack",
				Type: []interface{}{"datacenter:Stack", "schema:SoftwareApplication"},
				Name: "mystack",
				Deployment: &DeploymentConfig{
					Type:              "datacenter:DeploymentConfig",
					Mode:              "multi-host",
					PlacementStrategy: "auto",
					TargetDatacenter:  "dc1",
					NetworkMode:       "host-port",
					Comment:           "Automatic placement with resource awareness",
				},
				HasPart: []ContainerSpec{
					{
						ID:    "https://example.com/containers/db",
						Type:  []interface{}{"datacenter:Container"},
						Name:  "database",
						Image: "postgres:15",
						Environment: []EnvironmentVariable{
							{
								Type:  "datacenter:EnvironmentVariable",
								Name:  "POSTGRES_PASSWORD",
								Value: "secret",
							},
						},
						ResourceRequirements: &ResourceRequirements{
							Type:      "datacenter:ResourceRequirements",
							MinCPU:    2,
							MinMemory: 2147483648,
						},
					},
				},
			},
		},
	}

	// Marshal and unmarshal
	data, err := json.Marshal(definition)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded StackDefinition
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify structure
	if len(decoded.Graph) != 1 {
		t.Fatalf("Expected 1 graph node, got %d", len(decoded.Graph))
	}

	node := decoded.Graph[0]

	// Check deployment config
	if node.Deployment == nil {
		t.Fatal("Expected deployment config, got nil")
	}
	if node.Deployment.PlacementStrategy != "auto" {
		t.Errorf("Expected placement strategy 'auto', got '%s'", node.Deployment.PlacementStrategy)
	}

	// Check container
	if len(node.HasPart) != 1 {
		t.Fatalf("Expected 1 container, got %d", len(node.HasPart))
	}

	container := node.HasPart[0]

	// Check environment array
	if len(container.Environment) != 1 {
		t.Errorf("Expected 1 environment variable, got %d", len(container.Environment))
	}
	if container.Environment[0].Name != "POSTGRES_PASSWORD" {
		t.Errorf("Expected env name 'POSTGRES_PASSWORD', got '%s'", container.Environment[0].Name)
	}

	// Check resource requirements
	if container.ResourceRequirements == nil {
		t.Fatal("Expected resource requirements, got nil")
	}
	if container.ResourceRequirements.MinCPU != 2 {
		t.Errorf("Expected MinCPU 2, got %d", container.ResourceRequirements.MinCPU)
	}
}
