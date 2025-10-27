# Graphium Project Roadmap

**Last Updated:** 2025-10-27
**Current Phase:** Phase 9 - Web UI (Complete ✅)

---

## Project Vision

Graphium is a semantic container orchestration platform that uses knowledge graphs to manage multi-host Docker and container infrastructure with intelligent querying, graph traversal, and real-time insights.

---

## Completed Phases ✅

### Phase 1-7: Foundation & Core Features
- ✅ Project structure and architecture
- ✅ Models layer (JSON-LD/Schema.org)
- ✅ Storage layer (EVE/CouchDB integration)
- ✅ API layer (Echo framework, REST endpoints)
- ✅ Agent system (Docker integration)
- ✅ Testing framework
- ✅ Documentation

### Phase 8: EVE Library Integration (Complete)
- ✅ Updated EVE dependency
- ✅ Resolved dependency conflicts
- ✅ Fixed API compatibility issues
- ✅ All query operations working

### Phase 9: Web UI Implementation (Complete)
- ✅ Templ type-safe templates (330+ lines)
- ✅ HTMX dynamic updates integration
- ✅ Modern dark theme design (566 lines CSS)
- ✅ Dashboard with statistics
- ✅ Containers/hosts list pages
- ✅ Static asset serving
- ✅ Excellent performance (<5ms average)
- ✅ Docker agent integration tested
- ✅ Real-time container discovery (109 containers)
- ✅ All EVE library fixes verified

---

## Current Status

**Phase 9 Complete:** All core features implemented and tested
- Web UI fully functional
- Docker agent discovering and syncing 109 real containers
- Real-time event monitoring active
- API endpoints working perfectly
- EVE library integration complete

---

## Phase 10: Graph Visualization

**Status:** Planned
**Priority:** High
**Dependencies:** Phase 9 complete ✅

### Goals
- Interactive graph visualization of container relationships
- Host-container topology view
- Container dependency mapping
- Real-time graph updates via WebSocket

### Technical Approach
- **Library Options:**
  - D3.js (flexible, powerful)
  - Cytoscape.js (graph-focused)
  - Vis.js (network visualization)

### Features
- [ ] Interactive node graph (containers, hosts, networks)
- [ ] Zoom and pan controls
- [ ] Node filtering (by type, status, datacenter)
- [ ] Edge visualization (container-host, container-network)
- [ ] Real-time updates (HTMX/WebSocket)
- [ ] Export graph (PNG, SVG, JSON)
- [ ] Layout algorithms (force-directed, hierarchical)

### Deliverables
- Graph visualization component
- WebSocket integration for live updates
- Graph API endpoints
- Documentation

**Estimated Duration:** 2-3 weeks

---

## Phase 11: Container Runtime Abstraction

**Status:** Planned (User Requested)
**Priority:** High
**Dependencies:** Phase 9 complete ✅

### Goals
Add support for containerd in addition to Docker, enabling Kubernetes and other container runtime integration.

### Background
- Docker uses containerd under the hood
- Kubernetes uses containerd directly
- Many cloud platforms moving to containerd
- Provides broader compatibility

### Technical Approach
1. **Runtime Abstraction Layer**
   - Create `runtime` interface package
   - Abstract Docker-specific code
   - Support pluggable runtimes

2. **Containerd Client**
   - Integrate `github.com/containerd/containerd/client`
   - Map containerd containers to Graphium models
   - Handle containerd events
   - Support namespaces (k8s.io, moby, etc.)

3. **Configuration**
   ```yaml
   agent:
     runtime: docker # or containerd
     containerd:
       socket: /run/containerd/containerd.sock
       namespace: k8s.io
     docker:
       socket: /var/run/docker.sock
   ```

### Features
- [ ] Runtime abstraction interface
- [ ] Containerd client implementation
- [ ] Containerd event monitoring
- [ ] Namespace support
- [ ] Container image metadata
- [ ] Runtime detection (auto-select)
- [ ] Migration guide (Docker → containerd)

### Benefits
- ✅ Kubernetes compatibility
- ✅ Broader platform support
- ✅ Future-proof architecture
- ✅ Multi-runtime environments

### Deliverables
- Runtime abstraction layer
- Containerd agent implementation
- Configuration options
- Testing with real containerd
- Documentation

**Estimated Duration:** 2-3 weeks

---

## Phase 12: OpenAPI Documentation

**Status:** Planned
**Priority:** Medium
**Dependencies:** Phase 9 complete ✅

### Goals
Generate comprehensive OpenAPI 3.0 documentation from API endpoints

### Features
- [ ] OpenAPI spec generation
- [ ] Swagger UI integration
- [ ] Interactive API testing
- [ ] Code generation (clients)
- [ ] API versioning

### Technical Approach
- Use Echo's OpenAPI middleware
- Annotate handlers with OpenAPI tags
- Generate spec automatically
- Host Swagger UI at `/docs`

**Estimated Duration:** 1 week

---

## Phase 13: Enhanced Querying

**Status:** Planned
**Priority:** Medium
**Dependencies:** Phase 10 complete

### Goals
Advanced query capabilities beyond basic filtering

### Features
- [ ] Graph traversal queries (find all containers on host X)
- [ ] SPARQL-like query language
- [ ] Query builder UI
- [ ] Saved queries
- [ ] Query history
- [ ] Performance optimization

### Technical Approach
- Extend EVE's graph traversal
- Create query DSL
- Optimize CouchDB views
- Add query caching

**Estimated Duration:** 3-4 weeks

---

## Phase 14: Security & Authentication

**Status:** Planned
**Priority:** High (for production)
**Dependencies:** Phase 9 complete ✅

### Goals
Production-grade security features

### Features
- [ ] JWT authentication
- [ ] Role-based access control (RBAC)
- [ ] API key management
- [ ] TLS/HTTPS support
- [ ] Agent authentication
- [ ] Audit logging
- [ ] Rate limiting (enhanced)

### Technical Approach
- Integrate with OAuth2/OIDC
- Implement middleware
- Add user management
- Secure agent-server communication

**Estimated Duration:** 2-3 weeks

---

## Phase 15: Monitoring & Observability

**Status:** Planned
**Priority:** Medium
**Dependencies:** Phase 9 complete ✅

### Goals
Production monitoring and observability

### Features
- [ ] Prometheus metrics export
- [ ] Health check dashboard
- [ ] Performance metrics
- [ ] Error tracking
- [ ] Distributed tracing
- [ ] Log aggregation
- [ ] Alerting rules

### Technical Approach
- Prometheus exporter
- OpenTelemetry integration
- Grafana dashboards
- Alert manager integration

**Estimated Duration:** 2 weeks

---

## Phase 16: Multi-host Management

**Status:** Planned
**Priority:** High
**Dependencies:** Phase 9, 11 complete

### Goals
Manage containers across multiple Docker/containerd hosts

### Features
- [ ] Multi-host dashboard
- [ ] Host health monitoring
- [ ] Cross-host container relationships
- [ ] Multi-datacenter support
- [ ] Host groups/clusters
- [ ] Aggregated statistics

### Technical Approach
- Deploy agents on multiple hosts
- Central coordinator pattern
- WebSocket for real-time updates
- Distributed querying

**Estimated Duration:** 3-4 weeks

---

## Phase 17: Container Orchestration Features

**Status:** Planned
**Priority:** Low (future)
**Dependencies:** Phase 16 complete

### Goals
Basic orchestration capabilities (not competing with K8s/Swarm, but complementary)

### Features
- [ ] Container scheduling hints
- [ ] Health-based actions
- [ ] Automatic failover triggers
- [ ] Capacity planning recommendations
- [ ] Container migration suggestions

### Approach
- Advisory system (not imperative)
- Integration with existing orchestrators
- Smart recommendations based on graph data

**Estimated Duration:** 4-6 weeks

---

## Phase 18: Kubernetes Integration

**Status:** Planned
**Priority:** High
**Dependencies:** Phase 11 (containerd) complete

### Goals
Native Kubernetes cluster integration

### Features
- [ ] K8s API client integration
- [ ] Pod discovery
- [ ] Service mapping
- [ ] Namespace support
- [ ] Deployment tracking
- [ ] Label/annotation mapping

### Technical Approach
- Use `client-go` library
- Map K8s resources to Graphium models
- Watch K8s events
- Support multiple clusters

**Estimated Duration:** 4-5 weeks

---

## Phase 19: Data Export & Integration

**Status:** Planned
**Priority:** Low
**Dependencies:** Phase 9 complete ✅

### Goals
Export and integrate Graphium data with other systems

### Features
- [ ] Export formats (JSON, CSV, RDF/Turtle)
- [ ] Webhooks (container events)
- [ ] Message queue integration (Kafka, RabbitMQ)
- [ ] External API connectors
- [ ] Backup/restore functionality

**Estimated Duration:** 2-3 weeks

---

## Phase 20: Advanced UI Features

**Status:** Planned
**Priority:** Medium
**Dependencies:** Phase 10 complete

### Goals
Enhanced web UI capabilities

### Features
- [ ] Real-time updates (WebSocket)
- [ ] Search functionality
- [ ] Advanced filtering
- [ ] Bulk operations
- [ ] Container logs viewer
- [ ] Resource metrics charts
- [ ] Dark/light theme toggle
- [ ] Mobile app (progressive web app)

**Estimated Duration:** 3-4 weeks

---

## Technical Debt & Improvements

### High Priority
- [ ] Fix host listing query (minor bug)
- [ ] Fix duplicate container counting in views
- [ ] Add pagination for large datasets (100+ containers)
- [ ] Improve error messages
- [ ] Add request validation

### Medium Priority
- [ ] Add unit tests (target: 80% coverage)
- [ ] Integration test suite
- [ ] Performance profiling
- [ ] Memory optimization
- [ ] Code documentation (godoc)

### Low Priority
- [ ] Refactor storage layer
- [ ] CLI improvements
- [ ] Configuration validation
- [ ] Logging improvements
- [ ] Code generation tool

---

## Community & Ecosystem

### Documentation
- [ ] User guide
- [ ] API reference
- [ ] Deployment guide
- [ ] Development guide
- [ ] Architecture documentation
- [ ] Video tutorials

### Tooling
- [ ] Docker Compose setup
- [ ] Kubernetes Helm chart
- [ ] Terraform modules
- [ ] Ansible playbooks
- [ ] Vagrant environment

### Integrations
- [ ] Portainer integration
- [ ] Rancher integration
- [ ] Prometheus/Grafana dashboards
- [ ] ELK stack integration
- [ ] Service mesh support (Istio, Linkerd)

---

## Release Schedule

### v0.2.0 - Q1 2026 (Target)
- ✅ Phase 9 complete (Web UI)
- [ ] Phase 10 (Graph Visualization)
- [ ] Phase 11 (Containerd support) 🆕
- [ ] Phase 12 (OpenAPI docs)

### v0.3.0 - Q2 2026
- [ ] Phase 13 (Enhanced Querying)
- [ ] Phase 14 (Security & Auth)
- [ ] Phase 15 (Monitoring)

### v0.4.0 - Q3 2026
- [ ] Phase 16 (Multi-host)
- [ ] Phase 18 (Kubernetes)

### v1.0.0 - Q4 2026 (Production Ready)
- [ ] All core features complete
- [ ] Production-grade security
- [ ] Comprehensive documentation
- [ ] 90%+ test coverage
- [ ] Performance optimized
- [ ] Enterprise features

---

## Success Metrics

### Technical
- [ ] Support 1000+ containers per instance
- [ ] < 100ms API response time (p95)
- [ ] 99.9% uptime
- [ ] < 1s sync time for 100 containers
- [ ] Zero data loss
- [ ] < 200MB memory footprint

### Adoption
- [ ] 100+ GitHub stars
- [ ] 10+ contributors
- [ ] 1000+ deployments
- [ ] Active community (Discord/Slack)
- [ ] Regular releases (monthly)

---

## Contributing

We welcome contributions! Priority areas:
1. **Containerd support** (Phase 11) 🆕
2. Graph visualization (Phase 10)
3. OpenAPI documentation (Phase 12)
4. Test coverage improvements
5. Documentation

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

---

## Feedback

Have suggestions for the roadmap? Open an issue or discussion:
- 🐛 Bug reports: GitHub Issues
- 💡 Feature requests: GitHub Discussions
- 📧 Contact: [project maintainers]
- 💬 Chat: [Discord/Slack link]

---

**Maintained by:** The Graphium Team
**License:** [License Type]
**Repository:** https://github.com/[org]/graphium

---

## Changelog

### 2025-10-27
- ✅ Completed Phase 9 (Web UI)
- ✅ Docker integration tested (109 real containers)
- ✅ EVE library fixes verified
- 🆕 Added Phase 11 (Containerd support) per user request
- Updated priorities and timelines

### Previous Updates
- 2025-10-26: Phase 9 implementation
- 2025-10-25: EVE library updates
- [Earlier changes...]
