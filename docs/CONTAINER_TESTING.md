# Container-Based Testing with EVE

This document explains how to use EVE's container support for integration testing in Graphium.

## Overview

Graphium now uses [EVE](https://github.com/evalgo-org/eve) v0.0.12's container orchestration features to run integration tests with real containerized services instead of mocks or manually-managed databases.

**Benefits:**
- ✅ Test against real CouchDB instead of mocks
- ✅ Isolated test environments (containers are ephemeral)
- ✅ Consistent behavior across dev, CI, and production
- ✅ Automatic cleanup (no leftover test data)
- ✅ Fast startup with health checks

## Running Integration Tests Locally

### Prerequisites

- Docker installed and running
- Go 1.21 or later

### Run Tests

```bash
# Run integration tests (automatically starts CouchDB container)
go test -tags=integration ./tests/integration/... -v

# Run specific test
go test -tags=integration ./tests/integration/ -run TestCouchDBIntegration -v

# Skip long-running tests
go test -tags=integration ./tests/integration/... -short
```

### How It Works

The integration tests use EVE's `SetupCouchDB()` function which:

1. **Pulls CouchDB image** (couchdb:4.3.0) if not cached
2. **Starts container** with random port mapping
3. **Waits for health check** (HTTP GET /_up)
4. **Returns connection URL** (e.g., http://localhost:32769)
5. **Auto-cleanup** when test finishes (via defer)

Example test structure:

```go
func TestCouchDBIntegration(t *testing.T) {
    ctx := context.Background()

    // Start CouchDB container
    couchURL, cleanup, err := evetesting.SetupCouchDB(ctx, t, nil)
    require.NoError(t, err)
    defer cleanup()  // Automatic cleanup

    // Use real CouchDB for tests
    store, _ := storage.New(&config.Config{
        CouchDB: config.CouchDBConfig{
            URL: couchURL,
            Database: "graphium_test",
        },
    })

    // Run tests...
}
```

## CI/CD Integration

### GitHub Actions Example

Add to `.github/workflows/test.yml`:

```yaml
name: Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest

    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.21'

      - name: Run unit tests
        run: go test ./... -short

      - name: Run integration tests with containers
        run: |
          # Integration tests automatically start required containers
          go test -tags=integration ./tests/integration/... -v

      # No need to manually start/stop Docker containers!
      # testcontainers-go handles everything automatically
```

### GitLab CI Example

Add to `.gitlab-ci.yml`:

```yaml
test:integration:
  image: golang:1.21
  services:
    - docker:dind  # Docker-in-Docker for testcontainers
  variables:
    DOCKER_HOST: tcp://docker:2375
    DOCKER_TLS_CERTDIR: ""
  script:
    - go test -tags=integration ./tests/integration/... -v
  only:
    - main
    - merge_requests
```

### Key Points for CI

1. **Docker-in-Docker**: Most CI systems need `docker:dind` service or similar
2. **No manual setup**: testcontainers-go handles container lifecycle
3. **Parallel jobs**: Each job gets isolated containers (no conflicts)
4. **Fast with caching**: Docker images are cached between runs

## Development Environment

### Start Dev Stack

Graphium includes a development stack with CouchDB:

```bash
# Start entire dev environment
go run cmd/graphium-dev/main.go start

# Check status
go run cmd/graphium-dev/main.go status

# Stop environment
go run cmd/graphium-dev/main.go stop

# Remove (including data volumes)
go run cmd/graphium-dev/main.go remove
```

The dev stack is defined in `configs/graphium-dev-stack.json`.

### Access Services

Once started:

- **CouchDB UI**: http://localhost:5984/_utils
- **CouchDB API**: http://localhost:5984
- **Credentials**: admin / graphium-dev-password

### Configure Graphium

Update `configs/config.yaml` or set environment variables:

```yaml
couchdb:
  url: http://localhost:5984
  database: graphium
  username: admin
  password: graphium-dev-password
```

Or via environment:

```bash
export CG_COUCHDB_URL=http://localhost:5984
export CG_COUCHDB_DATABASE=graphium
export CG_COUCHDB_USERNAME=admin
export CG_COUCHDB_PASSWORD=graphium-dev-password
```

## Available Container Types

From EVE v0.0.12, Graphium can use:

### Databases
- ✅ **CouchDB** (currently used)
- PostgreSQL
- BaseX (XML database)
- DragonflyDB (Redis-compatible cache)

### Search & Analytics
- OpenSearch
- OpenSearchDashboards

### Semantic & Graph
- GraphDB (RDF triple store)
- RDF4J

### Messaging
- RabbitMQ

### Infrastructure
- Docker Registry
- LakeFS (data versioning)

See [EVE_NEW_FEATURES_v0.0.12.md](../EVE_NEW_FEATURES_v0.0.12.md) for details.

## Writing New Container Tests

### Basic Pattern

```go
//go:build integration
// +build integration

package integration

import (
    "context"
    "testing"

    evetesting "eve.evalgo.org/containers/testing"
    "github.com/stretchr/testify/require"
)

func TestMyFeature(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    ctx := context.Background()

    // Start container
    serviceURL, cleanup, err := evetesting.SetupCouchDB(ctx, t, nil)
    require.NoError(t, err)
    defer cleanup()

    // Use service in tests
    // ...
}
```

### Custom Configuration

```go
config := &evetesting.CouchDBConfig{
    Image:          "couchdb:4.3.0",
    StartupTimeout: 120 * time.Second,
}

couchURL, cleanup, err := evetesting.SetupCouchDB(ctx, t, config)
```

### Multiple Containers

```go
// Start CouchDB
couchURL, cleanup1, _ := evetesting.SetupCouchDB(ctx, t, nil)
defer cleanup1()

// Start DragonflyDB (cache)
cacheURL, cleanup2, _ := evetesting.SetupDragonflyDB(ctx, t, nil)
defer cleanup2()

// Use both in tests
```

## Troubleshooting

### Docker Not Running

```
Error: Cannot connect to the Docker daemon
```

**Solution**: Start Docker Desktop or Docker daemon

```bash
# Linux
sudo systemctl start docker

# macOS
open -a Docker
```

### Port Conflicts

testcontainers-go uses random ports to avoid conflicts. If you see port errors:

```bash
# Check what's using ports
sudo lsof -i :5984
```

### Slow Container Startup

First run downloads images (can take 1-2 minutes). Subsequent runs are fast:

```bash
# Pre-pull images to speed up tests
docker pull couchdb:4.3.0
```

### CI Failures

Common issues:

1. **Missing Docker**: Ensure CI has Docker available
2. **Permissions**: CI user needs Docker access
3. **Timeouts**: Increase timeout for slow CI runners

```go
config := &evetesting.CouchDBConfig{
    StartupTimeout: 180 * time.Second,  // 3 minutes for slow CI
}
```

## Best Practices

### ✅ DO

- Use build tags (`//go:build integration`) to separate integration tests
- Use `testing.Short()` to skip in quick test runs
- Always defer cleanup functions
- Use require for container setup (fail fast if setup fails)
- Clean up test data at end of tests

### ❌ DON'T

- Don't hardcode localhost:5984 (use dynamic URLs from setup)
- Don't assume container is ready immediately (trust health checks)
- Don't share containers between tests (use fresh containers)
- Don't commit with skipped container setup errors

## Performance

### Typical Timings

- **First run** (image download): 60-120 seconds
- **Subsequent runs** (cached image): 5-10 seconds per test
- **Container startup**: 2-5 seconds (with health checks)
- **Test execution**: Normal test speed

### Optimization Tips

1. **Parallel tests**: Run multiple test packages in parallel
```bash
go test -tags=integration ./... -p 4
```

2. **Shared setup**: Use `TestMain` for shared expensive setup
```go
func TestMain(m *testing.M) {
    // Setup once for all tests in package
    os.Exit(m.Run())
}
```

3. **Docker cache**: Keep images cached, don't clear Docker cache before tests

## Further Reading

- [testcontainers-go Documentation](https://golang.testcontainers.org/)
- [EVE Library](https://github.com/evalgo-org/eve)
- [Graphium EVE Integration](../EVE_NEW_FEATURES_v0.0.12.md)
- [CouchDB Documentation](https://docs.couchdb.org/)
