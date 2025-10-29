# Distributed Stack Deployment for Graphium

## Question: Can stacks be deployed across multiple hosts?

**Short Answer**: EVE's current implementation deploys to a **single host**, but **Graphium can extend this** to support distributed deployment across multiple hosts.

---

## Current EVE Behavior (Single Host)

### How EVE Stacks Work Today

EVE's stack deployment uses a single Docker API client:

```go
// Connects to ONE Docker daemon
ctx, cli := common.CtxCli("unix:///var/run/docker.sock")
defer cli.Close()

// Deploys ALL containers to that single Docker daemon
deployment, err := production.DeployStack(ctx, cli, stack)
```

**Current Characteristics:**
- ✅ All containers in a stack run on the **same Docker host**
- ✅ Containers communicate via **Docker's built-in DNS** (same network)
- ✅ Simple networking (no cross-host complexity)
- ✅ Fast deployment (no network overhead)
- ❌ **Cannot distribute** containers across multiple hosts

**Use Cases:**
- ✅ Perfect for: Development environments, single-server deployments, small stacks
- ❌ Not suitable for: Large-scale production, high availability, load distribution

---

## Graphium Extension: Distributed Deployment

### Architecture Overview

Since **Graphium is already a multi-host orchestration platform**, it can extend EVE stacks to support distributed deployment:

```
┌─────────────────────────────────────────────────────────┐
│ Graphium Stack Orchestrator                             │
│ (Extends EVE with multi-host awareness)                 │
└─────────────────────────────────────────────────────────┘
           │
           ├─────────────┬─────────────┬──────────────┐
           ▼             ▼             ▼              ▼
    ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐
    │ Host 1   │  │ Host 2   │  │ Host 3   │  │ Host 4   │
    │ us-west  │  │ us-west  │  │ us-east  │  │ eu-cent  │
    └──────────┘  └──────────┘  └──────────┘  └──────────┘
         │              │             │              │
    container1    container2    container3    container4
```

### Enhanced Stack Model

Extend the stack definition to include host placement hints:

```go
// models/stack.go - Extended for distributed deployment
type Stack struct {
    // ... existing fields ...

    // Distributed deployment configuration
    Placement     PlacementStrategy      `json:"placement,omitempty"`
    HostMapping   map[string]string      `json:"hostMapping,omitempty"` // service -> hostID
}

type PlacementStrategy string

const (
    PlacementAuto       PlacementStrategy = "auto"       // Graphium decides
    PlacementManual     PlacementStrategy = "manual"     // User specifies
    PlacementDatacenter PlacementStrategy = "datacenter" // Keep in same DC
    PlacementSpread     PlacementStrategy = "spread"     // Distribute evenly
)
```

### Stack Definition with Host Hints

**schema.org Extension:**

```json
{
  "@context": "https://schema.org",
  "@type": "ItemList",
  "name": "distributed-app",
  "placement": "manual",
  "itemListElement": [
    {
      "@type": "SoftwareApplication",
      "position": 1,
      "name": "postgres",
      "image": "postgres:17",
      "targetHost": "host-1"    // ← Deployment hint
    },
    {
      "@type": "SoftwareApplication",
      "position": 2,
      "name": "cache",
      "image": "dragonflydb/dragonfly:v1.26.1",
      "targetHost": "host-2"    // ← Different host
    },
    {
      "@type": "SoftwareApplication",
      "position": 3,
      "name": "app",
      "image": "myapp:latest",
      "targetHost": "host-3",   // ← Yet another host
      "softwareRequirements": [
        {
          "name": "postgres",
          "remoteHost": "host-1",      // ← Cross-host dependency
          "connectionString": "postgres://host-1:5432"
        },
        {
          "name": "cache",
          "remoteHost": "host-2",
          "connectionString": "redis://host-2:6379"
        }
      ]
    }
  ]
}
```

---

## Implementation: Distributed Orchestrator

### Architecture Components

```
┌─────────────────────────────────────────────────────────┐
│ Graphium Distributed Stack Orchestrator                 │
├─────────────────────────────────────────────────────────┤
│ 1. Stack Parser                                          │
│    - Parse schema.org ItemList                           │
│    - Extract container definitions                       │
│    - Identify dependencies                               │
├─────────────────────────────────────────────────────────┤
│ 2. Placement Engine                                      │
│    - Auto: Select optimal hosts                          │
│    - Manual: Use specified targetHost                    │
│    - Constraints: CPU, memory, datacenter                │
├─────────────────────────────────────────────────────────┤
│ 3. Multi-Host Deployer                                   │
│    - Deploy to each host via EVE                         │
│    - Track deployment state per host                     │
│    - Handle cross-host networking                        │
├─────────────────────────────────────────────────────────┤
│ 4. Cross-Host Networking                                 │
│    - Configure overlay networks OR                       │
│    - Use exposed ports + host IPs                        │
│    - Update env vars with connection strings             │
├─────────────────────────────────────────────────────────┤
│ 5. Health & Coordination                                 │
│    - Wait for cross-host dependencies                    │
│    - Monitor health across hosts                         │
│    - Handle failures and rollback                        │
└─────────────────────────────────────────────────────────┘
```

### Code Implementation

**File**: `internal/orchestrator/distributed_stack.go` (new file)

```go
package orchestrator

import (
    "context"
    "fmt"
    "evalgo.org/graphium/models"
    "evalgo.org/graphium/internal/storage"
    "eve.evalgo.org/common"
    "eve.evalgo.org/containers/stacks"
    "eve.evalgo.org/containers/production"
)

type DistributedStackOrchestrator struct {
    storage *storage.Storage
}

// DeployDistributedStack deploys a stack across multiple hosts
func (o *DistributedStackOrchestrator) DeployDistributedStack(
    stack *models.Stack,
    definition *stacks.Stack,
) error {
    // 1. Determine placement for each container
    placement, err := o.determinePlacement(stack, definition)
    if err != nil {
        return fmt.Errorf("placement failed: %w", err)
    }

    // 2. Group containers by target host
    hostGroups := o.groupByHost(definition, placement)

    // 3. Prepare cross-host networking
    networkConfig, err := o.prepareCrossHostNetworking(hostGroups)
    if err != nil {
        return fmt.Errorf("networking setup failed: %w", err)
    }

    // 4. Deploy to each host in dependency order
    deployedContainers := make(map[string]DeployedContainer)

    for hostID, containers := range hostGroups {
        // Get host connection
        host, err := o.storage.GetHost(hostID)
        if err != nil {
            return fmt.Errorf("host %s not found: %w", hostID, err)
        }

        // Connect to host's Docker daemon
        ctx, cli := common.CtxCli(host.DockerSocket)
        defer cli.Close()

        // Deploy containers on this host
        for _, container := range containers {
            // Inject cross-host connection strings
            container.Environment = o.injectConnectionStrings(
                container,
                networkConfig,
                deployedContainers,
            )

            // Deploy using EVE's production deployment
            containerID, err := o.deployContainer(ctx, cli, container, stack)
            if err != nil {
                return fmt.Errorf("deploy %s on %s failed: %w",
                    container.Name, hostID, err)
            }

            deployedContainers[container.Name] = DeployedContainer{
                ID:     containerID,
                HostID: hostID,
                HostIP: host.IPAddress,
                Ports:  container.Ports,
            }

            // Wait for health check
            if err := o.waitForHealth(ctx, cli, containerID); err != nil {
                return fmt.Errorf("health check failed for %s: %w",
                    container.Name, err)
            }
        }
    }

    // 5. Update stack record
    stack.Status = "running"
    stack.HostMapping = o.buildHostMapping(deployedContainers)

    return o.storage.SaveStack(stack)
}

// determinePlacement decides which host each container should run on
func (o *DistributedStackOrchestrator) determinePlacement(
    stack *models.Stack,
    definition *stacks.Stack,
) (map[string]string, error) {
    placement := make(map[string]string)

    switch stack.Placement {
    case models.PlacementManual:
        // Use user-specified targetHost
        for _, item := range definition.ItemListElement {
            if targetHost := item.Labels["targetHost"]; targetHost != "" {
                placement[item.Name] = targetHost
            } else {
                return nil, fmt.Errorf("manual placement but no targetHost for %s",
                    item.Name)
            }
        }

    case models.PlacementAuto:
        // Graphium's intelligent placement
        for _, item := range definition.ItemListElement {
            host, err := o.selectOptimalHost(item, stack.Datacenter)
            if err != nil {
                return nil, err
            }
            placement[item.Name] = host.ID
        }

    case models.PlacementDatacenter:
        // Keep all in same datacenter but distribute across hosts
        hosts, err := o.storage.GetHostsByDatacenter(stack.Datacenter)
        if err != nil {
            return nil, err
        }
        for i, item := range definition.ItemListElement {
            placement[item.Name] = hosts[i%len(hosts)].ID
        }

    case models.PlacementSpread:
        // Spread evenly across all available hosts
        hosts, err := o.storage.ListHosts(map[string]interface{}{
            "status": "active",
        })
        if err != nil {
            return nil, err
        }
        for i, item := range definition.ItemListElement {
            placement[item.Name] = hosts[i%len(hosts)].ID
        }
    }

    return placement, nil
}

// prepareCrossHostNetworking sets up networking for cross-host communication
func (o *DistributedStackOrchestrator) prepareCrossHostNetworking(
    hostGroups map[string][]stacks.StackItemElement,
) (*NetworkConfig, error) {
    // Option 1: Use overlay network (requires Docker Swarm or similar)
    // Option 2: Use exposed ports + host IPs (simpler, works everywhere)

    // We'll use Option 2: exposed ports
    config := &NetworkConfig{
        Mode:      "host-ports",
        Endpoints: make(map[string]Endpoint),
    }

    for hostID, containers := range hostGroups {
        host, _ := o.storage.GetHost(hostID)
        for _, container := range containers {
            for _, port := range container.Ports {
                endpoint := Endpoint{
                    Host:     host.IPAddress,
                    Port:     port.HostPort,
                    Protocol: port.Protocol,
                }
                config.Endpoints[container.Name] = endpoint
            }
        }
    }

    return config, nil
}

// injectConnectionStrings adds cross-host connection info to env vars
func (o *DistributedStackOrchestrator) injectConnectionStrings(
    container stacks.StackItemElement,
    networkConfig *NetworkConfig,
    deployed map[string]DeployedContainer,
) map[string]string {
    env := container.Environment
    if env == nil {
        env = make(map[string]string)
    }

    // Inject connection strings for dependencies
    for _, dep := range container.SoftwareRequirements {
        depContainer, exists := deployed[dep.Name]
        if !exists {
            continue
        }

        // Example: POSTGRES_HOST=192.168.1.10
        envKey := fmt.Sprintf("%s_HOST", strings.ToUpper(dep.Name))
        env[envKey] = depContainer.HostIP

        // Example: POSTGRES_PORT=5432
        if len(depContainer.Ports) > 0 {
            portKey := fmt.Sprintf("%s_PORT", strings.ToUpper(dep.Name))
            env[portKey] = fmt.Sprintf("%d", depContainer.Ports[0].HostPort)
        }

        // Example: POSTGRES_URL=postgres://192.168.1.10:5432/db
        if connectionTemplate := dep.ConnectionString; connectionTemplate != "" {
            urlKey := fmt.Sprintf("%s_URL", strings.ToUpper(dep.Name))
            env[urlKey] = o.renderConnectionString(connectionTemplate, depContainer)
        }
    }

    return env
}

type DeployedContainer struct {
    ID     string
    HostID string
    HostIP string
    Ports  []stacks.PortMapping
}

type NetworkConfig struct {
    Mode      string
    Endpoints map[string]Endpoint
}

type Endpoint struct {
    Host     string
    Port     int
    Protocol string
}
```

---

## Deployment Strategies

### 1. Auto Placement (Recommended)

Graphium intelligently decides host placement based on:

**Scoring Algorithm:**
```go
func (o *DistributedStackOrchestrator) selectOptimalHost(
    container stacks.StackItemElement,
    datacenter string,
) (*models.Host, error) {
    hosts, _ := o.storage.GetHostsByDatacenter(datacenter)

    bestHost := hosts[0]
    bestScore := 0.0

    for _, host := range hosts {
        score := 0.0

        // CPU availability (0-100)
        cpuFree := 100 - host.CPUUsage
        score += cpuFree * 0.4

        // Memory availability (0-100)
        memFree := (host.Memory - host.MemoryUsed) / host.Memory * 100
        score += memFree * 0.4

        // Container count (prefer less loaded)
        containerCount, _ := o.storage.GetContainersByHost(host.ID)
        score += (100 - len(containerCount)) * 0.2

        if score > bestScore {
            bestScore = score
            bestHost = host
        }
    }

    return bestHost, nil
}
```

**Benefits:**
- ✅ Balances load automatically
- ✅ Considers resource availability
- ✅ No manual configuration needed

### 2. Manual Placement

User explicitly specifies which container goes where:

```json
{
  "placement": "manual",
  "itemListElement": [
    {
      "name": "database",
      "targetHost": "db-host-01",
      "reason": "SSD storage"
    },
    {
      "name": "cache",
      "targetHost": "cache-host-01",
      "reason": "High memory"
    },
    {
      "name": "app",
      "targetHost": "app-host-01",
      "reason": "High CPU"
    }
  ]
}
```

**Benefits:**
- ✅ Full control over placement
- ✅ Can match workloads to specialized hardware
- ✅ Predictable deployment

### 3. Datacenter Placement

Keep all containers in same datacenter, distribute across hosts:

**Benefits:**
- ✅ Low latency (same DC)
- ✅ Load distribution
- ✅ High availability

### 4. Spread Placement

Distribute evenly across all available hosts:

**Benefits:**
- ✅ Maximum distribution
- ✅ Fault tolerance
- ✅ Simple strategy

---

## Networking Considerations

### Cross-Host Communication

When containers are on different hosts, they need to communicate. Two approaches:

#### Approach 1: Overlay Networks (Docker Swarm)

```
┌─────────────────────────────────────────┐
│ Docker Overlay Network                   │
│ (containers see each other as same net) │
└─────────────────────────────────────────┘
     │                    │
┌─────────┐          ┌─────────┐
│ Host 1  │          │ Host 2  │
│ app     │ ←→←→←→←→ │ db      │
└─────────┘          └─────────┘
```

**Pros:**
- ✅ Transparent service discovery
- ✅ Containers use service names (e.g., `postgres:5432`)
- ✅ No configuration changes needed

**Cons:**
- ❌ Requires Docker Swarm or Kubernetes
- ❌ More complex networking
- ❌ Additional overhead

#### Approach 2: Exposed Ports + Host IPs (Recommended for Graphium)

```
┌─────────────────────────────────────────┐
│ Container: app                           │
│ ENV:                                     │
│   POSTGRES_HOST=192.168.1.10            │
│   POSTGRES_PORT=5432                     │
└─────────────────────────────────────────┘
                    │
                    ▼
           192.168.1.10:5432
                    │
┌─────────────────────────────────────────┐
│ Host 2: 192.168.1.10                     │
│ Container: postgres                      │
│   Port 5432 → 5432                       │
└─────────────────────────────────────────┘
```

**Pros:**
- ✅ Works with plain Docker (no Swarm needed)
- ✅ Simple and reliable
- ✅ Easy to debug (standard TCP/IP)
- ✅ Graphium already tracks host IPs

**Cons:**
- ❌ Requires updating env vars
- ❌ Can't use service names directly

**Implementation:**

Graphium automatically injects connection info:

```go
// Before deployment on Host 3
env["POSTGRES_HOST"] = "192.168.1.10"  // Host 1's IP
env["POSTGRES_PORT"] = "5432"
env["CACHE_HOST"] = "192.168.1.11"     // Host 2's IP
env["CACHE_PORT"] = "6379"

// Application uses these env vars:
db.Connect(os.Getenv("POSTGRES_HOST"), os.Getenv("POSTGRES_PORT"))
```

---

## UI Integration

### Stack Deployment Form - Host Selection

Add host placement options to deployment form:

```
┌─────────────────────────────────────────────────────────┐
│ Deploy Stack                                            │
├─────────────────────────────────────────────────────────┤
│ DEPLOYMENT STRATEGY                                     │
│                                                         │
│ ● Auto Placement (Recommended)                          │
│   Graphium intelligently distributes containers         │
│                                                         │
│ ○ Manual Placement                                      │
│   Specify host for each container                       │
│                                                         │
│ ○ Same Datacenter                                       │
│   Keep all in same datacenter, distribute load          │
│                                                         │
│ ○ Single Host                                           │
│   Deploy entire stack to one host (EVE default)         │
├─────────────────────────────────────────────────────────┤
│ [Manual Placement Selected]                             │
│                                                         │
│ Container: postgres                                     │
│ Host: [db-host-01 ▼]                                    │
│       CPU: 8 cores (40% free)                           │
│       RAM: 32GB (12GB free)                             │
│                                                         │
│ Container: cache                                        │
│ Host: [cache-host-01 ▼]                                 │
│       CPU: 4 cores (80% free)                           │
│       RAM: 16GB (14GB free)                             │
│                                                         │
│ Container: app                                          │
│ Host: [app-host-01 ▼]                                   │
│       CPU: 16 cores (60% free)                          │
│       RAM: 64GB (40GB free)                             │
└─────────────────────────────────────────────────────────┘
```

### Stack Details - Multi-Host View

Show which containers are on which hosts:

```
┌─────────────────────────────────────────────────────────┐
│ distributed-app                            🟢 running   │
├─────────────────────────────────────────────────────────┤
│ DEPLOYMENT MAP                                          │
│                                                         │
│ ┌─────────────────────────────────────────────────┐   │
│ │ Host 1: db-host-01 (192.168.1.10)               │   │
│ │ ├─ postgres         🟢 healthy                   │   │
│ │ └─ Ports: 5432                                  │   │
│ └─────────────────────────────────────────────────┘   │
│                                                         │
│ ┌─────────────────────────────────────────────────┐   │
│ │ Host 2: cache-host-01 (192.168.1.11)            │   │
│ │ ├─ dragonflydb      🟢 healthy                   │   │
│ │ └─ Ports: 6379                                  │   │
│ └─────────────────────────────────────────────────┘   │
│                                                         │
│ ┌─────────────────────────────────────────────────┐   │
│ │ Host 3: app-host-01 (192.168.1.12)              │   │
│ │ ├─ app              🟢 healthy                   │   │
│ │ ├─ Connects to:                                 │   │
│ │ │  ├─ postgres @ 192.168.1.10:5432              │   │
│ │ │  └─ cache @ 192.168.1.11:6379                 │   │
│ │ └─ Ports: 8080 → 80                             │   │
│ └─────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
```

---

## Migration Path

### Phase 1: Single Host (Current EVE)

**Implementation:** Already done
- Use EVE's `DeployStack()` as-is
- All containers on one host
- Simple, works today

```go
func (o *StackOrchestrator) DeployStack(stack *models.Stack) error {
    ctx, cli := common.CtxCli("unix:///var/run/docker.sock")
    return production.DeployStack(ctx, cli, eveStack)
}
```

### Phase 2: Manual Multi-Host

**Add:** Distributed orchestrator with manual placement
- User specifies targetHost per container
- Graphium deploys to each host
- Cross-host networking via exposed ports

**Estimated Effort:** 2-3 days

### Phase 3: Auto Multi-Host

**Add:** Intelligent placement engine
- Automatic host selection
- Resource-based scoring
- Load balancing

**Estimated Effort:** 3-5 days

### Phase 4: Advanced Features

**Add:**
- Overlay network support
- Service mesh integration
- Auto-scaling across hosts
- Health-based re-placement

**Estimated Effort:** 1-2 weeks

---

## Recommendations

### For Initial Implementation (Phase 1)

**Keep it simple:** Start with single-host deployment (current EVE behavior)

**Why:**
- ✅ Works immediately
- ✅ No networking complexity
- ✅ Sufficient for dev/test environments
- ✅ Foundation for future enhancements

### For Production (Phase 2-3)

**Add distributed deployment** when needed:

**When to add:**
- Load requires distribution
- High availability needed
- Specialized hardware per workload
- Multi-datacenter deployments

**Timeline:** Add when first multi-host use case emerges

---

## Summary

### Current State
- **EVE:** Single-host deployment only
- **Reason:** Connects to one Docker daemon
- **Use case:** Dev environments, single-server prod

### Graphium Extension
- **Can add:** Multi-host distributed deployment
- **How:** Extend EVE with placement engine + cross-host networking
- **Effort:** 2-3 days for manual placement, 3-5 days for auto placement

### Answer to Your Question

**"Can I distribute 3 containers to 3 different hosts?"**

**With EVE alone:** ❌ No
**With Graphium extension:** ✅ **Yes!**

Graphium can:
1. Parse the stack definition
2. Decide which container goes to which host (manual or auto)
3. Deploy each container to its target host via EVE
4. Configure cross-host networking (env vars with IPs/ports)
5. Track the distributed deployment

**Recommended Approach:**
- Phase 1: Start with single-host (use current EVE)
- Phase 2: Add distributed when needed (2-3 days work)

This gives you immediate functionality while keeping the door open for distributed deployment when your use case requires it.
