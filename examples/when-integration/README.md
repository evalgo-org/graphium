# When → Graphium Semantic Integration

This directory contains example workflows for orchestrating container deployments using **When** (semantic task orchestrator) and **Graphium** (semantic container orchestration platform).

## Architecture

```
When Orchestrator (Scheduler)
    ↓ (HTTP POST with JSON-LD)
Graphium (/api/v1/stacks/jsonld)
    ↓ (Parse Schema.org @graph stacks)
Docker/Podman (Container Runtime)
```

## Key Features

✅ **Semantic-First**: Full Schema.org JSON-LD compliance end-to-end
✅ **Schedulable**: Run deployments on any schedule (hourly, daily, weekly)
✅ **Parallel Execution**: Deploy multiple stacks concurrently
✅ **Dependency Management**: Chain tasks with `dependsOn`
✅ **Multi-Host**: Spread containers across multiple hosts
✅ **Machine-Readable**: AI agents can understand and generate workflows
✅ **Introspectable**: Query deployments by semantic type

## Integration with pxgraphservice

Combine GraphDB migrations with container deployments for complete infrastructure orchestration:

```
When → pxgraphservice (GraphDB migration) → Graphium (Container deployment)
```

See `02-graphdb-then-containers.json` for a complete example.

## Workflow Files

### 01-scheduled-nginx-deployment.json
**Purpose**: Hourly deployment/update of Nginx load balancer stack

**Action Type**: `ScheduledAction` wrapping Graphium JSON-LD stack

**Schedule**: Every hour

**Usage with When**:
```bash
# Submit to When orchestrator
curl -X POST http://localhost:3000/api/workflows/create \
  -H "Content-Type: application/json" \
  -d @01-scheduled-nginx-deployment.json
```

**What it does**:
- Deploys 2 Nginx containers across available hosts
- Uses "spread" placement strategy for high availability
- Maps ports 8080 and 8081 to container port 80
- Refreshes deployment every hour to ensure consistency

### 02-graphdb-then-containers.json
**Purpose**: Sequential workflow - GraphDB migration followed by application deployment

**Action Type**: `ItemList` with sequential execution (parallel: false)

**Dependencies**: Application deployment depends on GraphDB migration

**Usage with When**:
```bash
curl -X POST http://localhost:3000/api/workflows/create \
  -H "Content-Type: application/json" \
  -d @02-graphdb-then-containers.json
```

**What it does**:
1. **Step 1**: Migrate production user graph to staging GraphDB
   - Calls pxgraphservice semantic API
   - Transfers specific named graph (http://example.org/graph/users)
2. **Step 2**: Deploy application containers (runs AFTER step 1 completes)
   - Calls Graphium semantic API
   - Deploys GraphDB client app + Nginx proxy
   - Application connects to staging GraphDB with migrated data

**Schedule**: Daily at 3:00 AM

### 03-parallel-infrastructure.json
**Purpose**: Deploy complete infrastructure stack in parallel

**Action Type**: `ItemList` with parallel execution (parallel: true, concurrency: 3)

**Components**:
- GraphDB cluster (2 nodes with persistent storage)
- Redis cache (single instance with persistent storage)
- Application services (2 instances with load balancing)

**Usage with When**:
```bash
curl -X POST http://localhost:3000/api/workflows/create \
  -H "Content-Type: application/json" \
  -d @03-parallel-infrastructure.json
```

**What it does**:
- Deploys 3 stacks in parallel (GraphDB, Redis, App)
- GraphDB nodes use "spread" strategy for HA
- Application services connect to both GraphDB nodes
- All services use persistent volumes

**Schedule**: Weekly on Sunday at 1:00 AM

## Environment Variables

Before running these workflows, set up your credentials:

```bash
# Graphium API Key
export GRAPHIUM_API_KEY="your-graphium-api-key"

# GraphDB API Key (for combined workflows)
export GRAPHDB_API_KEY="your-pxgraphservice-api-key"

# GraphDB credentials
export PROD_GRAPHDB_PASSWORD="prod-password"
export STAGING_GRAPHDB_PASSWORD="staging-password"
```

## Schema.org Types Used

### ScheduledAction (When)
Represents a scheduled task to be executed by When:
- **additionalProperty.url**: Target API endpoint
- **additionalProperty.httpMethod**: HTTP method (POST)
- **additionalProperty.headers**: API keys, content type
- **additionalProperty.body**: JSON-LD payload
- **schedule**: When/how often to run

### Stack (Graphium - datacenter:Stack)
Represents a multi-container application:
- **@type**: ["datacenter:Stack", "SoftwareApplication"]
- **deployment**: Deployment configuration (mode, strategy)
- **hasPart**: List of containers in the stack

### SoftwareApplication (Container)
Represents a containerized application:
- **name**: Container name
- **image**: Docker/Podman image
- **containerPort**: Port mappings (PropertyValue)
- **environment**: Environment variables (PropertyValue)
- **storageRequirement**: Volume mounts (PropertyValue)
- **dependsOn**: Container dependencies

### ItemList
Represents a workflow with multiple actions:
- **parallel**: true/false for parallel/sequential execution
- **concurrency**: Max parallel tasks
- **itemListElement**: List of ScheduledActions

### TransferAction (pxgraphservice)
Represents GraphDB data migration:
- **fromLocation**: Source repository (SoftwareSourceCode)
- **toLocation**: Target repository (SoftwareSourceCode)
- **object**: Specific graph to transfer (Dataset)

## Direct Testing (Without When)

You can test Graphium semantic API directly:

```bash
# Deploy a simple stack
curl -X POST http://localhost:8095/api/v1/stacks/jsonld \
  -H "x-api-key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "@context": ["https://schema.org", {"datacenter": "https://graphium.io/vocab/datacenter#"}],
    "@graph": [{
      "@id": "stack:test-nginx",
      "@type": ["datacenter:Stack", "SoftwareApplication"],
      "name": "Test Nginx",
      "deployment": {
        "mode": "single-host",
        "placementStrategy": "auto"
      },
      "hasPart": [{
        "@id": "container:nginx-test",
        "@type": "SoftwareApplication",
        "name": "nginx-test",
        "image": "nginx:alpine",
        "containerPort": [{
          "@type": "PropertyValue",
          "name": "http",
          "value": "80",
          "description": "8888:80"
        }]
      }]
    }]
  }'
```

## Creating Custom Workflows

### Step 1: Choose Workflow Type

**Single Stack Deployment**:
```json
{
  "@type": "ScheduledAction",
  "additionalProperty": {
    "url": "http://localhost:8095/api/v1/stacks/jsonld",
    "body": { /* Stack definition */ }
  }
}
```

**Sequential Workflow**:
```json
{
  "@type": "ItemList",
  "parallel": false,
  "itemListElement": [ /* Ordered tasks */ ]
}
```

**Parallel Workflow**:
```json
{
  "@type": "ItemList",
  "parallel": true,
  "concurrency": 3,
  "itemListElement": [ /* Independent tasks */ ]
}
```

### Step 2: Define Stack

```json
{
  "@context": ["https://schema.org", {"datacenter": "https://graphium.io/vocab/datacenter#"}],
  "@graph": [{
    "@id": "stack:my-app",
    "@type": ["datacenter:Stack", "SoftwareApplication"],
    "name": "My Application",
    "deployment": {
      "mode": "multi-host",
      "placementStrategy": "spread"
    },
    "hasPart": [
      { /* Container definitions */ }
    ]
  }]
}
```

### Step 3: Define Containers

```json
{
  "@id": "container:my-service",
  "@type": "SoftwareApplication",
  "name": "my-service",
  "image": "myapp/service:latest",
  "containerPort": [{
    "@type": "PropertyValue",
    "name": "http",
    "value": "8080",
    "description": "9000:8080"
  }],
  "environment": [{
    "@type": "PropertyValue",
    "name": "DATABASE_URL",
    "value": "postgres://db:5432"
  }],
  "storageRequirement": [{
    "@type": "PropertyValue",
    "name": "data",
    "value": "/app/data",
    "description": "my-volume:/app/data"
  }],
  "dependsOn": ["container:database"]
}
```

### Step 4: Add Schedule

```json
{
  "schedule": {
    "@type": "Schedule",
    "repeatFrequency": "P1D",
    "startTime": "02:00:00"
  }
}
```

## Deployment Strategies

### Auto Placement
```json
{
  "deployment": {
    "mode": "single-host",
    "placementStrategy": "auto"
  }
}
```
Graphium automatically selects the best host.

### Manual Placement
```json
{
  "@id": "container:web-1",
  "locatedInHost": {
    "@id": "host-id-from-graphium"
  }
}
```
Explicitly assign container to specific host.

### Spread Strategy
```json
{
  "deployment": {
    "mode": "multi-host",
    "placementStrategy": "spread",
    "maxInstancesPerHost": 2
  }
}
```
Distribute containers across available hosts for HA.

## Monitoring

View deployment status and logs:

**Graphium UI**: http://localhost:8095/
**When UI**: http://localhost:3000/
**Graphium API**: http://localhost:8095/api/v1/stacks

## Advanced Examples

### Blue-Green Deployment
```json
{
  "@type": "ItemList",
  "parallel": false,
  "itemListElement": [
    {
      "item": {
        "identifier": "deploy-green",
        "description": "Deploy new version (green)"
      }
    },
    {
      "item": {
        "identifier": "health-check-green",
        "description": "Verify green deployment",
        "dependsOn": ["deploy-green"]
      }
    },
    {
      "item": {
        "identifier": "deactivate-blue",
        "description": "Remove old version (blue)",
        "dependsOn": ["health-check-green"]
      }
    }
  ]
}
```

### Canary Deployment
```json
{
  "hasPart": [
    {
      "@id": "container:app-v1",
      "name": "app-v1",
      "image": "myapp:v1.0",
      "environment": [{
        "name": "WEIGHT",
        "value": "90"
      }]
    },
    {
      "@id": "container:app-v2",
      "name": "app-v2-canary",
      "image": "myapp:v2.0",
      "environment": [{
        "name": "WEIGHT",
        "value": "10"
      }]
    }
  ]
}
```

### Database Migration + App Deployment
See `02-graphdb-then-containers.json` for complete example combining:
- pxgraphservice (GraphDB migration)
- Graphium (container deployment)
- Dependencies (app waits for DB migration)

## Troubleshooting

**Issue**: Workflow not executing
- Check When UI for task status
- Verify API keys in environment variables
- Check Graphium logs: `docker logs graphium`

**Issue**: Stack deployment fails
- Check Graphium UI for error messages
- Verify image names are correct
- Check host connectivity
- Review container dependencies

**Issue**: Container fails to start
- Check Graphium container logs
- Verify port conflicts
- Check volume mount permissions
- Review environment variables

**Issue**: Schedule not triggering
- Verify ISO 8601 duration format
- Check When daemon is running
- Review When task logs

## References

- [Schema.org Actions](https://schema.org/Action)
- [Schema.org SoftwareApplication](https://schema.org/SoftwareApplication)
- [ISO 8601 Durations](https://en.wikipedia.org/wiki/ISO_8601#Durations)
- [When Documentation](http://localhost:3000/examples)
- [Graphium API](http://localhost:8095/docs)
- [Graphium Stack Examples](../nginx-multihost-stack.json)

## Complete Semantic Orchestration Stack

```
┌─────────────────────────────────────────────┐
│ When (Semantic Task Orchestrator)          │
│ - Schedule workflows                        │
│ - Manage dependencies                       │
│ - Parallel/sequential execution             │
└──────────────┬──────────────────────────────┘
               │
               ├──────────────────────────────────┐
               ▼                                  ▼
┌──────────────────────────┐   ┌──────────────────────────┐
│ pxgraphservice           │   │ Graphium                 │
│ (GraphDB Migrations)     │   │ (Container Orchestration)│
│                          │   │                          │
│ POST /v1/api/semantic/   │   │ POST /api/v1/stacks/     │
│      action              │   │      jsonld              │
│                          │   │                          │
│ - TransferAction         │   │ - Stack deployment       │
│ - CreateAction           │   │ - Multi-host support     │
│ - DeleteAction           │   │ - Auto-discovery         │
│ - UpdateAction           │   │ - Health monitoring      │
│ - UploadAction           │   │                          │
└──────────────┬───────────┘   └──────────────┬───────────┘
               ▼                              ▼
    ┌──────────────────┐          ┌──────────────────────┐
    │ GraphDB          │          │ Docker/Podman        │
    │ (RDF Triple Store│          │ (Container Runtime)  │
    └──────────────────┘          └──────────────────────┘
```

All components use **Schema.org JSON-LD** for semantic interoperability.
