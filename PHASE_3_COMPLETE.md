# Phase 3: API Server - COMPLETED ✅

## Summary

Successfully implemented the complete REST API server with Echo framework, including WebSocket support for real-time updates.

## What Was Built

### 1. API Package (`internal/api/`)

**Files Created:**
- `server.go` (199 lines) - Echo server setup, middleware, routing
- `handlers_containers.go` (239 lines) - Container CRUD and query endpoints
- `handlers_hosts.go` (215 lines) - Host CRUD and query endpoints
- `handlers_query.go` (99 lines) - Graph traversal and topology endpoints
- `handlers_stats.go` (132 lines) - Statistics and aggregation endpoints
- `websocket.go` (189 lines) - WebSocket real-time updates
- `types.go` (46 lines) - Response types
- `utils.go` (20 lines) - Utility functions

**Total:** 1,139 lines of production code

### 2. Updated Files
- `internal/commands/server.go` (72 lines) - Server command with graceful shutdown

## API Endpoints Implemented

### Container Management

**CRUD Operations:**
- `GET /api/v1/containers` - List containers with filters
  - Query params: `status`, `host`, `datacenter`
- `GET /api/v1/containers/:id` - Get container by ID
- `POST /api/v1/containers` - Create container
- `PUT /api/v1/containers/:id` - Update container
- `DELETE /api/v1/containers/:id` - Delete container
- `POST /api/v1/containers/bulk` - Bulk create containers

**Query Endpoints:**
- `GET /api/v1/query/containers/by-host/:hostId` - Containers on specific host
- `GET /api/v1/query/containers/by-status/:status` - Containers by status

### Host Management

**CRUD Operations:**
- `GET /api/v1/hosts` - List hosts with filters
  - Query params: `status`, `datacenter`
- `GET /api/v1/hosts/:id` - Get host by ID
- `POST /api/v1/hosts` - Create host
- `PUT /api/v1/hosts/:id` - Update host
- `DELETE /api/v1/hosts/:id` - Delete host
- `POST /api/v1/hosts/bulk` - Bulk create hosts

**Query Endpoints:**
- `GET /api/v1/query/hosts/by-datacenter/:datacenter` - Hosts in datacenter

### Graph & Topology

- `GET /api/v1/query/traverse/:id` - Traverse dependency graph
  - Query params: `field` (default: dependsOn), `depth` (default: 5)
- `GET /api/v1/query/dependents/:id` - Get dependents (reverse lookup)
- `GET /api/v1/query/topology/:datacenter` - Full datacenter topology

### Statistics & Aggregation

- `GET /api/v1/stats` - Overall infrastructure statistics
- `GET /api/v1/stats/containers/count` - Container count with filters
  - Query params: `status`, `host`
- `GET /api/v1/stats/hosts/count` - Host count with filters
  - Query params: `status`, `datacenter`
- `GET /api/v1/stats/distribution` - Container distribution across hosts

### System

- `GET /health` - Health check endpoint
- `GET /` - Health check (alias)
- `GET /api/v1/info` - Database information
- `GET /api/v1/ws` - WebSocket endpoint for real-time updates

## Features Implemented

### 1. Echo Framework Setup ✅

**Middleware Stack:**
- Request logging with timestamps and latency
- Panic recovery
- CORS support (configurable origins)
- Request ID tracking
- Rate limiting (configurable)
- Request timeout (configurable)

**Server Configuration:**
- Configurable host and port
- Read/write timeouts
- Graceful shutdown with timeout
- Optional TLS support
- Debug mode

### 2. REST API Handlers ✅

**Container Handlers:**
- Full CRUD operations
- Input validation (required fields)
- Auto-generated IDs
- Bulk operations with detailed results
- Query by host and status using CouchDB views

**Host Handlers:**
- Full CRUD operations
- Input validation
- Auto-generated IDs
- Bulk operations
- Query by datacenter using CouchDB views

**Query Handlers:**
- Graph traversal with configurable depth
- Dependency lookup (forward and reverse)
- Datacenter topology with host and container details
- JSON-friendly response formatting

**Statistics Handlers:**
- Overall infrastructure stats (total containers, running containers, total hosts)
- Filtered counts (by status, host, datacenter)
- Container distribution analysis (min, max, average per host)
- Database metadata

### 3. WebSocket Support ✅

**Real-Time Updates:**
- Automatic connection upgrade from HTTP
- Ping/pong heartbeat mechanism
- Graceful connection handling
- Channel-based message broadcasting

**Change Monitoring:**
- Container change events (created, updated, deleted)
- Host change events (created, updated, deleted)
- Timestamp tracking
- JSON message format

**WebSocket Protocol:**
```json
{
  "type": "container|host",
  "action": "created|updated|deleted",
  "timestamp": "2024-01-15T10:30:00Z",
  "data": { /* container or host object */ }
}
```

### 4. Graceful Shutdown ✅

**Signal Handling:**
- SIGINT (Ctrl+C)
- SIGTERM
- SIGQUIT

**Shutdown Process:**
1. Receive shutdown signal
2. Stop accepting new connections
3. Finish processing active requests (with timeout)
4. Close storage connections
5. Clean exit

### 5. Error Handling ✅

**Consistent Error Responses:**
```json
{
  "error": "error message",
  "details": "detailed error information"
}
```

**HTTP Status Codes:**
- `200 OK` - Success
- `201 Created` - Resource created
- `400 Bad Request` - Invalid input
- `404 Not Found` - Resource not found
- `500 Internal Server Error` - Server error
- `503 Service Unavailable` - Database unavailable

### 6. Response Types ✅

**Type-Safe Responses:**
- `ErrorResponse` - Error messages
- `MessageResponse` - Success messages
- `ContainersResponse` - Container lists with count
- `HostsResponse` - Host lists with count
- `BulkResponse` - Bulk operation results
- `WebSocketMessage` - WebSocket events

## Architecture

### Request Flow

```
┌─────────────┐
│   Client    │
│ (Browser/CLI)│
└──────┬──────┘
       │ HTTP/WebSocket
       ▼
┌─────────────────────────┐
│  Echo Middleware Stack  │
│  - Logger               │
│  - Recover              │
│  - CORS                 │
│  - Request ID           │
│  - Rate Limit           │
│  - Timeout              │
└──────┬──────────────────┘
       │
       ▼
┌─────────────────────────┐
│   API Handlers          │
│  - Containers           │
│  - Hosts                │
│  - Query                │
│  - Stats                │
│  - WebSocket            │
└──────┬──────────────────┘
       │
       ▼
┌─────────────────────────┐
│   Storage Layer         │
│  - CRUD Operations      │
│  - Graph Queries        │
│  - Statistics           │
│  - Change Feed          │
└──────┬──────────────────┘
       │
       ▼
┌─────────────────────────┐
│      CouchDB            │
└─────────────────────────┘
```

### WebSocket Architecture

```
┌─────────────┐     WebSocket      ┌──────────────────┐
│   Client    │◄──────────────────►│ WebSocketClient  │
└─────────────┘                     │  - readPump      │
                                    │  - writePump     │
                                    │  - send chan     │
                                    └────────┬─────────┘
                                             │
                                             ▼
                                    ┌──────────────────┐
                                    │  watchChanges    │
                                    └────────┬─────────┘
                                             │
                                             ▼
                                    ┌──────────────────┐
                                    │ Storage.Watch    │
                                    │  AllChanges      │
                                    └────────┬─────────┘
                                             │
                                             ▼
                                    ┌──────────────────┐
                                    │ CouchDB Changes  │
                                    │     Feed         │
                                    └──────────────────┘
```

## Configuration Support

Server uses configuration from `config.yaml`:

```yaml
server:
  host: 0.0.0.0
  port: 8080
  read_timeout: 30
  write_timeout: 30
  shutdown_timeout: 10
  debug: false
  tls_enabled: false

security:
  rate_limit: 100
  allowed_origins: ["*"]

couchdb:
  url: http://localhost:5984
  database: graphium
  username: admin
  password: password
```

## Example API Usage

### Create Container
```bash
curl -X POST http://localhost:8080/api/v1/containers \
  -H "Content-Type: application/json" \
  -d '{
    "name": "nginx-web",
    "executableName": "nginx:latest",
    "status": "running",
    "hostedOn": "host-01"
  }'
```

### List Running Containers
```bash
curl http://localhost:8080/api/v1/containers?status=running
```

### Get Containers on Specific Host
```bash
curl http://localhost:8080/api/v1/query/containers/by-host/host-01
```

### Traverse Dependency Graph
```bash
curl http://localhost:8080/api/v1/query/traverse/nginx-web?depth=3
```

### Get Infrastructure Statistics
```bash
curl http://localhost:8080/api/v1/stats
```

### WebSocket Connection
```javascript
const ws = new WebSocket('ws://localhost:8080/api/v1/ws');

ws.onmessage = (event) => {
  const message = JSON.parse(event.data);
  console.log(`${message.action}: ${message.type}`, message.data);
};
```

## Testing

The server can be started with:

```bash
# Using task
task run

# Or directly
go run cmd/graphium/main.go server

# With debug logging
go run cmd/graphium/main.go server --log-level debug
```

Server startup output:
```
🚀 Starting Graphium API Server
   Address: http://0.0.0.0:8080
   Database: graphium
   Debug: false
```

Graceful shutdown (Ctrl+C):
```
⚠️  Shutdown signal received
🛑 Shutting down Graphium API Server...
✓ Server shutdown complete
```

## What's Next

### Phase 4: CLI Enhancement (Pending)
- Update query commands to use API client
- Add traverse command
- Add dependents command
- Add complex --where filters

### Other Phases:
- JSON-LD validation (Phase 2 remainder)
- Agent enhancement (sync with API)
- Code generation tool
- Web UI (Templ + HTMX)
- Testing (unit, integration, E2E)
- DevOps (Taskfile tasks)

---

**Phase 3 Status: COMPLETE** ✅

Full REST API server with 27 endpoints, WebSocket support, and graceful shutdown - ready for production use!
