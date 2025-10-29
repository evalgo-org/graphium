# EVE Library v0.0.12 - New Features for Graphium

This document outlines the new features available in EVE v0.0.12 that Graphium can now use.

## Update Summary

**Previous Version:** v0.0.8
**Current Version:** v0.0.12
**Local Path:** `/home/opunix/eve` (via replace directive)

---

## 1. Container Orchestration Support ⭐ **NEW**

EVE now includes comprehensive container management for **16 container types** plus **multi-container stack orchestration**.

### Available Containers

#### Testing Package (`eve.evalgo.org/containers/testing`)
Ephemeral containers for integration tests using testcontainers-go:

**Databases:**
- `SetupBaseX()` - XML database
- `SetupCouchDB()` - Document store
- `SetupPostgreSQL()` - Relational database
- `SetupDragonflyDB()` - Redis-compatible cache

**Search & Analytics:**
- `SetupOpenSearch()` - Search engine
- `SetupOpenSearchDashboards()` - Visualization UI

**Semantic & Graph:**
- `SetupGraphDB()` - RDF triple store
- `SetupRDF4J()` - RDF framework

**Messaging:**
- `SetupRabbitMQ()` - AMQP message broker

**Infrastructure:**
- `SetupRegistry()` - Docker image registry
- `SetupLakeFS()` - Data lake versioning

**Observability:**
- `SetupGrafana()` - Dashboards
- `SetupMimir()` - Metrics storage
- `SetupFluentBit()` - Log processing
- `SetupOTelCollector()` - Telemetry collection
- `SetupDockerStatsExporter()` - Container metrics

#### Production Package (`eve.evalgo.org/containers/production`)
Production-ready container deployment with Docker API:

Each container has:
- `Deploy{Container}()` - Deploy with persistent storage
- `Stop{Container}()` - Graceful shutdown
- `Remove{Container}()` - Cleanup with optional volume removal
- `Get{Container}URL()` - Helper functions for connection URLs

**Example:**
```go
import "eve.evalgo.org/containers/production"

ctx, cli := common.CtxCli("unix:///var/run/docker.sock")
defer cli.Close()

// Deploy PostgreSQL
config := production.DefaultPostgreSQLProductionConfig()
config.Password = "secure-password"
containerID, err := production.DeployPostgreSQL(ctx, cli, config)

// Get connection URL
pgURL := production.GetPostgreSQLURL(config)
// postgres://postgres:secure-password@localhost:5432/postgres
```

---

## 2. Container Stacks (schema.org ItemList) ⭐ **NEW**

Multi-container orchestration with dependency management using schema.org standards.

### Package: `eve.evalgo.org/containers/stacks`

**Key Features:**
- Declarative stack definitions (JSON-LD or Go structs)
- Dependency resolution and ordering
- Health check waiting
- Post-start actions (migrations, initialization)
- Circular dependency detection
- Schema.org compliant structure

### Stack Structure

```go
import "eve.evalgo.org/containers/stacks"

// Load stack from JSON-LD
stack, err := stacks.LoadStackFromFile("definitions/my-stack.json")

// Or create programmatically
stack := stacks.Stack{
    Context: "https://schema.org",
    Type:    "ItemList",
    Name:    "My Application Stack",
    Network: stacks.NetworkConfig{
        Name:   "app-network",
        Driver: "bridge",
    },
    ItemListElement: []stacks.StackItemElement{
        {
            Type:     "SoftwareApplication",
            Position: 1,
            Name:     "database",
            Image:    "postgres:17",
            Environment: map[string]string{
                "POSTGRES_PASSWORD": "secret",
            },
            HealthCheck: stacks.HealthCheckConfig{
                Type:    "command",
                Command: []string{"pg_isready"},
            },
        },
        {
            Type:     "SoftwareApplication",
            Position: 2,
            Name:     "app",
            Image:    "myapp:latest",
            SoftwareRequirements: []stacks.SoftwareRequirement{
                {
                    Name:           "database",
                    WaitForHealthy: true,
                },
            },
            PotentialAction: []stacks.Action{
                {
                    Name:       "Run migrations",
                    ActionType: "migration",
                    Command:    []string{"npm", "run", "migrate"},
                },
            },
        },
    },
}
```

### Testing Support

```go
import (
    "eve.evalgo.org/containers/stacks/testing"
)

func TestWithStack(t *testing.T) {
    ctx := context.Background()
    stack, _ := stacks.LoadStackFromFile("my-stack.json")

    deployment, cleanup, err := testing.SetupStack(ctx, t, stack)
    require.NoError(t, err)
    defer cleanup()

    // Access containers via deployment.Ports
    appURL := fmt.Sprintf("http://localhost:%s", deployment.Ports["app"])
}
```

### Production Deployment

```go
import (
    "eve.evalgo.org/containers/stacks/production"
)

func main() {
    ctx, cli := common.CtxCli("unix:///var/run/docker.sock")
    defer cli.Close()

    stack, _ := stacks.LoadStackFromFile("my-stack.json")

    // Deploy entire stack
    deployment, err := production.DeployStack(ctx, cli, stack)

    // Later: graceful shutdown
    production.StopStack(ctx, cli, "My Application Stack")

    // Cleanup
    production.RemoveStack(ctx, cli, "My Application Stack", false)
}
```

### Example Stack: Infisical

A ready-to-use stack definition is available at:
`eve.evalgo.org/containers/stacks/definitions/infisical.json`

This demonstrates a real-world multi-container setup:
- PostgreSQL (database)
- DragonflyDB (cache)
- Infisical (secrets management)
- With dependencies and migrations

---

## 3. How Graphium Can Use These Features

### Use Case 1: Integration Testing with Real Dependencies

Instead of mocking databases, use real containerized instances:

```go
func TestGraphStorageWithRealDB(t *testing.T) {
    ctx := context.Background()

    // Start CouchDB container
    couchURL, cleanup, err := testing.SetupCouchDB(ctx, t, nil)
    require.NoError(t, err)
    defer cleanup()

    // Use real database for tests
    db := NewStorageWithURL(couchURL)
    // ... run tests with real CouchDB
}
```

### Use Case 2: Local Development Stack

Create a `graphium-dev.json` stack with all dependencies:

```json
{
  "@context": "https://schema.org",
  "@type": "ItemList",
  "name": "Graphium Development Stack",
  "network": {
    "name": "graphium-dev",
    "driver": "bridge"
  },
  "itemListElement": [
    {
      "@type": "SoftwareApplication",
      "position": 1,
      "name": "couchdb",
      "image": "couchdb:4.3.0",
      "environment": {
        "COUCHDB_USER": "admin",
        "COUCHDB_PASSWORD": "password"
      },
      "ports": [{"containerPort": 5984, "hostPort": 5984}]
    },
    {
      "@type": "SoftwareApplication",
      "position": 2,
      "name": "dragonflydb",
      "image": "docker.dragonflydb.io/dragonflydb/dragonfly:v1.26.1",
      "ports": [{"containerPort": 6379, "hostPort": 6379}]
    },
    {
      "@type": "SoftwareApplication",
      "position": 3,
      "name": "graphdb",
      "image": "ontotext/graphdb:10.8.1",
      "ports": [{"containerPort": 7200, "hostPort": 7200}]
    }
  ]
}
```

Then deploy with one command:
```bash
# In code or via CLI tool
production.DeployStack(ctx, cli, stack)
```

### Use Case 3: CI/CD Pipeline Testing

Use stacks in GitHub Actions / CI:

```go
func TestE2E(t *testing.T) {
    // Entire environment spins up automatically
    stack, _ := stacks.LoadStackFromFile("ci-stack.json")
    deployment, cleanup, _ := testing.SetupStack(ctx, t, stack)
    defer cleanup()

    // Run end-to-end tests against real services
}
```

---

## 4. Benefits for Graphium

### Immediate Benefits

1. **Remove Test Mocks**: Use real containerized services in tests
2. **Faster Development**: One-command stack deployment
3. **Better Testing**: Integration tests with real dependencies
4. **CI/CD Ready**: Automatic container management in pipelines
5. **Production Parity**: Dev and CI use same containers as production

### Container Candidates for Graphium

Based on `EVE_LIBRARY_REQUIREMENTS.md`, Graphium could benefit from:

- **CouchDB** (✅ Available): Primary database
- **DragonflyDB** (✅ Available): Cache for agent coordination
- **GraphDB** (✅ Available): RDF/semantic queries
- **RabbitMQ** (✅ Available): Agent message queue
- **Grafana + Stack** (✅ Available): Observability
- **OpenSearch** (✅ Available): Log aggregation from agents

---

## 5. Migration Path

### Phase 1: Testing (Immediate - Low Risk)

Replace test mocks with real containers:

```go
// Before (mocked):
func TestWithMockDB(t *testing.T) {
    mockDB := NewMockCouchDB()
    // ...
}

// After (real container):
func TestWithRealDB(t *testing.T) {
    couchURL, cleanup, _ := testing.SetupCouchDB(ctx, t, nil)
    defer cleanup()
    // ... use real database
}
```

**Impact:** Better test coverage, catches integration issues earlier

### Phase 2: Development Environment (Low Risk)

Create `graphium-dev-stack.json` and use in development:

```bash
# Start entire dev environment
go run cmd/graphium/main.go stack deploy graphium-dev-stack.json

# Later: stop everything
go run cmd/graphium/main.go stack stop graphium-dev
```

**Impact:** Faster onboarding, consistent dev environments

### Phase 3: CI/CD Integration (Medium Risk)

Add stack-based integration tests to CI:

```yaml
# .github/workflows/test.yml
- name: Run Integration Tests
  run: go test -tags=integration ./...
  # Tests automatically spin up containers via stacks
```

**Impact:** More comprehensive CI, earlier bug detection

### Phase 4: Production Deployment (Higher Risk)

Use EVE containers/stacks for production deployment orchestration.

**Recommendation:** Start with non-critical services first.

---

## 6. Compatibility Report

### ✅ Fully Compatible

All existing Graphium code continues to work. The new features are **additive only**.

- `eve.evalgo.org/db` - All existing functions unchanged
- No breaking changes to APIs
- New packages are separate (`containers/*`, `stacks/*`)

### ⚠️ Version Considerations

- **Minimum Go Version:** 1.21 (for generics in db package)
- **Docker Required:** For containers/stacks features
- **testcontainers-go:** Added as dependency (testing only)

### 🔧 Action Items

1. ✅ **Update go.mod:** Done - now uses local EVE at `../eve`
2. ✅ **Verify Build:** Graphium builds successfully with EVE v0.0.12
3. ⏳ **Update Tests:** Consider migrating to containerized integration tests
4. ⏳ **Create Dev Stack:** Create `graphium-dev-stack.json` for developers
5. ⏳ **Documentation:** Update Graphium docs with EVE capabilities

---

## 7. Example Integration Code

### Add Container-Based Integration Test

```go
// tests/integration/storage_integration_test.go
// +build integration

package integration

import (
    "context"
    "testing"

    "eve.evalgo.org/containers/testing"
    "github.com/stretchr/testify/require"
)

func TestStorageWithRealCouchDB(t *testing.T) {
    ctx := context.Background()

    // Spin up CouchDB container
    couchURL, cleanup, err := testing.SetupCouchDB(ctx, t, nil)
    require.NoError(t, err)
    defer cleanup()

    // Initialize storage with real DB
    storage := NewStorage(couchURL, "graphium-test")

    // Test actual storage operations
    host := &Host{ID: "test-host", Name: "test"}
    err = storage.SaveHost(host)
    require.NoError(t, err)

    retrieved, err := storage.GetHost("test-host")
    require.NoError(t, err)
    require.Equal(t, host.Name, retrieved.Name)
}
```

### Create Development Stack

Create `configs/dev-stack.json`:

```json
{
  "@context": "https://schema.org",
  "@type": "ItemList",
  "name": "Graphium Development Stack",
  "description": "Complete development environment for Graphium",
  "network": {
    "name": "graphium-dev",
    "driver": "bridge",
    "createIfNotExists": true
  },
  "itemListElement": [
    {
      "@type": "SoftwareApplication",
      "position": 1,
      "name": "couchdb",
      "applicationCategory": "DatabaseApplication",
      "image": "couchdb:4.3.0",
      "environment": {
        "COUCHDB_USER": "admin",
        "COUCHDB_PASSWORD": "graphium-dev-password"
      },
      "ports": [
        {"containerPort": 5984, "hostPort": 5984}
      ],
      "volumeMounts": [
        {
          "source": "graphium-couchdb-data",
          "target": "/opt/couchdb/data",
          "type": "volume"
        }
      ],
      "healthCheck": {
        "type": "http",
        "path": "/_up",
        "port": 5984,
        "interval": 10
      }
    },
    {
      "@type": "SoftwareApplication",
      "position": 2,
      "name": "dragonflydb",
      "applicationCategory": "CacheApplication",
      "image": "docker.dragonflydb.io/dragonflydb/dragonfly:v1.26.1",
      "ports": [
        {"containerPort": 6379, "hostPort": 6379}
      ],
      "healthCheck": {
        "type": "command",
        "command": ["redis-cli", "ping"],
        "interval": 5
      }
    },
    {
      "@type": "SoftwareApplication",
      "position": 3,
      "name": "grafana",
      "applicationCategory": "MonitoringApplication",
      "image": "grafana/grafana:12.3.0-18893060694",
      "ports": [
        {"containerPort": 3000, "hostPort": 3000}
      ],
      "environment": {
        "GF_SECURITY_ADMIN_PASSWORD": "graphium-dev"
      },
      "healthCheck": {
        "type": "http",
        "path": "/api/health",
        "port": 3000,
        "interval": 15
      }
    }
  ]
}
```

Then use in code:

```go
// cmd/graphium/main.go or separate dev tool
func startDevEnvironment() error {
    ctx, cli := common.CtxCli("unix:///var/run/docker.sock")
    defer cli.Close()

    stack, err := stacks.LoadStackFromFile("configs/dev-stack.json")
    if err != nil {
        return err
    }

    deployment, err := production.DeployStack(ctx, cli, stack)
    if err != nil {
        return err
    }

    log.Printf("Development environment ready!")
    log.Printf("CouchDB: http://localhost:5984")
    log.Printf("DragonflyDB: localhost:6379")
    log.Printf("Grafana: http://localhost:3000")

    return nil
}
```

---

## 8. Next Steps

### Recommended Actions

1. **Review New Features:** Familiarize team with containers/stacks capabilities
2. **Pilot Integration:** Add one container-based integration test
3. **Create Dev Stack:** Define `graphium-dev-stack.json`
4. **Update CI:** Add integration tests to CI pipeline
5. **Document:** Update Graphium README with EVE v0.0.12 features

### Questions to Consider

- Which services should be containerized first?
- Should dev environment use Docker Compose or EVE stacks?
- How to handle container lifecycle in CI/CD?
- Production deployment strategy for containerized services?

---

## Summary

EVE v0.0.12 brings powerful container orchestration capabilities to Graphium:

- **16 production-ready containers** for databases, messaging, observability
- **Multi-container stacks** with dependency management
- **schema.org compliant** definitions for interoperability
- **Testing and production** support
- **Zero breaking changes** - fully backward compatible

Graphium can now:
- Replace test mocks with real containers
- Define entire development environments declaratively
- Deploy multi-container stacks with one command
- Benefit from battle-tested container configurations

**Status:** ✅ Ready to use - Graphium builds successfully with EVE v0.0.12
