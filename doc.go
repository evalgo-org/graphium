// Package graphium is a semantic container orchestration platform.
//
// # Overview
//
// Graphium uses knowledge graphs and JSON-LD to manage multi-host Docker
// infrastructure with intelligent querying, graph traversal, and real-time insights.
//
// The platform consists of three main components:
//   - API Server: REST API and Web UI for managing infrastructure
//   - Docker Agent: Automatic container discovery and monitoring
//   - Storage Layer: CouchDB-backed graph storage with JSON-LD
//
// # Architecture
//
//	┌─────────────────┐
//	│   Web UI        │
//	│  (Templ/HTMX)   │
//	└────────┬────────┘
//	         │
//	┌────────▼────────┐       ┌─────────────────┐
//	│  API Server     │◄──────┤  Docker Agent   │
//	│  (Echo REST)    │       │  (Discovery)    │
//	└────────┬────────┘       └─────────────────┘
//	         │
//	┌────────▼────────┐
//	│  Storage Layer  │
//	│  (EVE/CouchDB)  │
//	└─────────────────┘
//
// # Core Features
//
// JSON-LD/Schema.org Models:
//   - Type-safe container and host models
//   - Semantic relationships and graph traversal
//   - Standards-based vocabulary
//
// REST API:
//   - Full CRUD operations for containers and hosts
//   - Advanced graph queries and topology views
//   - WebSocket support for real-time updates
//   - Comprehensive validation and error handling
//
// Docker Agent:
//   - Automatic container discovery
//   - Real-time event monitoring
//   - Periodic synchronization
//   - Multi-host support
//
// Web UI:
//   - Modern dark theme interface
//   - Real-time updates with HTMX
//   - Graph visualization
//   - Export capabilities (PNG, SVG, JSON)
//
// # Usage
//
// Start the API server:
//
//	graphium server --config configs/config.yaml
//
// Run the Docker agent on a host:
//
//	graphium agent --config configs/config.yaml
//
// Access the Web UI:
//
//	http://localhost:8095
//
// # Configuration
//
// Configuration can be provided via:
//   - YAML file (configs/config.yaml)
//   - Environment variables (CG_ prefix)
//   - .env file
//
// Example configuration:
//
//	server:
//	  host: localhost
//	  port: 8095
//	couchdb:
//	  url: http://localhost:5984
//	  database: graphium
//	  username: admin
//	  password: password
//	agent:
//	  enabled: true
//	  api_url: http://localhost:8095
//	  host_id: host-01
//	  datacenter: us-west-2
//
// # API Endpoints
//
// Container Management:
//   - GET    /api/v1/containers          - List containers (paginated)
//   - GET    /api/v1/containers/:id      - Get container by ID
//   - POST   /api/v1/containers          - Create container
//   - PUT    /api/v1/containers/:id      - Update container
//   - DELETE /api/v1/containers/:id      - Delete container
//   - POST   /api/v1/containers/bulk     - Bulk create containers
//
// Host Management:
//   - GET    /api/v1/hosts               - List hosts (paginated)
//   - GET    /api/v1/hosts/:id           - Get host by ID
//   - POST   /api/v1/hosts               - Create host
//   - PUT    /api/v1/hosts/:id           - Update host
//   - DELETE /api/v1/hosts/:id           - Delete host
//   - POST   /api/v1/hosts/bulk          - Bulk create hosts
//
// Graph Queries:
//   - GET /api/v1/query/containers/by-host/:hostId       - Containers on host
//   - GET /api/v1/query/containers/by-status/:status     - Containers by status
//   - GET /api/v1/query/hosts/by-datacenter/:datacenter  - Hosts in datacenter
//   - GET /api/v1/query/traverse/:id                     - Graph traversal
//   - GET /api/v1/query/dependents/:id                   - Get dependents
//   - GET /api/v1/query/topology/:datacenter             - Datacenter topology
//
// Statistics:
//   - GET /api/v1/stats                      - Overall statistics
//   - GET /api/v1/stats/containers/count     - Container count
//   - GET /api/v1/stats/hosts/count          - Host count
//   - GET /api/v1/stats/distribution         - Container distribution
//
// WebSocket:
//   - GET /api/v1/ws/graph    - Real-time graph updates
//   - GET /api/v1/ws/stats    - WebSocket statistics
//
// # JSON-LD Models
//
// Container (Schema.org SoftwareApplication):
//
//	{
//	  "@context": "https://schema.org",
//	  "@type": "SoftwareApplication",
//	  "@id": "container-abc123",
//	  "name": "web-server",
//	  "executableName": "nginx:latest",
//	  "status": "running",
//	  "hostedOn": "host-01"
//	}
//
// Host (Schema.org ComputerSystem):
//
//	{
//	  "@context": "https://schema.org",
//	  "@type": "ComputerSystem",
//	  "@id": "host-01",
//	  "name": "web-server-01",
//	  "ipAddress": "192.168.1.10",
//	  "status": "active",
//	  "location": "us-west-2"
//	}
//
// # Development
//
// Run tests:
//
//	go test ./...
//
// Run unit tests:
//
//	go test ./internal/api/...
//
// Run integration tests (requires CouchDB):
//
//	go test -v -tags=integration ./tests/integration/...
//
// Build the binary:
//
//	go build -o graphium ./cmd/graphium
//
// # Technology Stack
//
//   - Go 1.23+
//   - Echo v4 (Web framework)
//   - CouchDB 3.3+ (Database)
//   - EVE library (CouchDB client)
//   - Templ (Type-safe templates)
//   - HTMX (Frontend interactivity)
//   - Docker API (Container runtime)
//
// # License
//
// Graphium is open source software.
package graphium
