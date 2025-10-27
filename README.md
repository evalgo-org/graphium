# Graphium 🧬

> The Essential Element for Container Intelligence

Graphium is a **semantic container orchestration platform** that uses knowledge graphs to manage multi-host Docker infrastructure. It treats containers as semantic nodes in a graph database, enabling intelligent queries about dependencies, topology, and relationships.

## Features

- 🧬 **Semantic Graphs** - JSON-LD native knowledge representation
- 🔍 **Smart Queries** - Traverse relationships, find dependencies, analyze impact
- 🌐 **Multi-Host** - Distributed by design with CouchDB backend
- ⚡ **Real-time** - WebSocket updates for instant visibility
- 🎯 **Type-Safe** - Validated JSON-LD at every step
- 📊 **REST API** - Complete HTTP API with 30+ endpoints
- 🔐 **Validation** - Built-in JSON-LD schema validation

## Implementation Status

### ✅ Phase 1: Dependencies (Complete)
- Echo web framework v4
- CouchDB client (eve.evalgo.org/db)
- JSON-LD validation (json-gold)
- WebSocket support (gorilla/websocket)

### ✅ Phase 2: Storage Layer (Complete)
- CouchDB integration with eve library
- CRUD operations for containers and hosts
- MapReduce views for graph queries
- Real-time change monitoring
- **Files**: `internal/storage/` (953 lines)

### ✅ Phase 3: API Server (Complete)
- REST API with Echo framework
- 30+ HTTP endpoints (containers, hosts, query, stats, validation)
- WebSocket real-time updates
- Graceful shutdown
- **Files**: `internal/api/` (1,250+ lines)

### ✅ Phase 4: CLI Enhancement (Complete)
- Query commands (list, traverse, dependents, topology, stats)
- Multiple output formats (table, JSON, tree)
- Direct storage integration
- **Files**: `internal/commands/query.go` (360 lines)

### ✅ Phase 5: JSON-LD Validation (Complete)
- Validation engine with json-gold
- Container and host schema validation
- CLI and API validation
- **Files**: `internal/validation/` (373 lines)

### ✅ Phase 8: DevOps Setup (Complete)
- Automated development environment setup
- Task commands for dev workflow
- CouchDB Docker container management

### ✅ Phase 11: OpenAPI Documentation (Complete)
- Full API documentation with Swagger/OpenAPI 3.0
- Interactive Swagger UI at `/docs`
- 19+ documented endpoints with request/response schemas
- **Access**: http://localhost:8095/docs

### 🚧 Pending
- Agent enhancement (real-time sync)
- Code generation tool
- Web UI (Templ + HTMX)
- Comprehensive testing suite

## Quick Start

### Installation

```bash
# From source
git clone https://github.com/evalgo/graphium
cd graphium
task install
task build
```

### Development Setup

```bash
# One-command setup (installs CouchDB, creates config)
task dev:setup

# Start development server
task dev

# The dev server starts:
# - CouchDB: http://localhost:5984/_utils (admin/password)
# - API: http://localhost:8080
# - WebSocket: ws://localhost:8080/api/v1/ws
```

### Basic Usage

#### Start Server
```bash
# Production mode
graphium server

# Development mode with debug logging
task dev
```

#### Query Commands

```bash
# List containers
graphium query list containers
graphium query list containers --status running --host host-01

# List hosts
graphium query list hosts --datacenter us-east

# Traverse dependency graph
graphium query traverse nginx-web --depth 3

# Find dependents (reverse lookup)
graphium query dependents postgres-db

# Show datacenter topology
graphium query topology us-east

# Infrastructure statistics
graphium query stats
```

#### Validation

```bash
# Validate JSON-LD documents
graphium validate container my-container.json
graphium validate host my-host.json

# Example fixtures
graphium validate container tests/fixtures/valid-container.json
```

## API Documentation

**🎯 Interactive API Documentation**: [http://localhost:8095/docs](http://localhost:8095/docs)

The API is fully documented using OpenAPI 3.0 (Swagger). Once the server is running, visit the `/docs` endpoint for an interactive interface where you can:
- Browse all available endpoints
- View request/response schemas
- Test API calls directly from the browser
- Download the OpenAPI specification (JSON/YAML)

### API Endpoints Overview

**Containers** (6 endpoints)
- List, get, create, update, delete containers
- Bulk operations support
- Status and host filtering

**Hosts** (6 endpoints)
- List, get, create, update, delete hosts
- Bulk operations support
- Status and datacenter filtering

**Graph & Topology** (3 endpoints)
- Graph visualization data
- Graph statistics
- Multiple layout algorithms

**Query** (6 endpoints)
- Container/host lookups by various criteria
- Dependency graph traversal
- Datacenter topology views

**Statistics** (4 endpoints)
- Infrastructure statistics
- Container/host counts with filters
- Distribution metrics

**Validation** (2 endpoints)
- JSON-LD schema validation
- Container and host document validation

**WebSocket** (2 endpoints)
- Real-time graph updates
- WebSocket connection statistics

**System** (2 endpoints)
- Health check
- Database information

## Example: Container JSON-LD

```json
{
  "@context": "https://schema.org",
  "@type": "SoftwareApplication",
  "@id": "nginx-web-01",
  "name": "nginx-web",
  "executableName": "nginx:latest",
  "status": "running",
  "hostedOn": "host-01",
  "ports": [
    {
      "hostPort": 80,
      "containerPort": 80,
      "protocol": "tcp"
    }
  ],
  "environment": {
    "NGINX_HOST": "example.com"
  },
  "dateCreated": "2024-01-15T10:30:00Z"
}
```

## Example: API Usage

```bash
# Create a container
curl -X POST http://localhost:8080/api/v1/containers \
  -H "Content-Type: application/json" \
  -d '{
    "name": "nginx-web",
    "executableName": "nginx:latest",
    "status": "running",
    "hostedOn": "host-01"
  }'

# List running containers
curl http://localhost:8080/api/v1/containers?status=running

# Traverse dependencies
curl http://localhost:8080/api/v1/query/traverse/nginx-web?depth=3

# Get statistics
curl http://localhost:8080/api/v1/stats
```

## Development

### Available Tasks

```bash
# Show all available tasks
task --list

# Development
task dev:setup          # Set up development environment
task dev                # Start development server
task dev:stop           # Stop development environment
task dev:logs           # Show CouchDB logs
task dev:clean          # Clean dev environment (removes data!)

# Building
task build              # Build binary
task run                # Build and run server
task run:dev            # Run in development mode

# Code Quality
task fmt                # Format code
task lint:fix           # Auto-fix linting issues
task test               # Run tests
task clean              # Clean build artifacts

# Other
task install            # Install dependencies
task generate           # Generate code from models
task version            # Show version info
```

### Configuration

Edit `configs/config.yaml`:

```yaml
server:
  host: 0.0.0.0
  port: 8080
  read_timeout: 30
  write_timeout: 30
  shutdown_timeout: 10
  debug: true

couchdb:
  url: http://localhost:5984
  database: graphium
  username: admin
  password: password
  timeout: 30

logging:
  level: debug
  format: json

security:
  rate_limit: 100
  allowed_origins:
    - "*"
```

## Architecture

```
┌─────────────┐
│   CLI/User  │
└──────┬──────┘
       │
       ▼
┌─────────────────────────┐
│   Commands Layer        │
│  - query (list, etc.)   │
│  - validate             │
│  - server               │
│  - agent                │
└──────┬──────────────────┘
       │
       ├──► Direct Access ─────►┌──────────────────┐
       │                        │ Storage Layer    │
       │                        │ - CRUD ops       │
       │                        │ - Graph queries  │
       │                        │ - Change feed    │
       │                        └────────┬─────────┘
       │                                 │
       ▼                                 ▼
┌─────────────────────────┐    ┌──────────────────┐
│   API Server (Echo)     │    │    CouchDB       │
│  - REST endpoints       │◄───┤  - Documents     │
│  - WebSocket            │    │  - Views         │
│  - Validation           │    │  - Changes feed  │
└─────────────────────────┘    └──────────────────┘
```

## Project Structure

```
graphium/
├── cmd/graphium/           # CLI entry point
├── internal/
│   ├── api/               # REST API server (Echo)
│   │   ├── server.go
│   │   ├── handlers_*.go
│   │   ├── websocket.go
│   │   └── types.go
│   ├── commands/          # CLI commands (Cobra)
│   │   ├── root.go
│   │   ├── server.go
│   │   ├── query.go
│   │   ├── validate.go
│   │   └── agent.go
│   ├── storage/           # CouchDB storage layer
│   │   ├── storage.go
│   │   ├── graph.go
│   │   └── changes.go
│   ├── validation/        # JSON-LD validation
│   │   └── validator.go
│   └── config/            # Configuration
├── models/                # Data models
│   ├── container.go
│   └── host.go
├── tests/fixtures/        # Test data
├── configs/               # Configuration files
└── Taskfile.yml          # Task automation
```

## Documentation

- [OVERVIEW.md](OVERVIEW.md) - Complete vision and use cases
- [PHASE_2_COMPLETE.md](PHASE_2_COMPLETE.md) - Storage layer details
- [PHASE_3_COMPLETE.md](PHASE_3_COMPLETE.md) - API server details
- [PHASE_4_COMPLETE.md](PHASE_4_COMPLETE.md) - CLI enhancement details
- [PHASE_5_COMPLETE.md](PHASE_5_COMPLETE.md) - Validation details
- [CONTRIBUTING.md](CONTRIBUTING.md) - Development guide

## Technologies

- **Language**: Go 1.21+
- **Web Framework**: Echo v4
- **Database**: CouchDB 3.3+
- **Validation**: json-gold (JSON-LD processor)
- **WebSocket**: gorilla/websocket
- **CLI**: Cobra
- **Task Runner**: Task (go-task)

## Module

```
evalgo.org/graphium
```

## License

MIT License - see [LICENSE](LICENSE) file.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development guidelines.

---

Made with 🧬 by EvalGo
