# JSON-LD Stack Deployment Examples

This directory contains example JSON-LD stack definitions for deploying multi-container applications using Graphium's semantic orchestration API.

## Overview

Graphium uses JSON-LD (@graph format) to define container stacks with full semantic metadata. This enables:

- **Semantic container relationships** using Schema.org and custom vocabularies
- **Dependency management** with automatic wave-based deployment
- **Multi-host orchestration** with intelligent placement
- **Network and volume management** with full lifecycle control
- **Resource constraints** and healthcheck definitions

## API Endpoints

### Deploy a Stack
```bash
POST /api/v1/stacks/jsonld
Content-Type: application/json

{
  "stackDefinition": { ... },  # JSON-LD @graph structure
  "timeout": 300,              # Deployment timeout in seconds (optional)
  "rollbackOnError": true,     # Auto-rollback on failure (optional)
  "pullImages": false          # Pull images before deployment (optional)
}
```

### Validate a Stack Definition
```bash
POST /api/v1/stacks/jsonld/validate
Content-Type: application/json

{ ... JSON-LD stack definition ... }
```

### Get Deployment Status
```bash
GET /api/v1/stacks/jsonld/deployments/{deployment-id}
```

### List All Deployments
```bash
GET /api/v1/stacks/jsonld/deployments
```

## Examples

### 1. Simple Web Server (`simple-nginx.json`)

A minimal single-container stack with port mapping:
- 1 nginx container
- Port mapping 80 -> 8080
- Environment variables
- Restart policy

**Deploy:**
```bash
curl -X POST http://localhost:8095/api/v1/stacks/jsonld \
  -H "Content-Type: application/json" \
  -d @simple-nginx.json
```

### 2. Three-Tier Web Application (`3-tier-webapp.json`)

Classic 3-tier architecture with dependencies:
- PostgreSQL database with persistent volume
- Node.js API backend (depends on database)
- Nginx frontend (depends on API)
- Custom bridge network with subnet
- Healthchecks and resource limits

**Features demonstrated:**
- Container dependencies (`dependsOn`)
- Named volumes
- Custom network with IPAM configuration
- Health checks
- Resource constraints (CPU/memory)
- Environment variable configuration

**Deploy:**
```bash
curl -X POST http://localhost:8095/api/v1/stacks/jsonld \
  -H "Content-Type: application/json" \
  -d '{
    "stackDefinition": '$(cat 3-tier-webapp.json)',
    "timeout": 600,
    "rollbackOnError": true,
    "pullImages": false
  }'
```

### 3. Microservices Multi-Host (`microservices-multi-host.json`)

Advanced microservices deployment across multiple hosts:
- Redis cache/message broker
- Authentication service (depends on Redis)
- User service (depends on Redis + Auth, deployed to host2)
- API Gateway (depends on all services)
- Custom network for service discovery
- Multi-host placement with `locatedInHost`

**Features demonstrated:**
- Multi-host deployment
- Service dependencies
- Multiple port mappings
- Environment variable interpolation
- Healthcheck with custom commands
- Complex dependency graph (4 deployment waves)

**Deploy:**
```bash
curl -X POST http://localhost:8095/api/v1/stacks/jsonld \
  -H "Content-Type: application/json" \
  -d @microservices-multi-host.json
```

## JSON-LD Stack Structure

### Stack Node (Required)
```json
{
  "@id": "https://graphium.local/stacks/my-stack",
  "@type": ["datacenter:Stack", "SoftwareApplication"],
  "name": "my-stack",
  "description": "Stack description",
  "locatedInHost": {
    "@id": "host-id"  // Optional: default host for all containers
  },
  "network": {        // Optional: custom network
    "name": "my-network",
    "driver": "bridge",
    "subnet": "172.20.0.0/16",
    "gateway": "172.20.0.1",
    "labels": {}
  },
  "hasPart": [
    // Container specs...
  ]
}
```

### Container Spec
```json
{
  "@id": "https://graphium.local/containers/my-container",
  "@type": "datacenter:Container",
  "name": "container-name",
  "image": "nginx:latest",
  "description": "Container description",
  "locatedInHost": {
    "@id": "host-id"  // Optional: override stack default
  },
  "dependsOn": ["other-container"],  // Optional: dependencies
  "command": ["nginx", "-g", "daemon off;"],  // Optional
  "args": [],         // Optional: additional arguments
  "workingDir": "/app",  // Optional
  "user": "nginx",    // Optional
  "ports": [
    {
      "containerPort": 80,
      "hostPort": 8080,
      "protocol": "tcp"
    }
  ],
  "environment": {
    "KEY": "value"
  },
  "volumes": [
    {
      "name": "data-volume",
      "mountPath": "/data",
      "driver": "local",
      "driverOpts": {},
      "labels": {}
    }
  ],
  "labels": {
    "app": "my-app"
  },
  "restartPolicy": "unless-stopped",  // no, on-failure, always, unless-stopped
  "resources": {
    "cpu": "0.5",     // CPU cores
    "memory": "512M"  // Memory limit
  },
  "healthcheck": {
    "test": ["CMD", "curl", "-f", "http://localhost/health"],
    "interval": "30s",
    "timeout": "10s",
    "retries": 3,
    "startPeriod": "40s"
  }
}
```

## Deployment Workflow

1. **Parse & Validate**: Stack definition is parsed and validated
2. **Build Dependency Graph**: Container dependencies create deployment waves
3. **Network Creation**: Custom network is created if specified
4. **Volume Creation**: Named volumes are created
5. **Wave Deployment**: Containers are deployed in dependency order
   - Wave 1: Containers with no dependencies
   - Wave 2: Containers depending on Wave 1
   - Wave 3+: Subsequent dependencies
6. **Health Monitoring**: Containers are monitored for successful startup
7. **State Tracking**: Deployment state is persisted for monitoring

## Deployment State Response

```json
{
  "id": "deployment-abc123",
  "stackId": "stack-xyz789",
  "status": "running",
  "phase": "container-deployment",
  "progress": 75,
  "placements": {
    "container1": {
      "containerID": "docker-id",
      "hostID": "host1",
      "status": "running"
    }
  },
  "networkInfo": {
    "networkID": "net-id",
    "name": "my-network",
    "driver": "bridge",
    "subnet": "172.20.0.0/16"
  },
  "volumeInfo": {
    "volume1": {
      "name": "data-volume",
      "driver": "local",
      "mountpoint": "/var/lib/docker/volumes/..."
    }
  },
  "events": [
    {
      "timestamp": "2025-10-30T10:00:00Z",
      "phase": "initialization",
      "message": "Starting deployment",
      "level": "info"
    }
  ],
  "startedAt": "2025-10-30T10:00:00Z",
  "completedAt": "2025-10-30T10:05:00Z"
}
```

## Validation

Before deploying, validate your stack definition:

```bash
curl -X POST http://localhost:8095/api/v1/stacks/jsonld/validate \
  -H "Content-Type: application/json" \
  -d @my-stack.json
```

Response:
```json
{
  "valid": true,
  "warnings": [],
  "errors": [],
  "stackName": "my-stack",
  "containerCount": 3,
  "hasNetwork": true,
  "waveCount": 2
}
```

## Host Requirements

### Host Registration
Hosts must be registered with Graphium before deployment. Two options:

1. **Agent-based** (recommended):
```bash
./graphium agent --api-url http://graphium:8095
```

2. **Manual registration** via API:
```bash
POST /api/v1/hosts
{
  "name": "my-host",
  "ipAddress": "192.168.1.100",
  "cpu": 4.0,
  "memory": 8192,
  "datacenter": "dc1"
}
```

### Docker Access
- **Local hosts**: Unix socket at `/var/run/docker.sock`
- **Remote hosts**: TCP port 2375 must be accessible
- Ensure Docker API version compatibility (v28.x)

## Error Handling

### Validation Errors
- Missing required fields (name, image)
- Invalid port numbers or protocols
- Circular dependencies
- Non-existent dependency references

### Deployment Errors
- Host not found or unavailable
- Docker client connection failure
- Image pull failures
- Container creation/start failures
- Network or volume creation failures

### Rollback
If `rollbackOnError: true`, failures trigger automatic cleanup:
1. Stop all running containers
2. Remove created containers
3. Remove created networks
4. Remove created volumes (if no other references)

## Monitoring

Poll deployment status:
```bash
# Get deployment ID from deploy response
DEPLOYMENT_ID="deployment-abc123"

# Poll status every 5 seconds
watch -n 5 "curl -s http://localhost:8095/api/v1/stacks/jsonld/deployments/$DEPLOYMENT_ID | jq '.status, .progress, .phase'"
```

## Best Practices

1. **Use semantic IDs**: Absolute URLs for @id fields
2. **Define dependencies**: Use `dependsOn` for correct ordering
3. **Resource limits**: Set CPU/memory constraints
4. **Health checks**: Add healthcheck for critical services
5. **Networks**: Use custom networks for service isolation
6. **Labels**: Tag containers for filtering and organization
7. **Restart policies**: Use `unless-stopped` for production services
8. **Validation**: Always validate before deploying
9. **Monitoring**: Track deployment status via API
10. **Host affinity**: Use `locatedInHost` for specific placement

## Troubleshooting

### Stack validation fails
- Check JSON syntax
- Verify all required fields are present
- Ensure container names are unique within stack
- Check dependency references match container names

### Deployment hangs
- Check host connectivity
- Verify Docker is running on target hosts
- Check image availability
- Review deployment events for specific errors

### Containers fail to start
- Check container logs via API or Docker
- Verify environment variables and volumes
- Check port conflicts
- Review healthcheck configuration

## Next Steps

- [ ] Web UI for JSON-LD editor with syntax highlighting
- [ ] Real-time deployment progress visualization
- [ ] JSON Schema validation
- [ ] Template library management
- [ ] Stack versioning and rollback
- [ ] Resource usage visualization
- [ ] Advanced scheduling policies
