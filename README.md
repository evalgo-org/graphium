# Graphium 🧬

> The Essential Element for Container Intelligence

Graphium is a **semantic container orchestration platform** that uses knowledge graphs to manage multi-host Docker infrastructure. It treats containers as semantic nodes in a graph database, enabling intelligent queries about dependencies, topology, and relationships.

## Features

- 🧬 **Semantic Graphs** - JSON-LD native knowledge representation
- 🔍 **Smart Queries** - Traverse relationships, find dependencies, analyze impact
- 🌐 **Multi-Host** - Distributed by design with CouchDB backend
- ⚡ **Real-time** - WebSocket updates for instant visibility with Docker agent
- 🎯 **Type-Safe** - Validated JSON-LD at every step
- 📊 **REST API** - Complete HTTP API with 40+ endpoints
- 🔐 **Validation** - Built-in JSON-LD schema validation
- 🐳 **Docker Agent** - Automatic container discovery and synchronization
- 🎨 **Modern Web UI** - Type-safe Templ templates with HTMX interactivity
- 📦 **Stack Management** - Deploy multi-container applications across hosts
- 🏥 **Integrity Service** - Automated health checks and database repair
- 🔑 **Authentication** - JWT-based auth with role-based access control

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

### ✅ Phase 6: Docker Agent (Complete)
- Real-time container discovery and monitoring
- Automatic synchronization with API server
- Event-driven updates for container lifecycle
- Rate limiting and error handling
- **Files**: `agent/agent.go` (444 lines)

### ✅ Phase 9: Web UI (Complete)
- Modern dark-themed web interface
- Type-safe Templ templates with hot reload
- HTMX for dynamic interactivity
- Real-time container and host monitoring
- Stack management interface
- **Files**: `internal/web/` (2,500+ lines)
- **Access**: http://localhost:8095/

### ✅ Phase 10: Stack Management (Complete)
- Multi-container application deployment
- Distributed orchestration across hosts
- Multiple placement strategies (auto, manual, spread, datacenter)
- Stack status monitoring and lifecycle management
- **Files**: `internal/orchestration/`, `internal/commands/stack.go`

### ✅ Phase 11: OpenAPI Documentation (Complete)
- Full API documentation with Swagger/OpenAPI 3.0
- Interactive Swagger UI at `/docs`
- 40+ documented endpoints with request/response schemas
- **Access**: http://localhost:8095/docs

### ✅ Phase 12: Database Integrity (Complete)
- Automated integrity scanning and health checks
- Duplicate detection and resolution
- Repair plan generation and execution
- Audit logging for all operations
- **Files**: `internal/integrity/` (1,200+ lines)

### 🚧 Pending
- Graph visualization (D3.js/Cytoscape)
- Containerd runtime support
- Comprehensive testing suite
- Performance optimizations

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

#### Docker Agent

```bash
# Start agent for automatic container discovery
graphium agent \
  --api-url http://localhost:8095 \
  --host-id $(hostname) \
  --datacenter dc1

# With custom Docker socket
graphium agent \
  --api-url http://localhost:8095 \
  --host-id my-host \
  --datacenter us-east \
  --docker-socket /var/run/docker.sock
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

#### Stack Management

```bash
# Deploy a stack
graphium stack deploy my-stack.yaml

# List all stacks
graphium stack list

# Show stack status
graphium stack status my-stack

# Stop a stack
graphium stack stop my-stack

# Remove a stack
graphium stack remove my-stack
```

#### Database Integrity

```bash
# Check database health
graphium integrity health

# Scan for integrity issues
graphium integrity scan

# Create a repair plan
graphium integrity plan <scan-id> --strategy latest-wins

# Execute repairs
graphium integrity repair <plan-id>
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
  port: 8095
  read_timeout: 30
  write_timeout: 30
  shutdown_timeout: 10
  debug: true
  tls_enabled: false

couchdb:
  url: http://localhost:5984
  database: graphium
  username: admin
  password: password
  timeout: 30

agent:
  api_url: http://localhost:8095
  host_id: ""  # Auto-detected from hostname
  datacenter: "dc1"
  docker_socket: "/var/run/docker.sock"
  sync_interval: 30
  token: ""  # Agent authentication token

authentication:
  enabled: true
  jwt_secret: "your-secret-key-here"
  jwt_expiry: 3600
  session_expiry: 86400

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
┌─────────────┐              ┌──────────────────┐
│  CLI/User   │              │   Docker Agent   │
│   Browser   │              │ (per-host)       │
└──────┬──────┘              └────────┬─────────┘
       │                              │
       │                              │ Auto-sync
       ▼                              ▼
┌─────────────────────────────────────────────────┐
│              API Server (Echo)                  │
│  - REST endpoints (40+)                         │
│  - WebSocket (real-time updates)                │
│  - Web UI (Templ/HTMX)                          │
│  - Authentication (JWT/Sessions)                │
│  - Validation & Integrity                       │
└──────────────────┬──────────────────────────────┘
                   │
                   ▼
          ┌────────────────┐
          │ Storage Layer  │
          │ - CRUD ops     │
          │ - Graph queries│
          │ - MapReduce    │
          │ - Change feed  │
          └────────┬───────┘
                   │
                   ▼
          ┌────────────────┐
          │    CouchDB     │
          │ - Documents    │
          │ - Views        │
          │ - Replication  │
          └────────────────┘
```

## Project Structure

```
graphium/
├── agent/                  # Docker agent for container discovery
│   └── agent.go           # Agent implementation
├── cmd/graphium/          # CLI entry point
│   └── main.go
├── internal/
│   ├── api/               # REST API server (Echo)
│   │   ├── server.go
│   │   ├── handlers_*.go
│   │   ├── websocket.go
│   │   ├── graph.go
│   │   └── middleware.go
│   ├── auth/              # Authentication & authorization
│   │   ├── auth.go
│   │   ├── jwt.go
│   │   ├── session.go
│   │   └── middleware.go
│   ├── commands/          # CLI commands (Cobra)
│   │   ├── root.go
│   │   ├── server.go
│   │   ├── agent.go
│   │   ├── query.go
│   │   ├── stack.go
│   │   ├── integrity.go
│   │   └── validate.go
│   ├── config/            # Configuration management
│   │   └── config.go
│   ├── integrity/         # Database integrity service
│   │   ├── service.go
│   │   ├── scan.go
│   │   ├── repair.go
│   │   ├── audit.go
│   │   └── types.go
│   ├── orchestration/     # Stack orchestration
│   │   ├── orchestrator.go
│   │   ├── deployment.go
│   │   └── placement.go
│   ├── storage/           # CouchDB storage layer
│   │   ├── storage.go
│   │   ├── graph.go
│   │   ├── changes.go
│   │   └── stacks.go
│   ├── validation/        # JSON-LD validation
│   │   └── validator.go
│   ├── web/               # Web UI (Templ templates)
│   │   ├── handler.go
│   │   ├── templates/
│   │   └── components/
│   └── version/           # Version information
├── models/                # JSON-LD data models
│   ├── container.go
│   ├── host.go
│   ├── stack.go
│   └── user.go
├── static/                # Web assets
│   └── css/
├── tests/fixtures/        # Test data
├── configs/               # Configuration files
│   └── config.yaml
├── docs/                  # OpenAPI documentation
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

- **Language**: Go 1.23+
- **Web Framework**: Echo v4.13+
- **Database**: CouchDB 3.3+ (via EVE library)
- **Templates**: Templ v0.3+ (type-safe Go templates)
- **Frontend**: HTMX v1.9+ (dynamic interactivity)
- **Validation**: json-gold (JSON-LD processor)
- **WebSocket**: gorilla/websocket
- **Authentication**: JWT (golang-jwt/jwt), gorilla/sessions
- **CLI**: Cobra, Viper
- **Task Runner**: Task (go-task)
- **Documentation**: Swagger/OpenAPI 3.0
- **Container Runtime**: Docker API (containerd planned)

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
