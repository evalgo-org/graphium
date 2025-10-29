# EVE Distributed Deployment Requirements

## Overview

This document outlines what needs to be implemented in **EVE library itself** to support native distributed stack deployments across multiple Docker hosts.

**Current State**: EVE deploys to a single Docker daemon
**Target State**: EVE can orchestrate deployments across multiple Docker daemons

---

## Current EVE Architecture

### Single-Host Design

```go
// Current EVE API - Single Docker client
func DeployStack(
    ctx context.Context,
    cli common.DockerClient,      // ← Single Docker connection
    stack *stacks.Stack,
) (*StackDeployment, error)
```

**How it works:**
1. Connect to **ONE** Docker daemon (`unix:///var/run/docker.sock`)
2. Deploy **ALL** containers to that daemon
3. Use Docker's built-in networking (same host)
4. Simple and reliable for single-host scenarios

**Limitations:**
- ❌ Cannot deploy to multiple hosts
- ❌ Cannot distribute load across hosts
- ❌ No cross-host coordination
- ❌ Single point of failure

---

## Required Changes to EVE

### 1. Multi-Client Support

**File**: `eve.evalgo.org/common/docker.go` (modification)

#### Current Implementation

```go
// Current: Single client only
func CtxCli(dockerHost string) (context.Context, *client.Client) {
    cli, err := client.NewClientWithOpts(
        client.FromEnv,
        client.WithHost(dockerHost),
    )
    return context.Background(), cli
}
```

#### Proposed: Multi-Client Manager

```go
// New: Multi-client manager
package common

import (
    "context"
    "sync"
    "github.com/docker/docker/client"
)

// DockerClientManager manages connections to multiple Docker hosts
type DockerClientManager struct {
    clients map[string]*client.Client
    mu      sync.RWMutex
}

// NewDockerClientManager creates a new multi-host client manager
func NewDockerClientManager() *DockerClientManager {
    return &DockerClientManager{
        clients: make(map[string]*client.Client),
    }
}

// AddHost adds a Docker host connection
func (m *DockerClientManager) AddHost(hostID, dockerHost string) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    cli, err := client.NewClientWithOpts(
        client.WithHost(dockerHost),
        client.WithAPIVersionNegotiation(),
    )
    if err != nil {
        return fmt.Errorf("failed to connect to %s: %w", hostID, err)
    }

    m.clients[hostID] = cli
    return nil
}

// GetClient returns a client for a specific host
func (m *DockerClientManager) GetClient(hostID string) (*client.Client, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    cli, exists := m.clients[hostID]
    if !exists {
        return nil, fmt.Errorf("no client for host: %s", hostID)
    }
    return cli, nil
}

// Close closes all client connections
func (m *DockerClientManager) Close() error {
    m.mu.Lock()
    defer m.mu.Unlock()

    for _, cli := range m.clients {
        cli.Close()
    }
    m.clients = make(map[string]*client.Client)
    return nil
}

// HostInfo represents information about a Docker host
type HostInfo struct {
    ID         string
    DockerHost string // tcp://192.168.1.10:2376 or unix:///var/run/docker.sock
    IPAddress  string
    Datacenter string
}
```

**Estimated Effort**: 2-3 hours

---

### 2. Extended Stack Definition (schema.org)

**File**: `eve.evalgo.org/containers/stacks/stack.go` (modification)

#### Current Schema

```go
type Stack struct {
    Context         string             `json:"@context"`
    Type            string             `json:"@type"`
    Name            string             `json:"name"`
    ItemListElement []StackItemElement `json:"itemListElement"`
    Network         NetworkConfig      `json:"network,omitempty"`
    Volumes         []VolumeConfig     `json:"volumes,omitempty"`
}
```

#### Proposed: Add Placement Metadata

```go
type Stack struct {
    Context         string             `json:"@context"`
    Type            string             `json:"@type"`
    Name            string             `json:"name"`
    ItemListElement []StackItemElement `json:"itemListElement"`
    Network         NetworkConfig      `json:"network,omitempty"`
    Volumes         []VolumeConfig     `json:"volumes,omitempty"`

    // NEW: Multi-host deployment configuration
    Deployment      DeploymentConfig   `json:"deployment,omitempty"`
}

// DeploymentConfig configures multi-host deployment
type DeploymentConfig struct {
    // Mode: "single-host" or "multi-host"
    Mode string `json:"mode"` // Default: "single-host"

    // PlacementStrategy: "auto", "manual", "datacenter", "spread"
    PlacementStrategy string `json:"placementStrategy,omitempty"`

    // HostConstraints for automatic placement
    HostConstraints []HostConstraint `json:"hostConstraints,omitempty"`
}

type HostConstraint struct {
    // MinCPU minimum CPU cores required
    MinCPU int `json:"minCPU,omitempty"`
    // MinMemory minimum memory in MB
    MinMemory int64 `json:"minMemory,omitempty"`
    // RequiredLabels host must have these labels
    RequiredLabels map[string]string `json:"requiredLabels,omitempty"`
    // PreferredDatacenter prefer hosts in this datacenter
    PreferredDatacenter string `json:"preferredDatacenter,omitempty"`
}
```

#### Extended StackItemElement

```go
type StackItemElement struct {
    // ... existing fields ...

    // NEW: Placement hints
    TargetHost       string            `json:"targetHost,omitempty"`       // Manual placement
    HostConstraints  *HostConstraint   `json:"hostConstraints,omitempty"`  // Auto placement hints

    // NEW: Cross-host connectivity
    ServiceEndpoint  *ServiceEndpoint  `json:"serviceEndpoint,omitempty"`  // How to reach this service
}

// ServiceEndpoint defines how other services can reach this one
type ServiceEndpoint struct {
    // Type: "host-port", "overlay", "external"
    Type string `json:"type"`

    // For host-port type
    ExposedPort int    `json:"exposedPort,omitempty"`

    // Connection template for environment variables
    // Example: "postgres://{{HOST}}:{{PORT}}/{{DATABASE}}"
    ConnectionTemplate string `json:"connectionTemplate,omitempty"`
}
```

**Example Extended Stack Definition:**

```json
{
  "@context": "https://schema.org",
  "@type": "ItemList",
  "name": "distributed-app",
  "deployment": {
    "mode": "multi-host",
    "placementStrategy": "manual"
  },
  "itemListElement": [
    {
      "@type": "SoftwareApplication",
      "position": 1,
      "name": "postgres",
      "image": "postgres:17",
      "targetHost": "db-host-01",
      "serviceEndpoint": {
        "type": "host-port",
        "exposedPort": 5432,
        "connectionTemplate": "postgres://{{HOST}}:{{PORT}}/mydb"
      },
      "ports": [
        {"containerPort": 5432, "hostPort": 5432}
      ]
    },
    {
      "@type": "SoftwareApplication",
      "position": 2,
      "name": "app",
      "image": "myapp:latest",
      "targetHost": "app-host-01",
      "softwareRequirements": [
        {
          "name": "postgres",
          "waitForHealthy": true
        }
      ]
    }
  ]
}
```

**Estimated Effort**: 3-4 hours

---

### 3. Distributed Deployment Engine

**File**: `eve.evalgo.org/containers/stacks/distributed/deployment.go` (new package)

```go
package distributed

import (
    "context"
    "fmt"
    "eve.evalgo.org/common"
    "eve.evalgo.org/containers/stacks"
)

// DistributedStackDeployment represents a multi-host deployment
type DistributedStackDeployment struct {
    Stack        *stacks.Stack
    Deployments  map[string]*HostDeployment  // hostID -> deployment
    StartTime    time.Time
}

// HostDeployment represents containers deployed to one host
type HostDeployment struct {
    HostID     string
    Containers map[string]string  // serviceName -> containerID
    NetworkID  string
}

// DeployDistributedStack deploys a stack across multiple Docker hosts
func DeployDistributedStack(
    ctx context.Context,
    manager *common.DockerClientManager,
    stack *stacks.Stack,
    hosts []common.HostInfo,
) (*DistributedStackDeployment, error) {

    if stack.Deployment.Mode != "multi-host" {
        return nil, fmt.Errorf("stack is not configured for multi-host deployment")
    }

    // 1. Validate stack
    if err := stack.Validate(); err != nil {
        return nil, fmt.Errorf("invalid stack: %w", err)
    }

    // 2. Determine placement
    placement, err := determinePlacement(stack, hosts)
    if err != nil {
        return nil, fmt.Errorf("placement failed: %w", err)
    }

    // 3. Prepare cross-host networking
    networkConfig := prepareCrossHostNetworking(stack, placement, hosts)

    // 4. Deploy to each host in dependency order
    deployment := &DistributedStackDeployment{
        Stack:       stack,
        Deployments: make(map[string]*HostDeployment),
        StartTime:   time.Now(),
    }

    // Get startup order (respects dependencies)
    orderedServices := stack.GetStartupOrder()

    for _, service := range orderedServices {
        hostID := placement[service.Name]

        // Get Docker client for target host
        cli, err := manager.GetClient(hostID)
        if err != nil {
            return nil, fmt.Errorf("failed to get client for %s: %w", hostID, err)
        }

        // Wait for dependencies (may be on other hosts)
        if err := waitForCrossHostDependencies(ctx, manager, service, deployment); err != nil {
            return nil, fmt.Errorf("dependency wait failed for %s: %w", service.Name, err)
        }

        // Inject cross-host connection strings
        service.Environment = injectConnectionStrings(service, networkConfig, deployment)

        // Deploy container to target host
        containerID, err := deployContainerToHost(ctx, cli, service, stack)
        if err != nil {
            return nil, fmt.Errorf("failed to deploy %s to %s: %w",
                service.Name, hostID, err)
        }

        // Record deployment
        if deployment.Deployments[hostID] == nil {
            deployment.Deployments[hostID] = &HostDeployment{
                HostID:     hostID,
                Containers: make(map[string]string),
            }
        }
        deployment.Deployments[hostID].Containers[service.Name] = containerID

        // Wait for health check
        if err := waitForContainerHealth(ctx, cli, containerID, service.HealthCheck); err != nil {
            return nil, fmt.Errorf("health check failed for %s: %w", service.Name, err)
        }

        // Execute post-start actions
        for _, action := range service.PotentialAction {
            if err := executePostStartAction(ctx, cli, containerID, action); err != nil {
                return nil, fmt.Errorf("post-start action failed for %s: %w",
                    service.Name, err)
            }
        }
    }

    return deployment, nil
}

// determinePlacement decides which host each service should run on
func determinePlacement(
    stack *stacks.Stack,
    hosts []common.HostInfo,
) (map[string]string, error) {

    placement := make(map[string]string)

    switch stack.Deployment.PlacementStrategy {
    case "manual":
        // Use targetHost from each service
        for _, service := range stack.ItemListElement {
            if service.TargetHost == "" {
                return nil, fmt.Errorf("manual placement but no targetHost for %s",
                    service.Name)
            }
            placement[service.Name] = service.TargetHost
        }

    case "auto":
        // Automatic placement based on constraints
        for _, service := range stack.ItemListElement {
            host, err := selectOptimalHost(service, hosts)
            if err != nil {
                return nil, fmt.Errorf("failed to select host for %s: %w",
                    service.Name, err)
            }
            placement[service.Name] = host.ID
        }

    case "spread":
        // Distribute evenly across hosts
        for i, service := range stack.ItemListElement {
            placement[service.Name] = hosts[i%len(hosts)].ID
        }

    case "datacenter":
        // Keep in same datacenter, spread across hosts
        // Implementation would filter hosts by datacenter first
        dcHosts := filterHostsByDatacenter(hosts, stack.Deployment.HostConstraints)
        for i, service := range stack.ItemListElement {
            placement[service.Name] = dcHosts[i%len(dcHosts)].ID
        }

    default:
        return nil, fmt.Errorf("unknown placement strategy: %s",
            stack.Deployment.PlacementStrategy)
    }

    return placement, nil
}

// selectOptimalHost chooses the best host for a service
func selectOptimalHost(
    service stacks.StackItemElement,
    hosts []common.HostInfo,
) (*common.HostInfo, error) {

    // Filter hosts by constraints
    candidates := filterHostsByConstraints(hosts, service.HostConstraints)
    if len(candidates) == 0 {
        return nil, fmt.Errorf("no hosts match constraints for %s", service.Name)
    }

    // Score each candidate
    bestHost := candidates[0]
    bestScore := scoreHost(candidates[0], service)

    for _, host := range candidates[1:] {
        score := scoreHost(host, service)
        if score > bestScore {
            bestScore = score
            bestHost = host
        }
    }

    return &bestHost, nil
}

// prepareCrossHostNetworking sets up cross-host connectivity
func prepareCrossHostNetworking(
    stack *stacks.Stack,
    placement map[string]string,
    hosts []common.HostInfo,
) *NetworkConfig {

    config := &NetworkConfig{
        Mode:      "host-ports",
        Endpoints: make(map[string]Endpoint),
    }

    // Build map of hostID -> HostInfo
    hostMap := make(map[string]common.HostInfo)
    for _, host := range hosts {
        hostMap[host.ID] = host
    }

    // For each service, determine its endpoint
    for _, service := range stack.ItemListElement {
        hostID := placement[service.Name]
        host := hostMap[hostID]

        if service.ServiceEndpoint != nil && service.ServiceEndpoint.Type == "host-port" {
            config.Endpoints[service.Name] = Endpoint{
                Host:     host.IPAddress,
                Port:     service.ServiceEndpoint.ExposedPort,
                Template: service.ServiceEndpoint.ConnectionTemplate,
            }
        }
    }

    return config
}

// injectConnectionStrings adds cross-host connection info to environment
func injectConnectionStrings(
    service stacks.StackItemElement,
    networkConfig *NetworkConfig,
    deployment *DistributedStackDeployment,
) map[string]string {

    env := make(map[string]string)
    for k, v := range service.Environment {
        env[k] = v
    }

    // For each dependency, inject connection info
    for _, dep := range service.SoftwareRequirements {
        endpoint, exists := networkConfig.Endpoints[dep.Name]
        if !exists {
            continue
        }

        // Inject HOST and PORT env vars
        envPrefix := strings.ToUpper(dep.Name)
        env[envPrefix+"_HOST"] = endpoint.Host
        env[envPrefix+"_PORT"] = fmt.Sprintf("%d", endpoint.Port)

        // If there's a connection template, render it
        if endpoint.Template != "" {
            url := strings.ReplaceAll(endpoint.Template, "{{HOST}}", endpoint.Host)
            url = strings.ReplaceAll(url, "{{PORT}}", fmt.Sprintf("%d", endpoint.Port))
            env[envPrefix+"_URL"] = url
        }
    }

    return env
}

// waitForCrossHostDependencies waits for dependencies on other hosts
func waitForCrossHostDependencies(
    ctx context.Context,
    manager *common.DockerClientManager,
    service stacks.StackItemElement,
    deployment *DistributedStackDeployment,
) error {

    for _, dep := range service.SoftwareRequirements {
        if !dep.WaitForHealthy {
            continue
        }

        // Find which host the dependency is on
        var depContainerID string
        var depHostID string

        for hostID, hostDep := range deployment.Deployments {
            if containerID, exists := hostDep.Containers[dep.Name]; exists {
                depContainerID = containerID
                depHostID = hostID
                break
            }
        }

        if depContainerID == "" {
            return fmt.Errorf("dependency %s not yet deployed", dep.Name)
        }

        // Get client for the host running the dependency
        cli, err := manager.GetClient(depHostID)
        if err != nil {
            return fmt.Errorf("failed to get client for dependency host: %w", err)
        }

        // Wait for health check
        if err := waitForContainerHealth(ctx, cli, depContainerID,
            getDependencyHealthCheck(service, dep.Name)); err != nil {
            return fmt.Errorf("dependency %s health check failed: %w", dep.Name, err)
        }
    }

    return nil
}

type NetworkConfig struct {
    Mode      string
    Endpoints map[string]Endpoint
}

type Endpoint struct {
    Host     string
    Port     int
    Template string
}

// StopDistributedStack stops all containers in a distributed stack
func StopDistributedStack(
    ctx context.Context,
    manager *common.DockerClientManager,
    deployment *DistributedStackDeployment,
) error {

    // Stop in reverse order
    orderedServices := deployment.Stack.GetStartupOrder()
    for i := len(orderedServices) - 1; i >= 0; i-- {
        service := orderedServices[i]

        // Find which host this service is on
        for hostID, hostDep := range deployment.Deployments {
            if containerID, exists := hostDep.Containers[service.Name]; exists {
                cli, err := manager.GetClient(hostID)
                if err != nil {
                    return err
                }

                timeout := 30
                if err := cli.ContainerStop(ctx, containerID,
                    container.StopOptions{Timeout: &timeout}); err != nil {
                    return fmt.Errorf("failed to stop %s: %w", service.Name, err)
                }
            }
        }
    }

    return nil
}

// RemoveDistributedStack removes all containers in a distributed stack
func RemoveDistributedStack(
    ctx context.Context,
    manager *common.DockerClientManager,
    deployment *DistributedStackDeployment,
    removeVolumes bool,
) error {

    // Stop first
    if err := StopDistributedStack(ctx, manager, deployment); err != nil {
        return err
    }

    // Remove containers from each host
    for hostID, hostDep := range deployment.Deployments {
        cli, err := manager.GetClient(hostID)
        if err != nil {
            return err
        }

        for _, containerID := range hostDep.Containers {
            if err := cli.ContainerRemove(ctx, containerID,
                container.RemoveOptions{Force: true}); err != nil {
                return fmt.Errorf("failed to remove container: %w", err)
            }
        }

        // Optionally remove volumes
        if removeVolumes && hostDep.NetworkID != "" {
            // Remove volumes logic
        }
    }

    return nil
}
```

**Estimated Effort**: 2-3 days

---

### 4. Host Discovery Interface

**File**: `eve.evalgo.org/containers/stacks/distributed/discovery.go` (new)

```go
package distributed

// HostProvider interface for discovering available Docker hosts
type HostProvider interface {
    // ListHosts returns all available Docker hosts
    ListHosts(ctx context.Context) ([]common.HostInfo, error)

    // GetHost returns information about a specific host
    GetHost(ctx context.Context, hostID string) (*common.HostInfo, error)

    // GetHostsByDatacenter returns hosts in a specific datacenter
    GetHostsByDatacenter(ctx context.Context, datacenter string) ([]common.HostInfo, error)
}

// StaticHostProvider provides a fixed list of hosts
type StaticHostProvider struct {
    hosts []common.HostInfo
}

func NewStaticHostProvider(hosts []common.HostInfo) *StaticHostProvider {
    return &StaticHostProvider{hosts: hosts}
}

func (p *StaticHostProvider) ListHosts(ctx context.Context) ([]common.HostInfo, error) {
    return p.hosts, nil
}

func (p *StaticHostProvider) GetHost(ctx context.Context, hostID string) (*common.HostInfo, error) {
    for _, host := range p.hosts {
        if host.ID == hostID {
            return &host, nil
        }
    }
    return nil, fmt.Errorf("host not found: %s", hostID)
}

func (p *StaticHostProvider) GetHostsByDatacenter(ctx context.Context, datacenter string) ([]common.HostInfo, error) {
    var result []common.HostInfo
    for _, host := range p.hosts {
        if host.Datacenter == datacenter {
            result = append(result, host)
        }
    }
    return result, nil
}

// DynamicHostProvider could integrate with Graphium's storage
type DynamicHostProvider struct {
    endpoint string // Graphium API endpoint
}

func NewDynamicHostProvider(endpoint string) *DynamicHostProvider {
    return &DynamicHostProvider{endpoint: endpoint}
}

func (p *DynamicHostProvider) ListHosts(ctx context.Context) ([]common.HostInfo, error) {
    // Make HTTP request to Graphium API
    resp, err := http.Get(p.endpoint + "/api/v1/hosts")
    // ... parse and return hosts
    return nil, nil
}
```

**Estimated Effort**: 4-6 hours

---

### 5. Distributed Health Checks

**File**: `eve.evalgo.org/containers/stacks/distributed/health.go` (new)

```go
package distributed

import (
    "context"
    "fmt"
    "net/http"
    "time"
)

// CrossHostHealthChecker checks health of services across multiple hosts
type CrossHostHealthChecker struct {
    manager *common.DockerClientManager
}

func NewCrossHostHealthChecker(manager *common.DockerClientManager) *CrossHostHealthChecker {
    return &CrossHostHealthChecker{manager: manager}
}

// CheckServiceHealth checks if a service is healthy on its host
func (c *CrossHostHealthChecker) CheckServiceHealth(
    ctx context.Context,
    hostID string,
    containerID string,
    healthCheck stacks.HealthCheckConfig,
) error {

    cli, err := c.manager.GetClient(hostID)
    if err != nil {
        return fmt.Errorf("failed to get client: %w", err)
    }

    switch healthCheck.Type {
    case "http":
        return c.checkHTTPHealth(ctx, hostID, containerID, healthCheck)
    case "tcp":
        return c.checkTCPHealth(ctx, hostID, containerID, healthCheck)
    case "command":
        return c.checkCommandHealth(ctx, cli, containerID, healthCheck)
    default:
        return fmt.Errorf("unknown health check type: %s", healthCheck.Type)
    }
}

// checkHTTPHealth performs HTTP health check (may be cross-host)
func (c *CrossHostHealthChecker) checkHTTPHealth(
    ctx context.Context,
    hostID string,
    containerID string,
    healthCheck stacks.HealthCheckConfig,
) error {

    // Get container's exposed port and host IP
    endpoint := c.getContainerEndpoint(ctx, hostID, containerID, healthCheck.Port)

    url := fmt.Sprintf("http://%s:%d%s", endpoint.Host, endpoint.Port, healthCheck.Path)

    timeout := time.Duration(healthCheck.Timeout) * time.Second
    client := &http.Client{Timeout: timeout}

    resp, err := client.Get(url)
    if err != nil {
        return fmt.Errorf("http health check failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("unhealthy status: %d", resp.StatusCode)
    }

    return nil
}

// getContainerEndpoint determines how to reach a container's port
func (c *CrossHostHealthChecker) getContainerEndpoint(
    ctx context.Context,
    hostID string,
    containerID string,
    port int,
) Endpoint {
    // Query Docker API to get container's port mapping
    // Return host IP + mapped port
    return Endpoint{}
}
```

**Estimated Effort**: 3-4 hours

---

## Summary of Required Changes

### New Packages

1. **`eve.evalgo.org/containers/stacks/distributed`** (new package)
   - `deployment.go` - Core distributed deployment logic
   - `discovery.go` - Host discovery interfaces
   - `health.go` - Cross-host health checking
   - `placement.go` - Placement algorithms
   - `networking.go` - Cross-host networking
   - **Total**: ~1,500 lines

### Modified Packages

2. **`eve.evalgo.org/common`** (modifications)
   - Add `DockerClientManager` for multi-host connections
   - Add `HostInfo` structure
   - **Changes**: ~200 lines

3. **`eve.evalgo.org/containers/stacks`** (modifications)
   - Extend `Stack` with `DeploymentConfig`
   - Extend `StackItemElement` with placement hints
   - Add `ServiceEndpoint` structure
   - **Changes**: ~150 lines

### New API Surface

```go
// Distributed deployment API
import "eve.evalgo.org/containers/stacks/distributed"

// Setup
manager := common.NewDockerClientManager()
manager.AddHost("host-1", "tcp://192.168.1.10:2376")
manager.AddHost("host-2", "tcp://192.168.1.11:2376")
manager.AddHost("host-3", "tcp://192.168.1.12:2376")
defer manager.Close()

hosts := []common.HostInfo{
    {ID: "host-1", IPAddress: "192.168.1.10", DockerHost: "tcp://192.168.1.10:2376"},
    {ID: "host-2", IPAddress: "192.168.1.11", DockerHost: "tcp://192.168.1.11:2376"},
    {ID: "host-3", IPAddress: "192.168.1.12", DockerHost: "tcp://192.168.1.12:2376"},
}

// Deploy
deployment, err := distributed.DeployDistributedStack(ctx, manager, stack, hosts)

// Stop
err = distributed.StopDistributedStack(ctx, manager, deployment)

// Remove
err = distributed.RemoveDistributedStack(ctx, manager, deployment, false)
```

---

## Effort Estimation

### By Component

| Component | Files | Lines | Effort |
|-----------|-------|-------|--------|
| Multi-Client Manager | 1 | ~200 | 2-3 hours |
| Extended Stack Schema | 1 | ~150 | 3-4 hours |
| Distributed Deployment Engine | 1 | ~800 | 2 days |
| Host Discovery | 1 | ~200 | 4-6 hours |
| Cross-Host Health Checks | 1 | ~200 | 3-4 hours |
| Placement Algorithms | 1 | ~300 | 1 day |
| Cross-Host Networking | 1 | ~200 | 4-6 hours |
| Tests | 5+ | ~1000 | 1 day |
| Documentation | - | - | 4-6 hours |

**Total Effort**: ~5-7 days for complete implementation

### Phased Approach

#### Phase 1: Foundation (2 days)
- Multi-client manager
- Extended stack schema
- Basic distributed deployment (manual placement only)
- Host-port networking

#### Phase 2: Intelligence (2 days)
- Auto placement with constraints
- Smart host selection
- Resource-based scoring

#### Phase 3: Reliability (2 days)
- Cross-host health checks
- Robust error handling
- Rollback on failure
- Comprehensive testing

#### Phase 4: Polish (1 day)
- Documentation
- Examples
- Performance optimization

---

## Design Decisions

### 1. Networking Strategy

**Recommended: Host-Port (Exposed Ports)**

**Pros:**
- ✅ Works with vanilla Docker (no Swarm needed)
- ✅ Simple and reliable
- ✅ Easy to debug
- ✅ Graphium already tracks host IPs

**Alternative: Overlay Networks**
- Requires Docker Swarm or external tool
- More complex but more seamless

**Decision**: Start with host-port, add overlay as optional later

### 2. Placement Strategy

**Provide multiple strategies:**
1. **Manual** - User specifies exact host (Phase 1)
2. **Auto** - EVE selects based on constraints (Phase 2)
3. **Spread** - Distribute evenly (Phase 2)
4. **Datacenter** - Keep in same DC (Phase 2)

**Decision**: Implement all, default to manual

### 3. Backward Compatibility

**Critical**: Existing single-host API must continue working

```go
// Existing API - MUST still work
func DeployStack(ctx context.Context, cli common.DockerClient, stack *stacks.Stack)

// New API - Additional functionality
func DeployDistributedStack(ctx context.Context, manager *common.DockerClientManager,
    stack *stacks.Stack, hosts []common.HostInfo)
```

**Decision**: Keep both APIs, detect `deployment.mode` in stack definition

### 4. Host Discovery

**Pluggable architecture** via `HostProvider` interface

**Implementations:**
- `StaticHostProvider` - Fixed list (Phase 1)
- `DynamicHostProvider` - Query API like Graphium (Phase 2)
- `KubernetesProvider` - Kubernetes nodes (Future)
- `ConsulProvider` - Consul service discovery (Future)

**Decision**: Start with static, make pluggable for future extensions

---

## Testing Strategy

### Unit Tests

```go
func TestMultiClientManager(t *testing.T)
func TestPlacementStrategies(t *testing.T)
func TestCrossHostNetworking(t *testing.T)
func TestDistributedHealthChecks(t *testing.T)
```

### Integration Tests

```go
func TestDistributedStackDeployment(t *testing.T) {
    // Setup 3 Docker daemons (testcontainers-go with Docker-in-Docker)
    hosts := setupMultiHostEnvironment(t)

    // Deploy stack across hosts
    deployment, err := DeployDistributedStack(ctx, manager, stack, hosts)

    // Verify containers on correct hosts
    // Verify cross-host connectivity
    // Verify health checks work
}
```

### Example Test Stack

```json
{
  "@context": "https://schema.org",
  "@type": "ItemList",
  "name": "test-distributed",
  "deployment": {
    "mode": "multi-host",
    "placementStrategy": "manual"
  },
  "itemListElement": [
    {
      "name": "db",
      "targetHost": "host-1",
      "serviceEndpoint": {
        "type": "host-port",
        "exposedPort": 5432
      }
    },
    {
      "name": "app",
      "targetHost": "host-2",
      "softwareRequirements": [
        {"name": "db", "waitForHealthy": true}
      ]
    }
  ]
}
```

---

## Breaking Changes

### None (Fully Backward Compatible)

**Existing users** can continue using:
```go
// Single-host deployment (unchanged)
deployment, err := production.DeployStack(ctx, cli, stack)
```

**New users** can opt-in to distributed:
```json
{
  "deployment": {
    "mode": "multi-host"
  }
}
```

If `deployment.mode` is not specified or is `"single-host"`, EVE uses current behavior.

---

## Migration Path for Graphium

### Before (Graphium extends EVE)

Graphium implements distributed deployment in its own orchestrator layer.

**Pros:**
- ✅ Can start immediately
- ✅ EVE remains simple

**Cons:**
- ❌ Duplicate code if multiple projects need this
- ❌ Not reusable by other EVE users

### After (EVE supports distributed natively)

Graphium uses EVE's distributed deployment package.

**Pros:**
- ✅ Shared implementation
- ✅ Other projects benefit
- ✅ More testing (more users)
- ✅ Standardized approach

**Cons:**
- ❌ Requires EVE changes first
- ❌ More complex EVE library

### Recommended Approach

**Phase 1**: Graphium implements prototype (1-2 weeks)
- Proves the concept
- Identifies edge cases
- Validates API design

**Phase 2**: Move to EVE (1-2 weeks)
- Extract Graphium's implementation
- Add to EVE library
- Graphium migrates to use EVE's version

**Total**: 3-4 weeks end-to-end

---

## Conclusion

### What Needs to Be Added to EVE

1. **Multi-Client Manager** - Connect to multiple Docker daemons
2. **Extended Stack Schema** - Add placement and endpoint metadata
3. **Distributed Deployment Engine** - Orchestrate across hosts
4. **Host Discovery** - Pluggable host provider interface
5. **Cross-Host Health Checks** - Verify health across hosts
6. **Placement Algorithms** - Smart host selection

### Effort

- **Core Features**: 5-7 days
- **With Tests & Docs**: 8-10 days
- **Production Ready**: 2-3 weeks

### Recommendation

**Option 1 (Faster)**: Implement in Graphium first
- Start coding today
- Prototype in 1-2 weeks
- Later extract to EVE

**Option 2 (Better Long-term)**: Implement in EVE
- Design carefully
- 2-3 weeks for production-ready
- All EVE users benefit

**My Recommendation**: **Start with Graphium prototype**, then move to EVE if successful. This validates the design before committing to EVE's API surface.
