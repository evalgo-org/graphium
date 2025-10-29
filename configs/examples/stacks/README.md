# Example Stack Definitions

This directory contains example stack definitions for distributed deployment with Graphium.

## Available Examples

### 1. 3-Tier Web Application (`3-tier-webapp.json`)

A classic 3-tier architecture with:
- **nginx**: Web server / reverse proxy
- **api**: Node.js API server
- **postgres**: PostgreSQL database

**Deployment:**
```bash
# Deploy with automatic placement
graphium stack deploy configs/examples/stacks/3-tier-webapp.json --strategy auto

# Deploy to specific datacenter
graphium stack deploy configs/examples/stacks/3-tier-webapp.json --strategy datacenter --datacenter us-west-2
```

**Features:**
- Health checks on all services
- Persistent volume for database
- Automatic service discovery via environment variables
- Dependency management (API waits for database)

### 2. Microservices Architecture (`microservices.json`)

A microservices setup with:
- **api-gateway**: Kong API Gateway
- **user-service**: User management service
- **product-service**: Product catalog service
- **order-service**: Order processing service
- **postgres**: Shared database
- **redis**: Shared cache

**Deployment:**
```bash
# Deploy with spread strategy for high availability
graphium stack deploy configs/examples/stacks/microservices.json --strategy spread

# Deploy manually to specific hosts
graphium stack deploy configs/examples/stacks/microservices.json --strategy manual --hosts host1,host2,host3
```

**Features:**
- Multiple independent services
- Shared data layer (PostgreSQL + Redis)
- API Gateway for unified entry point
- Service-to-service communication via environment variables

### 3. High-Availability Setup (`high-availability.json`)

A high-availability configuration with:
- **haproxy**: Load balancer
- **web1, web2, web3**: Three nginx web server replicas

**Deployment:**
```bash
# Deploy with spread strategy to distribute across hosts
graphium stack deploy configs/examples/stacks/high-availability.json --strategy spread
```

**Features:**
- Load balancing with HAProxy
- Multiple service replicas
- Health monitoring
- Automatic failover capability

## Stack Definition Format

All examples follow the schema.org ItemList format with EVE container definitions. Key fields:

```json
{
  "@context": "https://schema.org",
  "@type": "ItemList",
  "name": "stack-name",
  "description": "Description of the stack",
  "network": {
    "name": "network-name",
    "driver": "bridge",
    "createIfNotExists": true
  },
  "itemListElement": [
    {
      "@type": "SoftwareApplication",
      "position": 1,
      "name": "container-name",
      "image": "image:tag",
      "ports": [...],
      "environment": {...},
      "volumeMounts": [...],
      "healthCheck": {...}
    }
  ]
}
```

## Deployment Strategies

### Auto
Graphium automatically places containers based on available resources (CPU, memory, current load).

```bash
graphium stack deploy stack.json --strategy auto
```

### Manual
Explicitly specify which host each container runs on (requires host constraints in the stack definition or via --hosts flag).

```bash
graphium stack deploy stack.json --strategy manual --hosts host1,host2,host3
```

### Spread
Distributes containers evenly across available hosts for maximum distribution.

```bash
graphium stack deploy stack.json --strategy spread
```

### Datacenter
Keeps all containers within a specific datacenter but spreads them across hosts.

```bash
graphium stack deploy stack.json --strategy datacenter --datacenter us-west-2
```

## Environment Variable Injection

Graphium automatically injects environment variables for cross-host communication:

- `{SERVICE}_ENDPOINT`: Full connection endpoint (host:port)
- Format: `POSTGRES_ENDPOINT=192.168.1.10:5432`

These replace empty strings in your environment configuration:

```json
"environment": {
  "DATABASE_HOST": "",  // Will be injected as: 192.168.1.10
  "DATABASE_PORT": "5432"
}
```

## Health Checks

All services should define health checks for reliable deployments:

**HTTP Health Check:**
```json
"healthCheck": {
  "type": "http",
  "path": "/health",
  "port": 3000,
  "intervalSeconds": 10,
  "timeoutSeconds": 5,
  "retries": 3
}
```

**Command Health Check:**
```json
"healthCheck": {
  "type": "command",
  "command": ["pg_isready", "-U", "postgres"],
  "intervalSeconds": 10,
  "timeoutSeconds": 5,
  "retries": 5
}
```

## Volume Management

### Named Volumes
```json
"volumeMounts": [
  {
    "source": "postgres-data",
    "target": "/var/lib/postgresql/data",
    "type": "volume"
  }
]
```

### Bind Mounts
```json
"volumeMounts": [
  {
    "source": "./config.yml",
    "target": "/etc/app/config.yml",
    "type": "bind",
    "readOnly": true
  }
]
```

## Common Patterns

### Database + Application
```json
{
  "itemListElement": [
    {
      "name": "app",
      "dependsOn": ["db"],
      "environment": {
        "DATABASE_HOST": "",
        "DATABASE_PORT": "5432"
      }
    },
    {
      "name": "db",
      "image": "postgres:14",
      "ports": [{"containerPort": 5432, "hostPort": 5432}]
    }
  ]
}
```

### Web + API + Database
Classic 3-tier with proper dependencies and health checks (see `3-tier-webapp.json`).

### Microservices with Shared Data Layer
Multiple services sharing PostgreSQL and Redis (see `microservices.json`).

### High Availability with Load Balancing
Load balancer + multiple service replicas (see `high-availability.json`).

## Tips

1. **Always define health checks** - Ensures services are ready before dependent services start
2. **Use persistent volumes** - For databases and stateful applications
3. **Set resource limits** - Prevent resource exhaustion (add to container spec)
4. **Use secrets management** - Don't hardcode sensitive data in stack definitions
5. **Test locally first** - Deploy to a single host before distributed deployment
6. **Monitor deployments** - Use `graphium stack status <id>` to check progress

## Next Steps

1. Customize these examples for your use case
2. Add your own stack definitions
3. Create deployment scripts for different environments
4. Integrate with CI/CD pipelines

For more information, see: `docs/DISTRIBUTED_DEPLOYMENT.md`
