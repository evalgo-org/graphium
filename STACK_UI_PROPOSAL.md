# Stack Management UI Proposal for Graphium

## Executive Summary

Add **EVE Stack Orchestration** to Graphium's UI, enabling visual management of multi-container deployments defined using schema.org ItemList structure. This proposal integrates seamlessly with Graphium's existing Templ+HTMX architecture and JSON-LD semantic data model.

**Estimated Effort**: 2-3 days for core features, +1 day for advanced features

---

## 1. Architecture Overview

### Stack Model (schema.org Compliant)

Graphium will manage **EVE Stacks** (schema.org ItemList) containing multiple containers (SoftwareApplication) with dependencies and orchestration.

```go
// models/stack.go
type Stack struct {
    Context     string                `json:"@context"`     // "https://schema.org"
    Type        string                `json:"@type"`        // "ItemList"
    ID          string                `json:"@id" couchdb:"_id"`
    Rev         string                `json:"_rev,omitempty"`

    Name        string                `json:"name"`
    Description string                `json:"description,omitempty"`
    Status      string                `json:"status"`       // pending, deploying, running, stopped, error

    // Stack definition (from EVE stacks)
    ItemListElement []StackItemElement `json:"itemListElement"`
    Network         NetworkConfig      `json:"network,omitempty"`
    Volumes         []VolumeConfig     `json:"volumes,omitempty"`

    // Deployment metadata
    Datacenter   string                `json:"location" couchdb:"index"`
    HostID       string                `json:"hostedOn" couchdb:"index"` // Primary deployment host
    DeploymentID string                `json:"deploymentId,omitempty"`   // EVE deployment ID

    // Timestamps
    CreatedAt    time.Time             `json:"dateCreated"`
    UpdatedAt    time.Time             `json:"dateModified"`
    DeployedAt   *time.Time            `json:"deployedAt,omitempty"`

    // Ownership
    Owner        string                `json:"owner,omitempty"` // User who created
    Labels       map[string]string     `json:"labels,omitempty"`
}

// Reuse EVE's StackItemElement, NetworkConfig, VolumeConfig structures
type StackItemElement = stacks.StackItemElement
type NetworkConfig = stacks.NetworkConfig
type VolumeConfig = stacks.VolumeConfig
```

### Integration with EVE

Graphium will use EVE's stack orchestration directly:

```go
// Deploy stack using EVE
import "eve.evalgo.org/containers/stacks/production"

func (s *Service) DeployStack(stack *models.Stack) error {
    ctx, cli := common.CtxCli("unix:///var/run/docker.sock")
    defer cli.Close()

    // Convert Graphium Stack to EVE Stack
    eveStack := convertToEVEStack(stack)

    // Deploy using EVE
    deployment, err := production.DeployStack(ctx, cli, eveStack)
    if err != nil {
        return err
    }

    // Update Graphium stack with deployment info
    stack.DeploymentID = deployment.Stack.Name
    stack.Status = "running"
    stack.DeployedAt = &deployment.StartTime

    return nil
}
```

---

## 2. New UI Pages

### 2.1 Stacks List Page

**Route**: `/web/stacks`
**Template**: `StacksListWithUser()`

**Features**:
- Paginated table (10 stacks per page)
- HTMX live search by name
- Filter by status (pending, deploying, running, stopped, error)
- Filter by datacenter
- Quick actions: Deploy, View Details, Stop, Remove

**Mockup**:
```
┌─────────────────────────────────────────────────────────────┐
│ Stacks                                      [Deploy Stack] │
├─────────────────────────────────────────────────────────────┤
│ [Search...] [Status: All ▼] [Datacenter: All ▼]           │
├─────────────────────────────────────────────────────────────┤
│ Name            Status     Services  Datacenter   Actions   │
├─────────────────────────────────────────────────────────────┤
│ ● dev-env       running    3/3       us-west-2   [View] [] │
│ ● infisical     running    3/3       us-east-1   [View] [] │
│ ○ test-stack    stopped    0/2       eu-central  [View] [] │
│ ⚠ app-stack     error      1/4       us-west-2   [View] [] │
├─────────────────────────────────────────────────────────────┤
│ Showing 1-10 of 45                    < 1 2 3 4 5 >        │
└─────────────────────────────────────────────────────────────┘
```

**Status Badges**:
- 🟢 **running** (green) - All services healthy
- 🟡 **deploying** (yellow) - Deployment in progress
- ⚪ **stopped** (gray) - Stack stopped
- 🔴 **error** (red) - One or more services failed
- 🔵 **pending** (blue) - Not yet deployed

**HTMX Interactions**:
```html
<input type="text" name="search" placeholder="Search stacks..."
    hx-get="/web/stacks/table"
    hx-target="#stacks-table"
    hx-trigger="keyup changed delay:300ms"
/>
```

### 2.2 Stack Details Page

**Route**: `/web/stacks/:id`
**Template**: `StackDetailWithUser()`

**Features**:
- Stack overview (name, status, datacenter, owner)
- Services table with status, health checks, ports
- Dependency graph visualization
- Network and volume information
- Deployment timeline
- Actions: Redeploy, Stop, Remove, Edit

**Mockup**:
```
┌─────────────────────────────────────────────────────────────┐
│ ← Back to Stacks                                            │
├─────────────────────────────────────────────────────────────┤
│ infisical                                      🟢 running   │
│ Complete secrets management platform                        │
├─────────────────────────────────────────────────────────────┤
│ OVERVIEW                                                    │
│ Datacenter:   us-east-1                                     │
│ Host:         prod-host-01 →                                │
│ Network:      infisical-network                             │
│ Deployed:     2025-10-29 17:30:15                           │
│ Owner:        admin                                         │
├─────────────────────────────────────────────────────────────┤
│ SERVICES (3/3 running)                                      │
│                                                             │
│ ┌─────────────────────────────────────────────────────┐   │
│ │ 1. postgres                            🟢 healthy   │   │
│ │    Image: postgres:17                               │   │
│ │    Ports: 5432                                      │   │
│ │    Health: pg_isready ✓                             │   │
│ │    [View Container] [Logs]                          │   │
│ └─────────────────────────────────────────────────────┘   │
│                                                             │
│ ┌─────────────────────────────────────────────────────┐   │
│ │ 2. dragonflydb                         🟢 healthy   │   │
│ │    Image: dragonflydb/dragonfly:v1.26.1             │   │
│ │    Ports: 6379                                      │   │
│ │    Health: redis-cli ping ✓                         │   │
│ │    Depends on: postgres (waiting for healthy)       │   │
│ │    [View Container] [Logs]                          │   │
│ └─────────────────────────────────────────────────────┘   │
│                                                             │
│ ┌─────────────────────────────────────────────────────┐   │
│ │ 3. infisical                           🟢 healthy   │   │
│ │    Image: infisical/infisical:v0.153.0              │   │
│ │    Ports: 8080 → 80                                 │   │
│ │    Health: HTTP GET /healthz ✓                      │   │
│ │    Depends on: postgres, dragonflydb                │   │
│ │    Post-start: Run migrations ✓                     │   │
│ │    [View Container] [Logs]                          │   │
│ └─────────────────────────────────────────────────────┘   │
├─────────────────────────────────────────────────────────────┤
│ DEPENDENCY GRAPH                                            │
│ [Show Visual Graph]                                         │
│                                                             │
│    postgres ──→ dragonflydb ──→ infisical                  │
│       ↓              ↓                                      │
│    healthy       healthy                                    │
├─────────────────────────────────────────────────────────────┤
│ VOLUMES                                                     │
│ • infisical-postgres-data → /var/lib/postgresql/data       │
│ • infisical-cache → /data                                   │
├─────────────────────────────────────────────────────────────┤
│ ACTIONS                                                     │
│ [Stop Stack] [Restart Stack] [Remove Stack] [Edit Stack]   │
└─────────────────────────────────────────────────────────────┘
```

**Service Status Indicators**:
- 🟢 **healthy** - Health check passing
- 🟡 **starting** - Container starting, health check pending
- 🔴 **unhealthy** - Health check failing
- ⚪ **stopped** - Container not running

### 2.3 Deploy Stack Form

**Route**: `/web/stacks/new`
**Template**: `DeployStackFormWithUser()`

**Features**:
- Stack name input
- Datacenter selector
- Host selector (optional - auto-select if not specified)
- Stack definition input (3 modes):
  1. **Upload JSON** - Upload schema.org ItemList JSON file
  2. **Select Template** - Choose from predefined templates (dev-env, infisical, etc.)
  3. **JSON Editor** - Direct JSON input with validation

**Mockup**:
```
┌─────────────────────────────────────────────────────────────┐
│ Deploy New Stack                                            │
├─────────────────────────────────────────────────────────────┤
│ BASIC INFORMATION                                           │
│                                                             │
│ Stack Name: [_________________________]                     │
│ Description: [___________________________________________]   │
│                                                             │
│ Datacenter: [us-west-2 ▼]                                   │
│ Host: [Auto-select ▼]  (Optional)                          │
├─────────────────────────────────────────────────────────────┤
│ STACK DEFINITION                                            │
│                                                             │
│ ○ Upload JSON File   ● Select Template   ○ JSON Editor     │
│                                                             │
│ ┌─────────────────────────────────────────────────────┐   │
│ │ Select Template:                                    │   │
│ │                                                     │   │
│ │ ○ Graphium Dev Environment (CouchDB)                │   │
│ │ ● Infisical (PostgreSQL + DragonflyDB + Infisical)  │   │
│ │ ○ Custom LAMP Stack (MySQL + Apache + PHP)          │   │
│ │ ○ Observability Stack (Grafana + Mimir + Fluent)    │   │
│ │ ○ Empty Template                                    │   │
│ │                                                     │   │
│ │ [Preview Template →]                                 │   │
│ └─────────────────────────────────────────────────────┘   │
│                                                             │
│ Preview:                                                    │
│ ┌─────────────────────────────────────────────────────┐   │
│ │ {                                                   │   │
│ │   "@context": "https://schema.org",                 │   │
│ │   "@type": "ItemList",                              │   │
│ │   "name": "Infisical Stack",                        │   │
│ │   "itemListElement": [                              │   │
│ │     { ... postgres ... },                           │   │
│ │     { ... dragonflydb ... },                        │   │
│ │     { ... infisical ... }                           │   │
│ │   ]                                                 │   │
│ │ }                                                   │   │
│ └─────────────────────────────────────────────────────┘   │
├─────────────────────────────────────────────────────────────┤
│ [Cancel] [Validate] [Deploy Stack]                         │
└─────────────────────────────────────────────────────────────┘
```

**Validation**:
- HTMX real-time JSON validation
- Endpoint: `POST /api/v1/stacks/validate`
- Shows errors: Invalid JSON, missing required fields, circular dependencies

### 2.4 Stack Logs Page

**Route**: `/web/stacks/:id/logs`
**Template**: `StackLogsWithUser()`

**Features**:
- Aggregated logs from all services
- Service selector (all, specific service)
- HTMX polling for live updates (5 second interval)
- Log level filter (all, error, warn, info)
- Tail lines selector (50, 100, 500, 1000)

**Mockup**:
```
┌─────────────────────────────────────────────────────────────┐
│ ← infisical Logs                                            │
├─────────────────────────────────────────────────────────────┤
│ Service: [All ▼] | Level: [All ▼] | Tail: [100 ▼] [Refresh]│
├─────────────────────────────────────────────────────────────┤
│ [postgres] 2025-10-29 17:30:15 INFO database ready          │
│ [dragonflydb] 2025-10-29 17:30:17 INFO server listening     │
│ [infisical] 2025-10-29 17:30:20 INFO running migrations     │
│ [infisical] 2025-10-29 17:30:22 INFO migrations complete    │
│ [infisical] 2025-10-29 17:30:23 INFO server started :8080   │
│ [postgres] 2025-10-29 17:30:25 INFO checkpoint complete     │
│ ...                                                         │
└─────────────────────────────────────────────────────────────┘
```

---

## 3. Navigation Integration

### Update Main Navigation

Add "Stacks" to the main navigation menu (between Containers and Hosts):

```templ
// internal/web/templates.templ - Navigation component
templ Navigation() {
    <nav>
        <a href="/">Dashboard</a>
        <a href="/web/containers">Containers</a>
        <a href="/web/stacks">Stacks</a>          <!-- NEW -->
        <a href="/web/hosts">Hosts</a>
        <a href="/web/topology">Topology</a>
        <a href="/web/graph">Graph</a>
    </nav>
}
```

### Update Dashboard

Add Stack statistics to dashboard overview:

```
┌────────────────────────────────────────┐
│ Infrastructure Overview                │
├────────────────────────────────────────┤
│ Hosts              42                  │
│ Containers         156                 │
│ Stacks             8    ← NEW          │
│ Running Stacks     6    ← NEW          │
└────────────────────────────────────────┘
```

---

## 4. API Endpoints

### New REST API Routes

```go
// internal/api/server.go - setupRoutes()

// Stack management routes
stacks := v1.Group("/stacks")
stacks.GET("", s.listStacks, webHandler.WebAuthMiddleware)
stacks.GET("/:id", s.getStack, ValidateIDFormat, webHandler.WebAuthMiddleware)
stacks.POST("", s.deployStack, s.authMiddle.RequireWrite)
stacks.PUT("/:id", s.updateStack, ValidateIDFormat, s.authMiddle.RequireWrite)
stacks.DELETE("/:id", s.removeStack, ValidateIDFormat, s.authMiddle.RequireWrite)
stacks.POST("/:id/stop", s.stopStack, ValidateIDFormat, s.authMiddle.RequireWrite)
stacks.POST("/:id/start", s.startStack, ValidateIDFormat, s.authMiddle.RequireWrite)
stacks.POST("/:id/restart", s.restartStack, ValidateIDFormat, s.authMiddle.RequireWrite)
stacks.GET("/:id/services", s.getStackServices, ValidateIDFormat, webHandler.WebAuthMiddleware)
stacks.GET("/:id/logs", s.getStackLogs, ValidateIDFormat, webHandler.WebAuthMiddleware)
stacks.POST("/validate", s.validateStackDefinition, s.authMiddle.RequireWrite)

// Stack templates
stacks.GET("/templates", s.listStackTemplates, webHandler.WebAuthMiddleware)
stacks.GET("/templates/:name", s.getStackTemplate, webHandler.WebAuthMiddleware)
```

### Handler Implementations

**File**: `internal/api/handlers_stacks.go` (new file, ~500 lines)

Key handler logic:

```go
func (s *Server) deployStack(c echo.Context) error {
    var req DeployStackRequest
    if err := c.Bind(&req); err != nil {
        return echo.NewHTTPError(http.StatusBadRequest, err.Error())
    }

    // Parse stack definition (schema.org ItemList JSON)
    eveStack, err := stacks.LoadStackFromJSON([]byte(req.Definition))
    if err != nil {
        return echo.NewHTTPError(http.StatusBadRequest, "Invalid stack definition")
    }

    // Create Graphium stack record
    stack := &models.Stack{
        Context:     "https://schema.org",
        Type:        "ItemList",
        ID:          generateID("stack"),
        Name:        req.Name,
        Description: req.Description,
        Status:      "deploying",
        Datacenter:  req.Datacenter,
        HostID:      req.HostID,
        Owner:       getUserID(c),
        CreatedAt:   time.Now(),
    }

    // Save to database
    if err := s.storage.SaveStack(stack); err != nil {
        return err
    }

    // Deploy using EVE (async)
    go s.orchestrator.DeployStack(stack, eveStack)

    return c.JSON(http.StatusAccepted, stack)
}
```

---

## 5. Web Handler Routes

### Add Web Routes

**File**: `internal/web/server.go`

```go
func setupWebRoutes(e *echo.Echo, h *Handler) {
    // ... existing routes ...

    // Stack web routes
    web.GET("/stacks", h.StacksList, h.WebAuthMiddleware)
    web.GET("/stacks/new", h.DeployStackForm, h.WebAuthMiddleware)
    web.GET("/stacks/table", h.StacksTable, h.WebAuthMiddleware) // HTMX partial
    web.GET("/stacks/:id", h.StackDetail, h.WebAuthMiddleware)
    web.GET("/stacks/:id/logs", h.StackLogs, h.WebAuthMiddleware)
    web.POST("/stacks/:id/stop", h.StopStack, h.WebAuthMiddleware)
    web.POST("/stacks/:id/start", h.StartStack, h.WebAuthMiddleware)
    web.POST("/stacks/:id/remove", h.RemoveStack, h.WebAuthMiddleware)
}
```

### Handler Implementations

**File**: `internal/web/handlers_stacks.go` (new file, ~400 lines)

```go
func (h *Handler) StacksList(c echo.Context) error {
    user := getUserFromContext(c)

    // Get stacks from API
    stacks, pagination, err := h.fetchStacks(c)
    if err != nil {
        return err
    }

    // Render template
    return Render(c, StacksListWithUser(stacks, pagination, user))
}

func (h *Handler) StackDetail(c echo.Context) error {
    stackID := c.Param("id")
    user := getUserFromContext(c)

    // Fetch stack and its services
    stack, services, err := h.fetchStackDetail(stackID)
    if err != nil {
        return err
    }

    return Render(c, StackDetailWithUser(stack, services, user))
}

func (h *Handler) DeployStackForm(c echo.Context) error {
    user := getUserFromContext(c)
    templates := h.getStackTemplates()

    return Render(c, DeployStackFormWithUser(templates, user))
}
```

---

## 6. Storage Layer

### New Storage Methods

**File**: `internal/storage/stacks.go` (new file, ~300 lines)

```go
package storage

import (
    "evalgo.org/graphium/models"
    "eve.evalgo.org/db"
)

func (s *Storage) SaveStack(stack *models.Stack) error {
    stack.Context = "https://schema.org"
    stack.Type = "ItemList"
    return s.service.CreateDocument(stack, stack.ID)
}

func (s *Storage) GetStack(id string) (*models.Stack, error) {
    var stack models.Stack
    err := s.service.GetDocument(id, &stack)
    return &stack, err
}

func (s *Storage) ListStacks(filters map[string]interface{}) ([]*models.Stack, error) {
    // Add @type filter for ItemList
    filters["@type"] = "ItemList"

    docs, err := s.service.QueryDocuments(filters)
    if err != nil {
        return nil, err
    }

    stacks := make([]*models.Stack, len(docs))
    for i, doc := range docs {
        var stack models.Stack
        if err := mapToStruct(doc, &stack); err != nil {
            continue
        }
        stacks[i] = &stack
    }

    return stacks, nil
}

func (s *Storage) DeleteStack(id, rev string) error {
    return s.service.DeleteDocument(id, rev)
}

func (s *Storage) GetStacksByDatacenter(datacenter string) ([]*models.Stack, error) {
    return s.ListStacks(map[string]interface{}{
        "location": datacenter,
    })
}

func (s *Storage) GetStacksByHost(hostID string) ([]*models.Stack, error) {
    return s.ListStacks(map[string]interface{}{
        "hostedOn": hostID,
    })
}
```

---

## 7. Orchestration Service

### Stack Orchestrator

**File**: `internal/orchestrator/stacks.go` (new file, ~400 lines)

Handles actual deployment using EVE:

```go
package orchestrator

import (
    "context"
    "evalgo.org/graphium/models"
    "evalgo.org/graphium/internal/storage"
    "eve.evalgo.org/common"
    "eve.evalgo.org/containers/stacks"
    "eve.evalgo.org/containers/stacks/production"
)

type StackOrchestrator struct {
    storage *storage.Storage
}

func (o *StackOrchestrator) DeployStack(stack *models.Stack, definition *stacks.Stack) error {
    ctx, cli := common.CtxCli("unix:///var/run/docker.sock")
    defer cli.Close()

    // Update status
    stack.Status = "deploying"
    o.storage.SaveStack(stack)

    // Deploy using EVE
    deployment, err := production.DeployStack(ctx, cli, definition)
    if err != nil {
        stack.Status = "error"
        o.storage.SaveStack(stack)
        return err
    }

    // Update stack with deployment info
    stack.Status = "running"
    stack.DeploymentID = deployment.Stack.Name
    now := time.Now()
    stack.DeployedAt = &now

    // Link containers to stack
    for serviceName, containerID := range deployment.Containers {
        // Update container with stack reference
        container, _ := o.storage.GetContainer(containerID)
        if container != nil {
            if container.Labels == nil {
                container.Labels = make(map[string]string)
            }
            container.Labels["stack"] = stack.ID
            container.Labels["stack-service"] = serviceName
            o.storage.SaveContainer(container)
        }
    }

    return o.storage.SaveStack(stack)
}

func (o *StackOrchestrator) StopStack(stackID string) error {
    stack, err := o.storage.GetStack(stackID)
    if err != nil {
        return err
    }

    ctx, cli := common.CtxCli("unix:///var/run/docker.sock")
    defer cli.Close()

    if err := production.StopStack(ctx, cli, stack.DeploymentID); err != nil {
        return err
    }

    stack.Status = "stopped"
    return o.storage.SaveStack(stack)
}

func (o *StackOrchestrator) RemoveStack(stackID string) error {
    stack, err := o.storage.GetStack(stackID)
    if err != nil {
        return err
    }

    ctx, cli := common.CtxCli("unix:///var/run/docker.sock")
    defer cli.Close()

    // Remove stack (with volumes = false by default)
    if err := production.RemoveStack(ctx, cli, stack.DeploymentID, false); err != nil {
        return err
    }

    // Delete from database
    return o.storage.DeleteStack(stack.ID, stack.Rev)
}
```

---

## 8. Templates & Styling

### Templ Components

**File**: `internal/web/templates.templ` (additions, ~600 lines)

#### 8.1 Stacks List Template

```templ
templ StacksListWithUser(stacks []*models.Stack, pagination PaginationInfo, user *models.User) {
    @LayoutWithUser("Stacks", user) {
        <div class="page-header">
            <h2>Container Stacks</h2>
            <a href="/web/stacks/new" class="btn btn-primary">Deploy Stack</a>
        </div>

        <div class="filters">
            <input type="text" name="search" placeholder="Search stacks..."
                hx-get="/web/stacks/table"
                hx-target="#stacks-table"
                hx-trigger="keyup changed delay:300ms"
                hx-include="[name='status'], [name='datacenter']"
            />
            <select name="status"
                hx-get="/web/stacks/table"
                hx-target="#stacks-table"
                hx-trigger="change"
                hx-include="[name='search'], [name='datacenter']"
            >
                <option value="">All Statuses</option>
                <option value="running">Running</option>
                <option value="stopped">Stopped</option>
                <option value="deploying">Deploying</option>
                <option value="error">Error</option>
            </select>
            <select name="datacenter"
                hx-get="/web/stacks/table"
                hx-target="#stacks-table"
                hx-trigger="change"
                hx-include="[name='search'], [name='status']"
            >
                <option value="">All Datacenters</option>
                <option value="us-west-2">US West 2</option>
                <option value="us-east-1">US East 1</option>
                <option value="eu-central-1">EU Central 1</option>
            </select>
        </div>

        <div id="stacks-table">
            @StacksTable(stacks, pagination)
        </div>
    }
}

templ StacksTable(stacks []*models.Stack, pagination PaginationInfo) {
    <table class="data-table">
        <thead>
            <tr>
                <th>Name</th>
                <th>Status</th>
                <th>Services</th>
                <th>Datacenter</th>
                <th>Deployed</th>
                <th>Actions</th>
            </tr>
        </thead>
        <tbody>
            for _, stack := range stacks {
                <tr>
                    <td>
                        <a href={templ.URL("/web/stacks/" + stack.ID)}>
                            {stack.Name}
                        </a>
                        if stack.Description != "" {
                            <div class="text-muted">{stack.Description}</div>
                        }
                    </td>
                    <td>@StackStatusBadge(stack.Status)</td>
                    <td>{fmt.Sprintf("%d", len(stack.ItemListElement))}</td>
                    <td>{stack.Datacenter}</td>
                    <td>
                        if stack.DeployedAt != nil {
                            {stack.DeployedAt.Format("2006-01-02 15:04")}
                        } else {
                            <span class="text-muted">Not deployed</span>
                        }
                    </td>
                    <td class="actions">
                        <a href={templ.URL("/web/stacks/" + stack.ID)}>View</a>
                        <a href={templ.URL("/web/stacks/" + stack.ID + "/logs")}>Logs</a>
                        if stack.Status == "running" {
                            <button hx-post={"/web/stacks/" + stack.ID + "/stop"}>Stop</button>
                        } else if stack.Status == "stopped" {
                            <button hx-post={"/web/stacks/" + stack.ID + "/start"}>Start</button>
                        }
                    </td>
                </tr>
            }
        </tbody>
    </table>

    @Pagination(pagination)
}

templ StackStatusBadge(status string) {
    <span class={"badge badge-" + status}>
        switch status {
            case "running":
                🟢 Running
            case "stopped":
                ⚪ Stopped
            case "deploying":
                🟡 Deploying
            case "error":
                🔴 Error
            default:
                🔵 {status}
        }
    </span>
}
```

#### 8.2 Stack Detail Template

```templ
templ StackDetailWithUser(stack *models.Stack, services []ServiceInfo, user *models.User) {
    @LayoutWithUser(stack.Name, user) {
        <div class="page-header">
            <a href="/web/stacks" class="back-link">← Back to Stacks</a>
            <div class="header-title">
                <h2>{stack.Name}</h2>
                @StackStatusBadge(stack.Status)
            </div>
        </div>

        <div class="detail-grid">
            <div class="detail-card">
                <h3>Overview</h3>
                <dl>
                    <dt>Description</dt>
                    <dd>{stack.Description}</dd>

                    <dt>Datacenter</dt>
                    <dd>{stack.Datacenter}</dd>

                    <dt>Host</dt>
                    <dd>
                        if stack.HostID != "" {
                            <a href={templ.URL("/web/hosts/" + stack.HostID)}>{stack.HostID}</a>
                        } else {
                            Auto-selected
                        }
                    </dd>

                    <dt>Network</dt>
                    <dd>{stack.Network.Name}</dd>

                    <dt>Deployed</dt>
                    <dd>
                        if stack.DeployedAt != nil {
                            {stack.DeployedAt.Format("2006-01-02 15:04:05")}
                        } else {
                            Not deployed
                        }
                    </dd>

                    <dt>Owner</dt>
                    <dd>{stack.Owner}</dd>
                </dl>
            </div>

            <div class="detail-card">
                <h3>Services ({fmt.Sprintf("%d", len(services))})</h3>

                for i, service := range services {
                    <div class="service-card">
                        <div class="service-header">
                            <span class="service-position">{fmt.Sprintf("%d", i+1)}</span>
                            <span class="service-name">{service.Name}</span>
                            @ServiceHealthBadge(service.Health)
                        </div>

                        <div class="service-details">
                            <div class="detail-row">
                                <strong>Image:</strong> {service.Image}
                            </div>

                            if len(service.Ports) > 0 {
                                <div class="detail-row">
                                    <strong>Ports:</strong>
                                    for _, port := range service.Ports {
                                        <span class="port-badge">
                                            {fmt.Sprintf("%d:%d/%s", port.HostPort, port.ContainerPort, port.Protocol)}
                                        </span>
                                    }
                                </div>
                            }

                            if len(service.Dependencies) > 0 {
                                <div class="detail-row">
                                    <strong>Depends on:</strong>
                                    for _, dep := range service.Dependencies {
                                        <span class="dependency-badge">{dep}</span>
                                    }
                                </div>
                            }

                            if service.HealthCheck != "" {
                                <div class="detail-row">
                                    <strong>Health:</strong> {service.HealthCheck}
                                </div>
                            }
                        </div>

                        <div class="service-actions">
                            <a href={templ.URL("/web/containers/" + service.ContainerID)}>View Container</a>
                            <a href={templ.URL("/web/stacks/" + stack.ID + "/logs?service=" + service.Name)}>Logs</a>
                        </div>
                    </div>
                }
            </div>

            if len(stack.Volumes) > 0 {
                <div class="detail-card">
                    <h3>Volumes</h3>
                    <ul class="volume-list">
                        for _, vol := range stack.Volumes {
                            <li>
                                <strong>{vol.Name}</strong>
                                <span class="text-muted">{vol.Driver}</span>
                            </li>
                        }
                    </ul>
                </div>
            }

            <div class="detail-card actions-card">
                <h3>Actions</h3>
                <div class="action-buttons">
                    if stack.Status == "running" {
                        <button hx-post={"/web/stacks/" + stack.ID + "/stop"}
                                hx-confirm="Stop this stack?">
                            Stop Stack
                        </button>
                        <button hx-post={"/web/stacks/" + stack.ID + "/restart"}>
                            Restart Stack
                        </button>
                    } else if stack.Status == "stopped" {
                        <button hx-post={"/web/stacks/" + stack.ID + "/start"}>
                            Start Stack
                        </button>
                    }

                    <a href={templ.URL("/web/stacks/" + stack.ID + "/logs")} class="btn">
                        View Logs
                    </a>

                    <button hx-post={"/web/stacks/" + stack.ID + "/remove"}
                            hx-confirm="Remove this stack and all containers?"
                            class="btn-danger">
                        Remove Stack
                    </button>
                </div>
            </div>
        </div>
    }
}

templ ServiceHealthBadge(health string) {
    <span class={"health-badge health-" + health}>
        switch health {
            case "healthy":
                🟢 Healthy
            case "unhealthy":
                🔴 Unhealthy
            case "starting":
                🟡 Starting
            default:
                ⚪ Unknown
        }
    </span>
}
```

### CSS Additions

**File**: `static/css/styles.css` (additions, ~200 lines)

```css
/* Stack-specific styles */

.service-card {
    background: var(--surface-color);
    border: 1px solid var(--border-color);
    border-radius: 8px;
    padding: 16px;
    margin-bottom: 12px;
}

.service-header {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 12px;
}

.service-position {
    background: var(--primary-color);
    color: white;
    width: 28px;
    height: 28px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    font-weight: bold;
    font-size: 14px;
}

.service-name {
    font-size: 18px;
    font-weight: 600;
    flex: 1;
}

.health-badge {
    padding: 4px 12px;
    border-radius: 12px;
    font-size: 13px;
    font-weight: 500;
}

.health-healthy { background: #22c55e; color: white; }
.health-unhealthy { background: #ef4444; color: white; }
.health-starting { background: #f59e0b; color: white; }

.service-details {
    margin-top: 12px;
    font-size: 14px;
}

.detail-row {
    margin-bottom: 8px;
}

.port-badge, .dependency-badge {
    display: inline-block;
    background: var(--secondary-bg);
    padding: 2px 8px;
    border-radius: 4px;
    margin-right: 6px;
    font-size: 13px;
}

.service-actions {
    margin-top: 12px;
    padding-top: 12px;
    border-top: 1px solid var(--border-color);
    display: flex;
    gap: 8px;
}

.badge-deploying { background: #f59e0b; }
.badge-running { background: #22c55e; }
.badge-stopped { background: #6b7280; }
.badge-error { background: #ef4444; }

/* Deploy form */
.stack-template-card {
    border: 2px solid var(--border-color);
    border-radius: 8px;
    padding: 16px;
    cursor: pointer;
    transition: all 0.2s;
}

.stack-template-card:hover {
    border-color: var(--primary-color);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.stack-template-card.selected {
    border-color: var(--primary-color);
    background: var(--primary-bg-light);
}

.json-preview {
    background: var(--code-bg);
    border: 1px solid var(--border-color);
    border-radius: 4px;
    padding: 12px;
    font-family: 'Monaco', 'Menlo', 'Courier New', monospace;
    font-size: 13px;
    max-height: 400px;
    overflow-y: auto;
}
```

---

## 9. Implementation Phases

### Phase 1: Core Features (Day 1-2)

**Priority: High**

1. ✅ Create Stack model (`models/stack.go`)
2. ✅ Add storage layer (`internal/storage/stacks.go`)
3. ✅ Create orchestrator service (`internal/orchestrator/stacks.go`)
4. ✅ Add API endpoints (`internal/api/handlers_stacks.go`)
5. ✅ Implement Stacks List page
6. ✅ Implement Stack Detail page
7. ✅ Add navigation integration
8. ✅ Basic CSS styling

**Deliverables**:
- View existing stacks
- Deploy simple stacks
- Stop/start stacks
- View stack details and services

### Phase 2: Advanced Features (Day 3)

**Priority: Medium**

1. Deploy Stack form with templates
2. Stack logs aggregation
3. Stack validation endpoint
4. Template library (5-10 common stacks)
5. Enhanced error handling
6. Real-time status updates (WebSocket)

**Deliverables**:
- User-friendly stack deployment
- Pre-built templates
- Live log viewing
- Real-time status

### Phase 3: Integration & Polish (Optional)

**Priority: Low**

1. Integration with graph visualization
2. Stack metrics/monitoring
3. Stack scaling controls
4. Bulk operations
5. Export/import stacks
6. Stack versioning

---

## 10. Testing Strategy

### Unit Tests

```go
// models/stack_test.go
func TestStackValidation(t *testing.T) { ... }

// internal/storage/stacks_test.go
func TestSaveStack(t *testing.T) { ... }
func TestListStacks(t *testing.T) { ... }

// internal/orchestrator/stacks_test.go (with mock Docker)
func TestDeployStack(t *testing.T) { ... }
```

### Integration Tests

```go
// tests/integration/stacks_test.go
func TestStackDeploymentFlow(t *testing.T) {
    // 1. Create stack via API
    // 2. Verify deployment using EVE
    // 3. Check containers exist
    // 4. Stop stack
    // 5. Verify containers stopped
    // 6. Remove stack
}
```

### Manual Testing Checklist

- [ ] Deploy stack from template
- [ ] Deploy stack from custom JSON
- [ ] View stacks list with filters
- [ ] View stack details with all services
- [ ] View aggregated logs
- [ ] Stop running stack
- [ ] Start stopped stack
- [ ] Remove stack (confirm containers deleted)
- [ ] Test with stacks containing dependencies
- [ ] Test with stacks containing health checks
- [ ] Test error handling (invalid JSON, failed deployment)

---

## 11. Benefits Summary

### For Developers
- 📦 **Visual Stack Management** - No more manual docker-compose commands
- 🔍 **Unified View** - See all stacks, services, and dependencies in one place
- 🚀 **One-Click Deployment** - Deploy complex multi-container stacks instantly
- 📊 **Real-Time Status** - Monitor health checks and service status

### For Operations
- 🎯 **Template Library** - Standardized deployment patterns
- 🔄 **Version Control** - Track stack changes over time
- 📈 **Centralized Management** - Manage stacks across datacenters
- 🛡️ **Access Control** - Role-based permissions for stack operations

### Technical Benefits
- ✅ **EVE Integration** - Leverages existing container orchestration
- ✅ **Semantic Data** - schema.org compliant stack definitions
- ✅ **Consistent UX** - Matches existing Graphium UI patterns
- ✅ **Scalable** - Handles hundreds of stacks efficiently

---

## 12. Next Steps

### Immediate Actions

1. **Review & Approve** this proposal
2. **Create Implementation Plan** with detailed tasks
3. **Set Up Development Environment** for stack testing
4. **Create Sample Templates** (5-10 common stacks)

### Implementation Order

```
Week 1:
├─ Day 1-2: Phase 1 (Core Features)
├─ Day 3: Phase 2 (Advanced Features)
└─ Day 4-5: Testing & Documentation

Week 2 (Optional):
└─ Phase 3: Integration & Polish
```

---

## 13. Questions & Decisions

### Open Questions

1. **Stack Templates**: Which templates should we include initially?
   - Graphium Dev (CouchDB)
   - Infisical (Postgres + DragonflyDB + Infisical)
   - LAMP Stack
   - Observability Stack (Grafana + Mimir)
   - Custom suggestions?

2. **Permissions**: Should stack operations require special permissions?
   - Deploy: Require "write" role
   - Stop/Start: Require "write" role
   - Remove: Require "admin" role?

3. **Multi-Host Stacks**: Should we support stacks deployed across multiple hosts?
   - Phase 1: Single host per stack
   - Phase 2: Multi-host support with network coordination

4. **Real-Time Updates**: WebSocket for live status updates?
   - Phase 1: Manual refresh
   - Phase 2: WebSocket integration

### Design Decisions Needed

- [ ] Confirm stack status color scheme
- [ ] Approve mockup designs
- [ ] Decide on default stack templates
- [ ] Define permission requirements
- [ ] Choose WebSocket update frequency

---

## Summary

This proposal adds comprehensive Stack Management to Graphium's UI, enabling visual orchestration of multi-container deployments using EVE's schema.org compliant stack definitions. The implementation integrates seamlessly with Graphium's existing Templ+HTMX architecture, follows established UI patterns, and provides a professional, user-friendly experience for deploying and managing complex container stacks.

**Estimated Timeline**: 2-3 days for core features, +1 day for advanced features
**Files to Create**: 8 new files (~2,500 lines total)
**Files to Modify**: 5 existing files (~300 lines additions)

Ready to proceed with implementation! 🚀
