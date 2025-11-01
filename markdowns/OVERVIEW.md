# Graphium 🧬

## The Essential Element for Container Intelligence

### What is Graphium?

Graphium is a **semantic container orchestration platform** that treats your infrastructure as a **knowledge graph**. Instead of managing containers as isolated entities, Graphium understands the *relationships* between containers, hosts, networks, and volumes, enabling intelligent queries and insights across your entire multi-host Docker infrastructure.

### The Problem It Solves

**Traditional container management tools (Docker CLI, Portainer, etc.) treat containers as lists:**
- "Show me all containers"
- "Find containers on host-5"
- Hard to answer: "What will break if I restart this database?"

**Graphium treats containers as a semantic graph:**
- "Show me all containers that depend on this database"
- "Find overloaded hosts in datacenter-east with >50 containers"
- "Traverse the dependency chain 3 levels deep from this service"
- "What containers will be affected if I take down this network?"

### Core Concept: Infrastructure as a Knowledge Graph
```
Traditional View:
Container1, Container2, Container3...
Host1, Host2, Host3...

Graphium View:
Container1 --[runs on]--> Host1 --[located in]--> Datacenter-East
Container1 --[depends on]--> Database1
Container1 --[connects to]--> Network1
Container2 --[depends on]--> Container1
```

Every entity is a **semantic node** (JSON-LD), every relationship is an **edge**. You can traverse, query, and reason about your infrastructure like a graph database.

---

## Use Cases

### 1. **Multi-Host Container Management**
Manage Docker containers across dozens or hundreds of hosts from a single control plane.
```bash
# Deploy agent on each host
graphium agent --host-id prod-server-1 --datacenter us-east

# Query from anywhere
graphium query list containers --datacenter us-east --status running
```

### 2. **Dependency Discovery**
Understand what will break before you break it.
```bash
# What depends on this database?
graphium query traverse postgres-db --show-dependents

# What networks does this container use?
graphium query traverse web-app --depth 2 --type network
```

### 3. **Impact Analysis**
Before maintenance or changes, see the blast radius.
```bash
# If I restart this host, what's affected?
graphium query dependents host-5

# What containers are on overloaded hosts?
graphium query containers --where "host.cpu > 90"
```

### 4. **Semantic Queries**
Query by meaning, not just properties.
```bash
# Find all web servers in production
graphium query containers --type SoftwareApplication --location production

# Which containers are using deprecated images?
graphium query containers --where "image contains 'alpine:3.10'"
```

### 5. **Real-Time Monitoring**
WebSocket-powered live updates across all hosts.
```javascript
// In the web UI
ws://graphium.io/ws
// Instant updates when containers start/stop/change
```

### 6. **Infrastructure as Code (IaC) Validation**
Validate container definitions before deployment.
```bash
# Validate JSON-LD definition
graphium validate container my-container.json

# Returns detailed validation errors
✗ Validation failed:
  - Image: This field is required
  - HostedOn: Invalid host reference
```

---

## Architecture

### Components
```
┌─────────────────────────────────────────────────────────┐
│                    Web Browser / CLI                     │
│               (Query, Visualize, Manage)                 │
└───────────────────────┬─────────────────────────────────┘
                        │ HTTP/WS
                        │
        ┌───────────────▼────────────────┐
        │     Graphium API Server         │
        │        (Echo/Go)                │
        │                                 │
        │  - REST API (JSON-LD)           │
        │  - WebSocket (real-time)        │
        │  - Validation Engine            │
        │  - Graph Query Engine           │
        └────────┬──────────────┬─────────┘
                 │              │
        ┌────────▼────┐    ┌────▼──────────────┐
        │  CouchDB    │    │  Graphium Agents   │
        │             │    │  (on each host)    │
        │  - Nodes    │    │                    │
        │  - Edges    │◄───┤  - Docker Monitor  │
        │  - Views    │    │  - Event Stream    │
        │  - Changes  │    │  - Sync Engine     │
        └─────────────┘    └───────────────────┘
                                    │
                                    ▼
                            ┌──────────────┐
                            │   Docker     │
                            │   Daemon     │
                            └──────────────┘
```

### Technology Stack

**Backend:**
- Go 1.21+ (performance, concurrency)
- Echo framework (HTTP server)
- Cobra (CLI)
- Viper (configuration)

**Storage:**
- CouchDB (distributed, conflict-free replication)
- JSON-LD (semantic data format)
- Graph views (MapReduce queries)

**Frontend:**
- Templ (type-safe HTML templates)
- HTMX (dynamic updates without JS frameworks)
- WebSocket (real-time updates)

**DevOps:**
- Task (build automation)
- Nixpacks (containerization)
- GitHub Actions (CI/CD)

---

## Key Features

### 🧬 Semantic Data Model
Every entity is JSON-LD with `@context`, `@type`, and `@id`. This enables:
- Standard vocabularies (schema.org)
- Type validation
- Graph traversal
- Semantic reasoning

**Example Container:**
```json
{
  "@context": "https://schema.org",
  "@type": "SoftwareApplication",
  "@id": "urn:uuid:abc-123",
  "name": "nginx-prod",
  "executableName": "nginx:latest",
  "status": "running",
  "hostedOn": "urn:uuid:host-456",
  "dateCreated": "2024-01-15T10:30:00Z"
}
```

### 🔍 Graph Queries
Query infrastructure as a graph, not a table.
```bash
# Traverse relationships
graphium query traverse container-id --depth 3

# Find dependents (reverse lookup)
graphium query dependents database-id

# Complex filters
graphium query containers \
  --where "status=running" \
  --where "hostedOn.location=us-east" \
  --where "dateCreated > 2024-01-01"
```

### 🌐 Multi-Host by Design
Built from the ground up for distributed infrastructure.

- **Agents** run on each Docker host
- **CouchDB** handles distributed state with conflict-free replication
- **API server** provides unified control plane
- **WebSocket** streams updates across the cluster

### ⚡ Real-Time Updates
Changes propagate instantly via WebSocket.
```javascript
const ws = new WebSocket('ws://graphium.io/ws');
ws.onmessage = (event) => {
  const update = JSON.parse(event.data);
  console.log('Container updated:', update);
};
```

### 🎯 Type-Safe Generation
Models are the single source of truth. Everything else generates:
```
models/*.go  (define once)
    │
    ├─> Storage layer (CouchDB views)
    ├─> API handlers (Echo routes)
    ├─> Validation rules (go-playground/validator)
    ├─> Web templates (Templ)
    └─> Documentation (OpenAPI, Hydra)
```
```bash
# One command generates everything
task generate
```

### 🔒 Validation at Every Level
Multi-layered validation ensures data integrity:

1. **Struct tags** - Go validator annotations
2. **JSON-LD** - Schema validation with json-gold
3. **Business logic** - Custom validation rules
4. **API level** - Pre-save validation endpoint
```bash
# Validate before deploying
graphium validate container production-config.json
```

---

## Ideal For

### ✅ You Should Use Graphium If:

- Managing containers across **multiple hosts** (5-500+ servers)
- Need to understand **dependencies and relationships**
- Want **semantic queries** beyond basic filters
- Require **real-time visibility** across infrastructure
- Building **multi-datacenter** container platforms
- Need **distributed state** without complex orchestration
- Want **type-safe** infrastructure management
- Prefer **knowledge graphs** over relational databases

### ❌ You Probably Don't Need Graphium If:

- Single host with <10 containers (use Docker CLI)
- Already using Kubernetes (K8s has its own orchestration)
- Need complex scheduling/networking (use K8s/Nomad)
- Don't care about relationships between entities
- Just need basic monitoring (use Portainer/Dozzle)

---

## Comparison to Other Tools

### vs Docker CLI
- **Docker CLI:** Manages single host, no relationships
- **Graphium:** Multi-host, understands dependencies, semantic queries

### vs Kubernetes
- **Kubernetes:** Heavy orchestration, complex, scheduling-focused
- **Graphium:** Lightweight discovery, relationship-focused, simpler

### vs Portainer
- **Portainer:** UI for Docker, single/multi host, basic management
- **Graphium:** Graph queries, semantic data, programmatic access

### vs Nomad
- **Nomad:** Workload orchestration, scheduling, placement
- **Graphium:** Knowledge graph, discovery, relationships

### Graphium's Niche
**Discovery and relationships for multi-host Docker**, without the complexity of full orchestration. Think of it as:
- "Graph database for your infrastructure"
- "Google for your containers"
- "LinkedIn for your Docker hosts"

---

## Quick Start

### Installation
```bash
# Go install
go install evalgo.org/graphium@latest

# Or download binary
curl -sSL https://graphium.sh/install.sh | bash
```

### Deploy
```bash
# 1. Start API server
graphium server

# 2. Deploy agents on each host
graphium agent \
  --host-id prod-01 \
  --datacenter us-east \
  --api-url http://api.graphium.io

# 3. Query from anywhere
graphium query list containers --status running
```

### Use as Library
```go
package main

import (
    "evalgo.org/graphium/pkg/graphium/client"
)

func main() {
    c, _ := client.New("http://localhost:8080")
    
    containers, _ := c.ListContainers(client.Query{
        Status: "running",
        Datacenter: "us-east",
    })
    
    for _, container := range containers {
        println(container.Name)
    }
}
```

---

## Development Workflow
```bash
# Setup
task dev:setup

# Generate code from models
task generate

# Start development environment
task dev

# Run tests
task test

# Build
task build

# Deploy
task nixpacks:build
task docker:up
```

---

## Roadmap

### Current (v0.1)
- ✅ Multi-host Docker monitoring
- ✅ JSON-LD semantic data
- ✅ CouchDB storage
- ✅ Basic CLI
- ✅ Code generation

### Next (v0.2)
- [ ] Web UI with graph visualization
- [ ] Advanced graph queries (Cypher-like syntax)
- [ ] Alerting and notifications
- [ ] Metrics collection
- [ ] Container lifecycle management

### Future (v1.0)
- [ ] Multi-cloud support (not just Docker)
- [ ] Machine learning insights
- [ ] Predictive scaling recommendations
- [ ] Cost optimization
- [ ] Compliance validation

---

## Philosophy

### Design Principles

1. **Graphs > Tables**
   Infrastructure has relationships. Model them explicitly.

2. **Semantics > Schemas**
   Use web standards (JSON-LD, schema.org) instead of proprietary formats.

3. **Generation > Duplication**
   Define once (models), generate everything else.

4. **Distribution > Centralization**
   CouchDB's masterless replication beats single points of failure.

5. **Simplicity > Features**
   Do one thing (graph-based discovery) exceptionally well.

### Why These Technologies?

**Go:** Fast, concurrent, single binary deployment
**CouchDB:** Distributed by design, HTTP-native, conflict-free replication
**JSON-LD:** Semantic web standard, graph-native, extensible
**Echo:** Fast, simple, middleware-friendly
**Nixpacks:** Reproducible builds without Dockerfiles
**Task:** Simpler than Make, more powerful than scripts

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md)

---

## License

MIT License - see [LICENSE](LICENSE)

---

## Support

- 📧 Email: support@evalgo.org
- 💬 Discussions: [GitHub Discussions](https://github.com/evalgo/graphium/discussions)
- 🐛 Issues: [GitHub Issues](https://github.com/evalgo/graphium/issues)
- 📖 Docs: [graphium.evalgo.org](https://graphium.evalgo.org)

---

**Module:** `evalgo.org/graphium`  
**Repository:** [github.com/evalgo/graphium](https://github.com/evalgo/graphium)

Made with 🧬 by [EvalGo](https://evalgo.org)
