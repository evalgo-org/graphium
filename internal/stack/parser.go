package stack

import (
	"fmt"
	"strings"

	"evalgo.org/graphium/models"
)

// StackParser parses JSON-LD @graph stack definitions into executable deployment plans.
type StackParser struct {
	// HostResolver resolves absolute @id URLs to actual host objects
	HostResolver HostResolver
}

// HostResolver defines the interface for resolving host references.
type HostResolver interface {
	// ResolveHost takes an absolute @id URL and returns the host information
	ResolveHost(id string) (*models.HostInfo, error)

	// ListHosts returns all available hosts for automatic placement
	ListHosts() ([]*models.HostInfo, error)
}

// ParseResult contains the parsed stack definition and any validation warnings.
type ParseResult struct {
	// Definition is the original stack definition
	Definition *models.StackDefinition

	// Plan is the executable deployment plan
	Plan *models.DeploymentPlan

	// Warnings contains non-fatal validation warnings
	Warnings []string

	// Errors contains fatal validation errors
	Errors []string
}

// NewStackParser creates a new stack parser with the given host resolver.
func NewStackParser(resolver HostResolver) *StackParser {
	return &StackParser{
		HostResolver: resolver,
	}
}

// Parse parses a JSON-LD stack definition and creates a deployment plan.
func (p *StackParser) Parse(def *models.StackDefinition) (*ParseResult, error) {
	result := &ParseResult{
		Definition: def,
		Warnings:   []string{},
		Errors:     []string{},
	}

	// Validate basic structure
	if err := p.validateDefinition(def, result); err != nil {
		return result, err
	}

	// Extract the stack node from @graph
	stackNode, err := p.extractStackNode(def)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return result, fmt.Errorf("failed to extract stack node: %w", err)
	}

	// Extract container specifications
	containerSpecs := stackNode.HasPart
	if len(containerSpecs) == 0 {
		result.Warnings = append(result.Warnings, "stack contains no containers")
	}

	// Build host mapping (container @id -> host @id)
	hostMap, err := p.buildHostMapping(stackNode, containerSpecs, result)
	if err != nil {
		return result, fmt.Errorf("failed to build host mapping: %w", err)
	}

	// Build topology from @graph nodes
	topology, err := p.buildTopology(def.Graph)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("topology extraction incomplete: %v", err))
	}

	// Build dependency graph for container startup ordering
	depGraph, err := p.buildDependencyGraph(containerSpecs)
	if err != nil {
		return result, fmt.Errorf("failed to build dependency graph: %w", err)
	}

	// Create the deployment plan
	result.Plan = &models.DeploymentPlan{
		StackNode:       stackNode,
		ContainerSpecs:  containerSpecs,
		HostMap:         hostMap,
		Network:         stackNode.Network,
		Topology:        topology,
		DependencyGraph: depGraph,
	}

	// Validate the plan
	if err := p.validatePlan(result.Plan, result); err != nil {
		return result, err
	}

	return result, nil
}

// validateDefinition performs basic validation of the stack definition structure.
func (p *StackParser) validateDefinition(def *models.StackDefinition, result *ParseResult) error {
	if def == nil {
		return fmt.Errorf("stack definition is nil")
	}

	if def.Context == nil {
		result.Warnings = append(result.Warnings, "@context is missing")
	}

	if len(def.Graph) == 0 {
		return fmt.Errorf("@graph is empty")
	}

	return nil
}

// extractStackNode finds and extracts the main Stack node from the @graph.
func (p *StackParser) extractStackNode(def *models.StackDefinition) (*models.GraphNode, error) {
	for i := range def.Graph {
		node := &def.Graph[i]
		if p.isStackType(node.Type) {
			return node, nil
		}
	}

	return nil, fmt.Errorf("no Stack node found in @graph")
}

// isStackType checks if a @type value includes "Stack" or "SoftwareApplication".
func (p *StackParser) isStackType(typeVal interface{}) bool {
	switch t := typeVal.(type) {
	case string:
		return strings.Contains(t, "Stack") || t == "SoftwareApplication"
	case []interface{}:
		for _, v := range t {
			if s, ok := v.(string); ok {
				if strings.Contains(s, "Stack") || s == "SoftwareApplication" {
					return true
				}
			}
		}
	}
	return false
}

// buildHostMapping creates a map from container @id to host @id.
func (p *StackParser) buildHostMapping(stackNode *models.GraphNode, containers []models.ContainerSpec, result *ParseResult) (map[string]string, error) {
	hostMap := make(map[string]string)

	// Check if stack has a default host
	var defaultHostID string
	if stackNode.LocatedInHost != nil {
		defaultHostID = stackNode.LocatedInHost.ID

		// Validate the host exists (Option B: absolute URL resolution)
		if _, err := p.HostResolver.ResolveHost(defaultHostID); err != nil {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("default host %s cannot be resolved: %v", defaultHostID, err))
		}
	}

	// Process each container
	for _, container := range containers {
		var targetHostID string

		// Container-specific host takes precedence
		if container.LocatedInHost != nil {
			targetHostID = container.LocatedInHost.ID

			// Validate the host exists
			if _, err := p.HostResolver.ResolveHost(targetHostID); err != nil {
				result.Errors = append(result.Errors,
					fmt.Sprintf("container %s: host %s cannot be resolved: %v",
						container.Name, targetHostID, err))
				continue
			}
		} else if defaultHostID != "" {
			// Use stack default host
			targetHostID = defaultHostID
		} else {
			// No host specified - will need automatic placement
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("container %s has no host assignment, automatic placement required",
					container.Name))
			targetHostID = "" // Empty means auto-placement needed
		}

		hostMap[container.ID] = targetHostID
	}

	return hostMap, nil
}

// buildTopology extracts host, rack, and datacenter topology from @graph.
func (p *StackParser) buildTopology(graph []models.GraphNode) (*models.Topology, error) {
	topology := &models.Topology{
		Hosts:       make(map[string]*models.GraphNode),
		Racks:       make(map[string]*models.GraphNode),
		Datacenters: make(map[string]*models.GraphNode),
	}

	for i := range graph {
		node := &graph[i]

		switch {
		case p.isNodeType(node.Type, "Host", "Server", "ComputeNode"):
			topology.Hosts[node.ID] = node
		case p.isNodeType(node.Type, "Rack"):
			topology.Racks[node.ID] = node
		case p.isNodeType(node.Type, "Datacenter", "DataCenter"):
			topology.Datacenters[node.ID] = node
		}
	}

	return topology, nil
}

// isNodeType checks if a @type value contains any of the given type names.
func (p *StackParser) isNodeType(typeVal interface{}, names ...string) bool {
	switch t := typeVal.(type) {
	case string:
		for _, name := range names {
			if strings.Contains(t, name) {
				return true
			}
		}
	case []interface{}:
		for _, v := range t {
			if s, ok := v.(string); ok {
				for _, name := range names {
					if strings.Contains(s, name) {
						return true
					}
				}
			}
		}
	}
	return false
}

// buildDependencyGraph creates a deployment wave order based on container dependencies.
func (p *StackParser) buildDependencyGraph(containers []models.ContainerSpec) ([][]string, error) {
	// Build adjacency list and in-degree map
	graph := make(map[string][]string)
	inDegree := make(map[string]int)
	allContainers := make(map[string]bool)

	// Initialize all containers
	for _, container := range containers {
		allContainers[container.Name] = true
		inDegree[container.Name] = 0
		graph[container.Name] = []string{}
	}

	// Build dependency edges
	for _, container := range containers {
		for _, dep := range container.DependsOn {
			// Validate dependency exists
			if !allContainers[dep] {
				return nil, fmt.Errorf("container %s depends on non-existent container %s",
					container.Name, dep)
			}

			// dep -> container (dep must start before container)
			graph[dep] = append(graph[dep], container.Name)
			inDegree[container.Name]++
		}
	}

	// Topological sort using Kahn's algorithm
	var waves [][]string
	queue := []string{}

	// Start with containers that have no dependencies
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}

	// Process waves
	for len(queue) > 0 {
		// Current wave contains all containers with no remaining dependencies
		wave := make([]string, len(queue))
		copy(wave, queue)
		waves = append(waves, wave)

		// Process current wave and prepare next wave
		nextQueue := []string{}
		for _, name := range queue {
			for _, dependent := range graph[name] {
				inDegree[dependent]--
				if inDegree[dependent] == 0 {
					nextQueue = append(nextQueue, dependent)
				}
			}
		}
		queue = nextQueue
	}

	// Check for circular dependencies
	totalProcessed := 0
	for _, wave := range waves {
		totalProcessed += len(wave)
	}
	if totalProcessed != len(containers) {
		return nil, fmt.Errorf("circular dependency detected in container dependencies")
	}

	return waves, nil
}

// validatePlan performs final validation on the complete deployment plan.
func (p *StackParser) validatePlan(plan *models.DeploymentPlan, result *ParseResult) error {
	// Validate stack node
	if plan.StackNode == nil {
		return fmt.Errorf("deployment plan has no stack node")
	}

	if plan.StackNode.Name == "" {
		result.Errors = append(result.Errors, "stack name is required")
	}

	// Validate containers
	for _, container := range plan.ContainerSpecs {
		if err := p.validateContainerSpec(&container, result); err != nil {
			result.Errors = append(result.Errors,
				fmt.Sprintf("container %s: %v", container.Name, err))
		}
	}

	// Validate network if specified
	if plan.Network != nil {
		if plan.Network.Name == "" {
			result.Warnings = append(result.Warnings, "network name is empty")
		}
		if plan.Network.Driver == "" {
			result.Warnings = append(result.Warnings, "network driver not specified, will use default")
		}
	}

	// Return error if there are any fatal errors
	if len(result.Errors) > 0 {
		return fmt.Errorf("validation failed with %d error(s)", len(result.Errors))
	}

	return nil
}

// validateContainerSpec validates a single container specification.
func (p *StackParser) validateContainerSpec(spec *models.ContainerSpec, result *ParseResult) error {
	if spec.Name == "" {
		return fmt.Errorf("container name is required")
	}

	if spec.Image == "" {
		return fmt.Errorf("container image is required")
	}

	// Validate port mappings
	for i, port := range spec.Ports {
		if port.ContainerPort <= 0 || port.ContainerPort > 65535 {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("container %s: invalid container port %d at index %d",
					spec.Name, port.ContainerPort, i))
		}
		if port.HostPort < 0 || port.HostPort > 65535 {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("container %s: invalid host port %d at index %d",
					spec.Name, port.HostPort, i))
		}
		if port.Protocol == "" {
			spec.Ports[i].Protocol = "tcp" // Default to TCP
		}
	}

	// Validate volume mounts
	for i, vol := range spec.VolumeMounts {
		if vol.Source == "" {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("container %s: volume mount at index %d has empty source",
					spec.Name, i))
		}
		if vol.Target == "" {
			return fmt.Errorf("volume mount at index %d has empty target path", i)
		}
		if vol.Type == "" {
			spec.VolumeMounts[i].Type = "volume" // Default to volume
		}
	}

	// Validate health check
	if spec.HealthCheck != nil {
		if spec.HealthCheck.Type == "" {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("container %s: health check type not specified", spec.Name))
		}
		if spec.HealthCheck.Interval <= 0 {
			spec.HealthCheck.Interval = 30 // Default 30s
		}
		if spec.HealthCheck.Timeout <= 0 {
			spec.HealthCheck.Timeout = 30 // Default 30s
		}
		if spec.HealthCheck.Retries <= 0 {
			spec.HealthCheck.Retries = 3 // Default 3 retries
		}
	}

	// Validate restart policy
	if spec.RestartPolicy != "" {
		validPolicies := map[string]bool{
			"no":             true,
			"always":         true,
			"on-failure":     true,
			"unless-stopped": true,
		}
		if !validPolicies[spec.RestartPolicy] {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("container %s: invalid restart policy %s, will use default",
					spec.Name, spec.RestartPolicy))
		}
	}

	return nil
}

// GetContainersByWave returns containers grouped by deployment wave.
func (p *StackParser) GetContainersByWave(plan *models.DeploymentPlan) [][]models.ContainerSpec {
	if plan == nil || len(plan.DependencyGraph) == 0 {
		return [][]models.ContainerSpec{plan.ContainerSpecs}
	}

	// Create a map for quick lookup
	containerMap := make(map[string]models.ContainerSpec)
	for _, spec := range plan.ContainerSpecs {
		containerMap[spec.Name] = spec
	}

	// Build waves
	var waves [][]models.ContainerSpec
	for _, waveNames := range plan.DependencyGraph {
		wave := make([]models.ContainerSpec, 0, len(waveNames))
		for _, name := range waveNames {
			if spec, ok := containerMap[name]; ok {
				wave = append(wave, spec)
			}
		}
		if len(wave) > 0 {
			waves = append(waves, wave)
		}
	}

	return waves
}
