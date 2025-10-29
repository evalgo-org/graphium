# Distributed Stack Deployment

Graphium now supports distributed stack deployment, allowing you to deploy multi-container applications across multiple Docker hosts. This feature enables better resource utilization, high availability, and scalability.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Placement Strategies](#placement-strategies)
- [Getting Started](#getting-started)
- [Configuration](#configuration)
- [Examples](#examples)
- [API Reference](#api-reference)
- [Limitations](#limitations)

## Overview

Distributed deployment allows you to:

- **Distribute containers across multiple hosts** - Spread your application across 2+ physical or virtual machines
- **Automatic placement** - Let Graphium decide optimal container placement based on resources
- **Manual placement** - Explicitly control which containers run on which hosts
- **Cross-host networking** - Containers on different hosts can communicate via exposed ports
- **Resource-aware scheduling** - Place containers based on CPU, memory, and current load

## Architecture

### Components

1. **DockerClientManager** - Manages Docker client connections to multiple hosts
2. **PlacementStrategy** - Determines optimal container placement
3. **DistributedStackOrchestrator** - Orchestrates the entire deployment process
4. **NetworkConfig** - Manages cross-host networking and service discovery

### Deployment Flow

```
1. Load stack definition (JSON-LD)
2. Determine placement strategy
3. Calculate container placements across hosts
4. Prepare cross-host networking configuration
5. Deploy containers to assigned hosts
6. Wait for health checks
7. Inject cross-host connection environment variables
8. Update deployment status
```

## Placement Strategies

### 1. Auto Placement

Automatically places containers based on available resources.

**Scoring Factors:**
- CPU availability (0-30 points)
- Memory availability (0-30 points)
- Container count (±10 points)
- Datacenter preference (±20 points)
- Host status (active required)

**Example Configuration:**
```json
{
  "deployment": {
    "mode": "multi-host",
    "placementStrategy": "auto"
  }
}
```

**Best For:** General-purpose deployments, when you want Graphium to optimize placement

### 2. Manual Placement

Explicitly specify which host each container runs on.

**Example Configuration:**
```json
{
  "deployment": {
    "mode": "multi-host",
    "placementStrategy": "manual",
    "hostConstraints": [
      {
        "containerName": "web",
        "targetHost": "host1"
      },
      {
        "containerName": "db",
        "targetHost": "host2",
        "minCpu": 4,
        "minMemory": 4294967296
      }
    ]
  }
}
```

**Best For:** Specific placement requirements, compliance, data locality needs

### 3. Spread Placement

Distributes containers evenly across available hosts.

**Algorithm:**
- Places each container on the host with fewest containers
- Only considers active hosts
- Ignores resource availability

**Example Configuration:**
```json
{
  "deployment": {
    "mode": "multi-host",
    "placementStrategy": "spread"
  }
}
```

**Best For:** High availability, load balancing, avoiding single points of failure

### 4. Datacenter Placement

Keeps all containers within a specific datacenter, but spreads them across hosts.

**Example Configuration:**
```json
{
  "location": "us-west-2",
  "deployment": {
    "mode": "multi-host",
    "placementStrategy": "datacenter"
  }
}
```

**Best For:** Data locality, compliance requirements, network latency optimization

## Getting Started

### Prerequisites

1. Multiple Docker hosts with Docker Engine installed
2. Network connectivity between hosts
3. Graphium with access to all Docker sockets

### Step 1: Register Hosts

```go
import "evalgo.org/graphium/internal/orchestration"

// Create orchestrator
orch := orchestration.NewDistributedStackOrchestrator(storage)

// Register hosts
hosts := []*models.Host{
    {
        ID:         "host1",
        Name:       "Web Server 1",
        IPAddress:  "192.168.1.10",
        CPU:        8,
        Memory:     17179869184, // 16GB
        Status:     "active",
        Datacenter: "us-west-2",
    },
    {
        ID:         "host2",
        Name:       "DB Server 1",
        IPAddress:  "192.168.1.11",
        CPU:        16,
        Memory:     34359738368, // 32GB
        Status:     "active",
        Datacenter: "us-west-2",
    },
}

// Register with Docker sockets
orch.RegisterHost(hosts[0], "tcp://192.168.1.10:2375")
orch.RegisterHost(hosts[1], "tcp://192.168.1.11:2375")
```

### Step 2: Create Stack Definition

Create a file `my-app-stack.json`:

```json
{
  "@context": "https://schema.org",
  "@type": "ItemList",
  "name": "my-app",
  "description": "My distributed application",
  "deployment": {
    "mode": "multi-host",
    "placementStrategy": "auto",
    "networkMode": "host-port"
  },
  "itemListElement": [
    {
      "@type": "SoftwareApplication",
      "position": 1,
      "name": "web",
      "image": "nginx:latest",
      "ports": [
        {
          "containerPort": 80,
          "hostPort": 8080
        }
      ],
      "environment": {
        "POSTGRES_HOST": "",
        "POSTGRES_PORT": "5432"
      }
    },
    {
      "@type": "SoftwareApplication",
      "position": 2,
      "name": "api",
      "image": "myapp/api:latest",
      "ports": [
        {
          "containerPort": 3000,
          "hostPort": 3000
        }
      ],
      "environment": {
        "POSTGRES_HOST": "",
        "POSTGRES_PORT": "5432",
        "REDIS_HOST": "",
        "REDIS_PORT": "6379"
      }
    },
    {
      "@type": "SoftwareApplication",
      "position": 3,
      "name": "postgres",
      "image": "postgres:14",
      "ports": [
        {
          "containerPort": 5432,
          "hostPort": 5432
        }
      ],
      "environment": {
        "POSTGRES_PASSWORD": "secret"
      }
    },
    {
      "@type": "SoftwareApplication",
      "position": 4,
      "name": "redis",
      "image": "redis:7",
      "ports": [
        {
          "containerPort": 6379,
          "hostPort": 6379
        }
      ]
    }
  ]
}
```

### Step 3: Deploy Stack

```go
import (
    "context"
    "eve.evalgo.org/containers/stacks"
)

ctx := context.Background()

// Load stack definition
definition, err := stacks.LoadStackFromFile("my-app-stack.json")
if err != nil {
    log.Fatal(err)
}

// Create stack model
stack := &models.Stack{
    Context:        "https://schema.org",
    Type:           "ItemList",
    ID:             "my-app-stack",
    Name:           "my-app",
    Status:         "pending",
    Deployment:     definition.Deployment,
    DefinitionPath: "my-app-stack.json",
    CreatedAt:      time.Now(),
    UpdatedAt:      time.Now(),
}

// Deploy!
deployment, err := orch.DeployStack(ctx, stack, definition, hosts)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Stack deployed successfully!\n")
fmt.Printf("Deployment ID: %s\n", deployment.StackID)
fmt.Printf("Status: %s\n", deployment.Status)

// Print placements
for name, placement := range deployment.Placements {
    fmt.Printf("Container %s: %s (%s:%d)\n",
        name,
        placement.HostID,
        placement.IPAddress,
        placement.Ports[0])
}
```

### Step 4: Verify Deployment

```go
// Get deployment status
deployment, err := orch.storage.GetDeployment("my-app-stack")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Status: %s\n", deployment.Status)
fmt.Printf("Started: %s\n", deployment.StartedAt)

// Check service endpoints
for service, endpoint := range deployment.NetworkConfig.ServiceEndpoints {
    fmt.Printf("%s: %s\n", service, endpoint)
}
```

## Configuration

### Stack Configuration

```json
{
  "@context": "https://schema.org",
  "@type": "ItemList",
  "name": "stack-name",
  "description": "Optional description",
  "location": "datacenter-id",
  "deployment": {
    "mode": "multi-host",
    "placementStrategy": "auto|manual|spread|datacenter",
    "networkMode": "host-port",
    "hostConstraints": [
      {
        "containerName": "container-name",
        "targetHost": "host-id",
        "requiredDatacenter": "dc-id",
        "minCpu": 4,
        "minMemory": 4294967296,
        "labels": {
          "key": "value"
        }
      }
    ]
  }
}
```

### Host Configuration

```go
host := &models.Host{
    ID:         "unique-host-id",
    Name:       "Human-readable name",
    IPAddress:  "192.168.1.10",
    CPU:        8,                // Number of CPU cores
    Memory:     17179869184,      // Total memory in bytes
    Status:     "active",         // active, maintenance, offline
    Datacenter: "datacenter-id",  // Datacenter/region identifier
}
```

## Examples

### Example 1: Simple 3-Tier Application

```json
{
  "@context": "https://schema.org",
  "@type": "ItemList",
  "name": "webapp",
  "deployment": {
    "mode": "multi-host",
    "placementStrategy": "auto"
  },
  "itemListElement": [
    {
      "name": "nginx",
      "image": "nginx:latest",
      "ports": [{"containerPort": 80, "hostPort": 80}]
    },
    {
      "name": "app",
      "image": "myapp:latest",
      "ports": [{"containerPort": 3000, "hostPort": 3000}]
    },
    {
      "name": "postgres",
      "image": "postgres:14",
      "ports": [{"containerPort": 5432, "hostPort": 5432}]
    }
  ]
}
```

### Example 2: High-Availability Setup

```json
{
  "@context": "https://schema.org",
  "@type": "ItemList",
  "name": "ha-webapp",
  "deployment": {
    "mode": "multi-host",
    "placementStrategy": "spread"
  },
  "itemListElement": [
    {
      "name": "web1",
      "image": "nginx:latest",
      "ports": [{"containerPort": 80, "hostPort": 8080}]
    },
    {
      "name": "web2",
      "image": "nginx:latest",
      "ports": [{"containerPort": 80, "hostPort": 8081}]
    },
    {
      "name": "web3",
      "image": "nginx:latest",
      "ports": [{"containerPort": 80, "hostPort": 8082}]
    }
  ]
}
```

### Example 3: Data Locality (Manual Placement)

```json
{
  "@context": "https://schema.org",
  "@type": "ItemList",
  "name": "data-processing",
  "deployment": {
    "mode": "multi-host",
    "placementStrategy": "manual",
    "hostConstraints": [
      {
        "containerName": "processor",
        "targetHost": "compute-node-1",
        "minCpu": 8,
        "minMemory": 17179869184
      },
      {
        "containerName": "database",
        "targetHost": "storage-node-1",
        "minMemory": 34359738368
      }
    ]
  },
  "itemListElement": [
    {
      "name": "processor",
      "image": "myapp/processor:latest"
    },
    {
      "name": "database",
      "image": "postgres:14"
    }
  ]
}
```

## API Reference

### DistributedStackOrchestrator

```go
type DistributedStackOrchestrator struct {
    // Private fields
}

// Create new orchestrator
func NewDistributedStackOrchestrator(storage StackStorage) *DistributedStackOrchestrator

// Register a Docker host
func (o *DistributedStackOrchestrator) RegisterHost(host *models.Host, dockerSocket string) error

// Deploy a stack across multiple hosts
func (o *DistributedStackOrchestrator) DeployStack(
    ctx context.Context,
    stack *models.Stack,
    definition *stacks.Stack,
    hosts []*models.HostInfo,
) (*models.StackDeployment, error)

// Stop a distributed stack
func (o *DistributedStackOrchestrator) StopStack(ctx context.Context, stackID string) error

// Remove a distributed stack
func (o *DistributedStackOrchestrator) RemoveStack(
    ctx context.Context,
    stackID string,
    removeVolumes bool,
) error

// Close all Docker connections
func (o *DistributedStackOrchestrator) Close() error
```

### PlacementStrategy Interface

```go
type PlacementStrategy interface {
    PlaceContainers(
        ctx context.Context,
        stack *models.Stack,
        definition *stacks.Stack,
        hosts []*models.HostInfo,
    ) (map[string]string, error)
}
```

## Networking

### Cross-Host Communication

Graphium uses **host-port networking** by default for cross-host communication:

1. Each container exposes ports on its host machine
2. Graphium injects environment variables with connection strings
3. Containers use these environment variables to connect to services on other hosts

**Example:**
```
Container "web" on host1 needs to connect to "postgres" on host2
Graphium injects:
- POSTGRES_ENDPOINT=192.168.1.11:5432
```

### Environment Variable Injection

For each exposed service, Graphium creates an environment variable:

```
Pattern: {SERVICE_NAME}_ENDPOINT={HOST_IP}:{HOST_PORT}

Examples:
- POSTGRES_ENDPOINT=192.168.1.10:5432
- REDIS_ENDPOINT=192.168.1.11:6379
- API_ENDPOINT=192.168.1.10:3000
```

## Limitations

### Current Limitations

1. **No Docker Overlay Networks** - Currently only supports host-port networking
2. **No Swarm Mode** - Works with vanilla Docker, not Docker Swarm
3. **No Auto-Discovery** - Hosts must be manually registered
4. **No Live Migration** - Cannot move containers between hosts after deployment
5. **No Health Monitoring** - Post-deployment health checks not implemented
6. **Single Deployment Per Stack** - Cannot update running stacks

### Future Enhancements

- Docker overlay network support
- Dynamic host discovery
- Live container migration
- Advanced health monitoring
- Rolling updates
- Auto-scaling based on metrics
- Multi-datacenter support with WAN networking
- Integration with Kubernetes for hybrid deployments

## Troubleshooting

### Connection Issues

**Problem:** Cannot connect to remote Docker daemon

**Solution:**
```bash
# Enable TCP access on remote host
dockerd -H tcp://0.0.0.0:2375 -H unix:///var/run/docker.sock

# Or use SSH tunnel
ssh -L 2375:localhost:2375 user@remote-host
```

### Placement Failures

**Problem:** "No suitable host found for container"

**Solutions:**
- Check host status (must be "active")
- Verify resource requirements
- Check datacenter constraints
- Review host constraints

### Network Issues

**Problem:** Containers cannot communicate across hosts

**Solutions:**
- Verify firewall rules allow traffic on exposed ports
- Check that host IPs are reachable
- Verify environment variables are injected correctly
- Test connectivity: `telnet <host-ip> <port>`

## Performance Considerations

### Scaling Guidelines

- **2-5 hosts**: Any placement strategy works well
- **5-20 hosts**: Use auto or datacenter placement
- **20+ hosts**: Consider external orchestrator (Kubernetes, Nomad)

### Resource Recommendations

- Reserve 10-20% CPU/memory overhead per host
- Use SSDs for database hosts
- Keep latency under 10ms between hosts in same datacenter
- Monitor network bandwidth usage

## Security

### Best Practices

1. **Secure Docker Socket Access**
   - Use TLS for remote Docker connections
   - Or use SSH tunnels for remote access

2. **Network Segmentation**
   - Use firewalls to restrict container-to-container traffic
   - Only expose necessary ports

3. **Secrets Management**
   - Don't store secrets in stack definitions
   - Use environment variables or secret management systems

4. **Access Control**
   - Implement RBAC for stack deployments
   - Audit all deployment activities

## Support

For issues, questions, or feature requests:
- GitHub Issues: https://github.com/yourusername/graphium/issues
- Documentation: https://docs.graphium.dev
- Community: https://community.graphium.dev
