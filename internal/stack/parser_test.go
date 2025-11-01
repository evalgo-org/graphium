package stack

import (
	"fmt"
	"testing"

	"evalgo.org/graphium/models"
)

// MockHostResolver is a test implementation of HostResolver
type MockHostResolver struct {
	hosts map[string]*models.HostInfo
}

func (m *MockHostResolver) ResolveHost(id string) (*models.HostInfo, error) {
	if host, ok := m.hosts[id]; ok {
		return host, nil
	}
	return nil, fmt.Errorf("host %s not found", id)
}

func (m *MockHostResolver) ListHosts() ([]*models.HostInfo, error) {
	hosts := make([]*models.HostInfo, 0, len(m.hosts))
	for _, h := range m.hosts {
		hosts = append(hosts, h)
	}
	return hosts, nil
}

func TestStackParser_ParseValidStack(t *testing.T) {
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

	parser := NewStackParser(resolver)

	definition := &models.StackDefinition{
		Context: "https://schema.org",
		Graph: []models.GraphNode{
			{
				ID:   "https://example.com/stacks/my-stack",
				Type: []interface{}{"datacenter:Stack", "SoftwareApplication"},
				Name: "my-stack",
				LocatedInHost: &models.Reference{
					ID: "host1",
				},
				HasPart: []models.ContainerSpec{
					{
						ID:    "https://example.com/containers/web",
						Type:  []interface{}{"datacenter:Container"},
						Name:  "web",
						Image: "nginx:latest",
						Ports: []models.PortMapping{
							{ContainerPort: 80, HostPort: 8080, Protocol: "tcp"},
						},
					},
				},
			},
		},
	}

	result, err := parser.Parse(definition)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if result.Plan == nil {
		t.Fatal("Expected deployment plan, got nil")
	}

	if result.Plan.StackNode.Name != "my-stack" {
		t.Errorf("Expected stack name 'my-stack', got '%s'", result.Plan.StackNode.Name)
	}

	if len(result.Plan.ContainerSpecs) != 1 {
		t.Errorf("Expected 1 container, got %d", len(result.Plan.ContainerSpecs))
	}

	if result.Plan.ContainerSpecs[0].Name != "web" {
		t.Errorf("Expected container name 'web', got '%s'", result.Plan.ContainerSpecs[0].Name)
	}
}

func TestStackParser_EmptyGraph(t *testing.T) {
	resolver := &MockHostResolver{hosts: map[string]*models.HostInfo{}}
	parser := NewStackParser(resolver)

	definition := &models.StackDefinition{
		Context: "https://schema.org",
		Graph:   []models.GraphNode{},
	}

	_, err := parser.Parse(definition)
	if err == nil {
		t.Error("Expected error for empty graph, got nil")
	}
}

func TestStackParser_NoStackNode(t *testing.T) {
	resolver := &MockHostResolver{hosts: map[string]*models.HostInfo{}}
	parser := NewStackParser(resolver)

	definition := &models.StackDefinition{
		Context: "https://schema.org",
		Graph: []models.GraphNode{
			{
				ID:   "https://example.com/hosts/host1",
				Type: "datacenter:Host",
				Name: "host1",
			},
		},
	}

	_, err := parser.Parse(definition)
	if err == nil {
		t.Error("Expected error when no Stack node found, got nil")
	}
}

func TestStackParser_DependencyGraph_Simple(t *testing.T) {
	resolver := &MockHostResolver{hosts: map[string]*models.HostInfo{}}
	parser := NewStackParser(resolver)

	containers := []models.ContainerSpec{
		{Name: "db", Image: "postgres:15"},
		{Name: "api", Image: "api:latest", DependsOn: []string{"db"}},
		{Name: "web", Image: "nginx:latest", DependsOn: []string{"api"}},
	}

	depGraph, err := parser.buildDependencyGraph(containers)
	if err != nil {
		t.Fatalf("buildDependencyGraph failed: %v", err)
	}

	// Should have 3 waves: [db] -> [api] -> [web]
	if len(depGraph) != 3 {
		t.Errorf("Expected 3 waves, got %d", len(depGraph))
	}

	// Wave 1 should have db
	if len(depGraph[0]) != 1 || depGraph[0][0] != "db" {
		t.Errorf("Expected wave 1 to be [db], got %v", depGraph[0])
	}

	// Wave 2 should have api
	if len(depGraph[1]) != 1 || depGraph[1][0] != "api" {
		t.Errorf("Expected wave 2 to be [api], got %v", depGraph[1])
	}

	// Wave 3 should have web
	if len(depGraph[2]) != 1 || depGraph[2][0] != "web" {
		t.Errorf("Expected wave 3 to be [web], got %v", depGraph[2])
	}
}

func TestStackParser_DependencyGraph_Parallel(t *testing.T) {
	resolver := &MockHostResolver{hosts: map[string]*models.HostInfo{}}
	parser := NewStackParser(resolver)

	containers := []models.ContainerSpec{
		{Name: "db", Image: "postgres:15"},
		{Name: "redis", Image: "redis:latest"},
		{Name: "api", Image: "api:latest", DependsOn: []string{"db", "redis"}},
	}

	depGraph, err := parser.buildDependencyGraph(containers)
	if err != nil {
		t.Fatalf("buildDependencyGraph failed: %v", err)
	}

	// Should have 2 waves: [db, redis] -> [api]
	if len(depGraph) != 2 {
		t.Errorf("Expected 2 waves, got %d", len(depGraph))
	}

	// Wave 1 should have db and redis (order doesn't matter)
	if len(depGraph[0]) != 2 {
		t.Errorf("Expected wave 1 to have 2 containers, got %d", len(depGraph[0]))
	}

	// Wave 2 should have api
	if len(depGraph[1]) != 1 || depGraph[1][0] != "api" {
		t.Errorf("Expected wave 2 to be [api], got %v", depGraph[1])
	}
}

func TestStackParser_DependencyGraph_CircularDependency(t *testing.T) {
	resolver := &MockHostResolver{hosts: map[string]*models.HostInfo{}}
	parser := NewStackParser(resolver)

	containers := []models.ContainerSpec{
		{Name: "a", Image: "image:latest", DependsOn: []string{"b"}},
		{Name: "b", Image: "image:latest", DependsOn: []string{"c"}},
		{Name: "c", Image: "image:latest", DependsOn: []string{"a"}},
	}

	_, err := parser.buildDependencyGraph(containers)
	if err == nil {
		t.Error("Expected error for circular dependency, got nil")
	}

	if err != nil && err.Error() != "circular dependency detected in container dependencies" {
		t.Errorf("Expected circular dependency error, got: %v", err)
	}
}

func TestStackParser_DependencyGraph_NonExistentDependency(t *testing.T) {
	resolver := &MockHostResolver{hosts: map[string]*models.HostInfo{}}
	parser := NewStackParser(resolver)

	containers := []models.ContainerSpec{
		{Name: "api", Image: "api:latest", DependsOn: []string{"nonexistent"}},
	}

	_, err := parser.buildDependencyGraph(containers)
	if err == nil {
		t.Error("Expected error for non-existent dependency, got nil")
	}
}

func TestStackParser_HostMapping(t *testing.T) {
	resolver := &MockHostResolver{
		hosts: map[string]*models.HostInfo{
			"host1": {Host: &models.Host{ID: "host1"}},
			"host2": {Host: &models.Host{ID: "host2"}},
		},
	}

	parser := NewStackParser(resolver)

	stackNode := &models.GraphNode{
		Name: "test-stack",
		LocatedInHost: &models.Reference{
			ID: "host1",
		},
	}

	containers := []models.ContainerSpec{
		{
			ID:    "container1",
			Name:  "web",
			Image: "nginx:latest",
			// No host specified, should use stack default
		},
		{
			ID:    "container2",
			Name:  "db",
			Image: "postgres:15",
			LocatedInHost: &models.Reference{
				ID: "host2",
			},
		},
	}

	result := &ParseResult{
		Warnings: []string{},
		Errors:   []string{},
	}

	hostMap, err := parser.buildHostMapping(stackNode, containers, result)
	if err != nil {
		t.Fatalf("buildHostMapping failed: %v", err)
	}

	// container1 should use stack default (host1)
	if hostMap["container1"] != "host1" {
		t.Errorf("Expected container1 to map to host1, got %s", hostMap["container1"])
	}

	// container2 should use its own host (host2)
	if hostMap["container2"] != "host2" {
		t.Errorf("Expected container2 to map to host2, got %s", hostMap["container2"])
	}
}

func TestStackParser_ValidateContainerSpec(t *testing.T) {
	resolver := &MockHostResolver{hosts: map[string]*models.HostInfo{}}
	parser := NewStackParser(resolver)

	tests := []struct {
		name      string
		spec      models.ContainerSpec
		wantError bool
	}{
		{
			name: "valid container",
			spec: models.ContainerSpec{
				Name:  "web",
				Image: "nginx:latest",
			},
			wantError: false,
		},
		{
			name: "missing name",
			spec: models.ContainerSpec{
				Image: "nginx:latest",
			},
			wantError: true,
		},
		{
			name: "missing image",
			spec: models.ContainerSpec{
				Name: "web",
			},
			wantError: true,
		},
		{
			name: "valid with ports",
			spec: models.ContainerSpec{
				Name:  "web",
				Image: "nginx:latest",
				Ports: []models.PortMapping{
					{ContainerPort: 80, HostPort: 8080, Protocol: "tcp"},
				},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ParseResult{
				Warnings: []string{},
				Errors:   []string{},
			}

			err := parser.validateContainerSpec(&tt.spec, result)

			if tt.wantError && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
	}
}

func TestStackParser_PortValidation(t *testing.T) {
	resolver := &MockHostResolver{hosts: map[string]*models.HostInfo{}}
	parser := NewStackParser(resolver)

	spec := models.ContainerSpec{
		Name:  "web",
		Image: "nginx:latest",
		Ports: []models.PortMapping{
			{ContainerPort: 80, HostPort: 8080, Protocol: ""},      // Missing protocol
			{ContainerPort: 0, HostPort: 8081, Protocol: "tcp"},    // Invalid container port
			{ContainerPort: 443, HostPort: 70000, Protocol: "tcp"}, // Invalid host port
		},
	}

	result := &ParseResult{
		Warnings: []string{},
		Errors:   []string{},
	}

	parser.validateContainerSpec(&spec, result)

	// Should have warnings for invalid ports and auto-fix missing protocol
	if len(result.Warnings) == 0 {
		t.Error("Expected warnings for invalid ports")
	}

	// Protocol should be auto-filled
	if spec.Ports[0].Protocol != "tcp" {
		t.Errorf("Expected protocol to be auto-filled to 'tcp', got '%s'", spec.Ports[0].Protocol)
	}
}

func TestStackParser_RestartPolicyValidation(t *testing.T) {
	resolver := &MockHostResolver{hosts: map[string]*models.HostInfo{}}
	parser := NewStackParser(resolver)

	tests := []struct {
		name          string
		restartPolicy string
		wantWarning   bool
	}{
		{"valid always", "always", false},
		{"valid on-failure", "on-failure", false},
		{"valid unless-stopped", "unless-stopped", false},
		{"valid no", "no", false},
		{"invalid policy", "invalid-policy", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := models.ContainerSpec{
				Name:          "test",
				Image:         "test:latest",
				RestartPolicy: tt.restartPolicy,
			}

			result := &ParseResult{
				Warnings: []string{},
				Errors:   []string{},
			}

			parser.validateContainerSpec(&spec, result)

			hasWarning := len(result.Warnings) > 0
			if hasWarning != tt.wantWarning {
				t.Errorf("Expected warning=%v, got warning=%v (warnings: %v)",
					tt.wantWarning, hasWarning, result.Warnings)
			}
		})
	}
}

func TestStackParser_GetContainersByWave(t *testing.T) {
	resolver := &MockHostResolver{hosts: map[string]*models.HostInfo{}}
	parser := NewStackParser(resolver)

	plan := &models.DeploymentPlan{
		ContainerSpecs: []models.ContainerSpec{
			{Name: "db", Image: "postgres:15"},
			{Name: "redis", Image: "redis:latest"},
			{Name: "api", Image: "api:latest"},
		},
		DependencyGraph: [][]string{
			{"db", "redis"},
			{"api"},
		},
	}

	waves := parser.GetContainersByWave(plan)

	if len(waves) != 2 {
		t.Errorf("Expected 2 waves, got %d", len(waves))
	}

	// Wave 1 should have db and redis
	if len(waves[0]) != 2 {
		t.Errorf("Expected wave 1 to have 2 containers, got %d", len(waves[0]))
	}

	// Wave 2 should have api
	if len(waves[1]) != 1 {
		t.Errorf("Expected wave 2 to have 1 container, got %d", len(waves[1]))
	}

	if waves[1][0].Name != "api" {
		t.Errorf("Expected wave 2 container to be 'api', got '%s'", waves[1][0].Name)
	}
}

func TestStackParser_IsStackType(t *testing.T) {
	resolver := &MockHostResolver{hosts: map[string]*models.HostInfo{}}
	parser := NewStackParser(resolver)

	tests := []struct {
		name     string
		typeVal  interface{}
		expected bool
	}{
		{"string with Stack", "datacenter:Stack", true},
		{"string with SoftwareApplication", "SoftwareApplication", true},
		{"array with Stack", []interface{}{"datacenter:Stack", "Thing"}, true},
		{"array with SoftwareApplication", []interface{}{"Thing", "SoftwareApplication"}, true},
		{"string without Stack", "Container", false},
		{"array without Stack", []interface{}{"Container", "Thing"}, false},
		{"empty string", "", false},
		{"empty array", []interface{}{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.isStackType(tt.typeVal)
			if result != tt.expected {
				t.Errorf("isStackType(%v) = %v, want %v", tt.typeVal, result, tt.expected)
			}
		})
	}
}

func TestStackParser_TopologyBuilding(t *testing.T) {
	resolver := &MockHostResolver{hosts: map[string]*models.HostInfo{}}
	parser := NewStackParser(resolver)

	graph := []models.GraphNode{
		{
			ID:   "https://example.com/hosts/host1",
			Type: "datacenter:Host",
			Name: "host1",
		},
		{
			ID:   "https://example.com/racks/rack1",
			Type: "datacenter:Rack",
			Name: "rack1",
		},
		{
			ID:   "https://example.com/datacenters/dc1",
			Type: "datacenter:Datacenter",
			Name: "dc1",
		},
		{
			ID:   "https://example.com/stacks/stack1",
			Type: "datacenter:Stack",
			Name: "stack1",
		},
	}

	topology, err := parser.buildTopology(graph)
	if err != nil {
		t.Fatalf("buildTopology failed: %v", err)
	}

	if len(topology.Hosts) != 1 {
		t.Errorf("Expected 1 host, got %d", len(topology.Hosts))
	}

	if len(topology.Racks) != 1 {
		t.Errorf("Expected 1 rack, got %d", len(topology.Racks))
	}

	if len(topology.Datacenters) != 1 {
		t.Errorf("Expected 1 datacenter, got %d", len(topology.Datacenters))
	}

	if _, ok := topology.Hosts["https://example.com/hosts/host1"]; !ok {
		t.Error("Expected host1 in topology")
	}
}
