# Stack Management Integration Guide

Quick reference for adding Docker Compose stack management to Graphium UI & API.

## 1. Create Stack Model

**File**: `models/stack.go`

```go
package models

import "time"

// Stack represents a Docker Compose stack deployment
type Stack struct {
    Context     string    `json:"@context" jsonld:"@context"`      // "https://schema.org"
    Type        string    `json:"@type" jsonld:"@type"`            // "BundleOffer"
    ID          string    `json:"@id" jsonld:"@id" couchdb:"_id"`
    Rev         string    `json:"_rev,omitempty" couchdb:"_rev"`
    
    Name        string    `json:"name" jsonld:"name" couchdb:"required,index"`
    Status      string    `json:"status" jsonld:"status" couchdb:"index"` // deploying, running, stopped, error
    Description string    `json:"description,omitempty" jsonld:"description"`
    
    // Composition
    Compose     string    `json:"compose" jsonld:"compose"` // YAML content
    Services    []string  `json:"services,omitempty" jsonld:"services"` // Service IDs (container IDs)
    
    // Infrastructure
    Datacenter  string    `json:"location" jsonld:"location" couchdb:"index"`
    HostID      string    `json:"hostedOn" jsonld:"hostedOn" couchdb:"index"` // Primary host
    
    // Timestamps
    CreatedAt   time.Time `json:"dateCreated" jsonld:"dateCreated"`
    UpdatedAt   time.Time `json:"dateModified" jsonld:"dateModified"`
    DeployedAt  *time.Time `json:"deployedAt,omitempty"`
    
    // Metadata
    Labels      map[string]string `json:"labels,omitempty" jsonld:"labels"`
    Owner       string    `json:"owner,omitempty"` // User ID who created stack
}

// StackService represents a service within a stack (convenience struct)
type StackService struct {
    Name      string
    Image     string
    Status    string
    Replicas  int
    Ports     []string
}
```

## 2. Add Storage Layer Methods

**File**: `internal/storage/stacks.go` (new file)

```go
package storage

import (
    "evalgo.org/graphium/models"
)

func (s *Storage) GetStack(id string) (*models.Stack, error)
func (s *Storage) ListStacks(filters map[string]interface{}) ([]*models.Stack, error)
func (s *Storage) SaveStack(stack *models.Stack) error
func (s *Storage) DeleteStack(id string) error
func (s *Storage) GetStacksByHost(hostID string) ([]*models.Stack, error)
func (s *Storage) GetStacksByDatacenter(datacenter string) ([]*models.Stack, error)
```

## 3. Add REST API Endpoints

**File**: `internal/api/handlers_stacks.go` (new file)

Add to `setupRoutes()` in `server.go`:

```go
// Stack routes
stacks := v1.Group("/stacks")
stacks.GET("", s.listStacks, webHandler.WebAuthMiddleware)
stacks.GET("/:id", s.getStack, ValidateIDFormat, webHandler.WebAuthMiddleware)
stacks.POST("", s.deployStack, s.authMiddle.RequireWrite)      // Deploy new stack
stacks.PUT("/:id", s.updateStack, ValidateIDFormat, s.authMiddle.RequireWrite)
stacks.DELETE("/:id", s.deleteStack, ValidateIDFormat, s.authMiddle.RequireWrite)
stacks.GET("/:id/services", s.getStackServices, ValidateIDFormat, webHandler.WebAuthMiddleware)
stacks.POST("/:id/scale", s.scaleStackService, ValidateIDFormat, s.authMiddle.RequireWrite)
```

**Handler Methods:**
```go
func (s *Server) listStacks(c echo.Context) error {
    // GET /api/v1/stacks
    // Returns paginated list with status, services count
}

func (s *Server) getStack(c echo.Context) error {
    // GET /api/v1/stacks/:id
    // Returns stack details + compose content
}

func (s *Server) deployStack(c echo.Context) error {
    // POST /api/v1/stacks
    // Payload: { "name": "...", "compose": "...", "location": "..." }
    // Validates YAML, creates stack, returns in response
}

func (s *Server) updateStack(c echo.Context) error {
    // PUT /api/v1/stacks/:id
    // Can update compose content and/or metadata
}

func (s *Server) deleteStack(c echo.Context) error {
    // DELETE /api/v1/stacks/:id
    // Removes stack and all associated containers
}

func (s *Server) getStackServices(c echo.Context) error {
    // GET /api/v1/stacks/:id/services
    // Returns list of container objects for this stack
}

func (s *Server) scaleStackService(c echo.Context) error {
    // POST /api/v1/stacks/:id/scale
    // Payload: { "service": "web", "replicas": 3 }
}
```

## 4. Add Web UI Templates

**File**: `internal/web/templates.templ` (append to existing file)

```templ
// Stacks list page
templ StacksListWithUser(stacks []*models.Stack, pagination PaginationInfo, user *models.User) {
    @LayoutWithUser("Stacks", user) {
        <div class="page-header">
            <div class="header-title">
                <h2>Docker Compose Stacks</h2>
                <a href="/web/stacks/new" class="btn btn-primary">Deploy New Stack</a>
            </div>
            <div class="filters">
                <input type="text" name="search" placeholder="Search stacks..."
                    hx-get="/web/stacks/table"
                    hx-target="#stacks-table"
                    hx-trigger="keyup changed delay:300ms"
                />
                <select name="status"
                    hx-get="/web/stacks/table"
                    hx-target="#stacks-table"
                    hx-trigger="change"
                >
                    <option value="">All Status</option>
                    <option value="running">Running</option>
                    <option value="stopped">Stopped</option>
                    <option value="deploying">Deploying</option>
                    <option value="error">Error</option>
                </select>
            </div>
        </div>
        
        <div id="stacks-table">
            @StacksTableWithPagination(stacks, pagination, "")
        </div>
    }
}

// Stacks table (for HTMX)
templ StacksTableWithPagination(stacks []*models.Stack, pagination PaginationInfo, queryParams string) {
    <div class="table-container">
        <table class="data-table">
            <thead>
                <tr>
                    <th>Name</th>
                    <th>Status</th>
                    <th>Services</th>
                    <th>Datacenter</th>
                    <th>Created</th>
                    <th>Actions</th>
                </tr>
            </thead>
            <tbody>
                for _, stack := range stacks {
                    <tr>
                        <td>
                            <strong>{ stack.Name }</strong>
                            <br/>
                            <small class="text-muted">{ stack.ID[:12] }</small>
                        </td>
                        <td>
                            <span class={ fmt.Sprintf("badge badge-%s", stack.Status) }>
                                { stack.Status }
                            </span>
                        </td>
                        <td>{ fmt.Sprintf("%d", len(stack.Services)) }</td>
                        <td>{ stack.Datacenter }</td>
                        <td>{ stack.CreatedAt.Format("2006-01-02 15:04") }</td>
                        <td>
                            <div class="action-buttons">
                                <a href={ templ.URL(fmt.Sprintf("/web/stacks/%s", stack.ID)) } class="btn-icon">👁️</a>
                                <a href={ templ.URL(fmt.Sprintf("/web/stacks/%s/edit", stack.ID)) } class="btn-icon">✏️</a>
                                <form method="POST" action={ templ.URL(fmt.Sprintf("/web/stacks/%s/delete", stack.ID)) } style="display:inline;">
                                    <button type="submit" class="btn-icon" onclick="return confirm('Delete stack?')">🗑️</button>
                                </form>
                            </div>
                        </td>
                    </tr>
                }
            </tbody>
        </table>
        @Pagination(pagination, "/web/stacks", queryParams)
    </div>
}

// Stack detail page
templ StackDetailWithUser(stack *models.Stack, services []*models.Container, user *models.User) {
    @LayoutWithUser("Stack: " + stack.Name, user) {
        <div class="detail-container">
            <div class="detail-header">
                <h2>{ stack.Name }</h2>
                <div class="detail-actions">
                    <span class={ fmt.Sprintf("badge badge-%s", stack.Status) }>{ stack.Status }</span>
                    <a href={ templ.URL(fmt.Sprintf("/web/stacks/%s/edit", stack.ID)) } class="btn btn-secondary">Edit</a>
                    <form method="POST" action={ templ.URL(fmt.Sprintf("/web/stacks/%s/delete", stack.ID)) } style="display:inline;">
                        <button type="submit" class="btn btn-danger" onclick="return confirm('Delete stack?')">Delete</button>
                    </form>
                </div>
            </div>
            
            <div class="detail-grid">
                <div class="detail-section">
                    <h3>Stack Information</h3>
                    <dl>
                        <dt>ID</dt>
                        <dd>{ stack.ID }</dd>
                        <dt>Status</dt>
                        <dd>{ stack.Status }</dd>
                        <dt>Datacenter</dt>
                        <dd>{ stack.Datacenter }</dd>
                        <dt>Created</dt>
                        <dd>{ stack.CreatedAt.Format("2006-01-02 15:04:05") }</dd>
                    </dl>
                </div>
                
                <div class="detail-section">
                    <h3>Services ({ fmt.Sprintf("%d", len(services)) })</h3>
                    <table class="data-table">
                        <thead>
                            <tr>
                                <th>Name</th>
                                <th>Image</th>
                                <th>Status</th>
                                <th>Action</th>
                            </tr>
                        </thead>
                        <tbody>
                            for _, svc := range services {
                                <tr>
                                    <td>{ svc.Name }</td>
                                    <td>{ svc.Image }</td>
                                    <td><span class={ fmt.Sprintf("badge badge-%s", svc.Status) }>{ svc.Status }</span></td>
                                    <td><a href={ templ.URL(fmt.Sprintf("/web/containers/%s", svc.ID)) }>View</a></td>
                                </tr>
                            }
                        </tbody>
                    </table>
                </div>
            </div>
            
            <div class="detail-section">
                <h3>Docker Compose File</h3>
                <pre class="compose-viewer">{ stack.Compose }</pre>
            </div>
        </div>
    }
}

// Stack deploy form
templ StackDeployForm(stack *models.Stack, user *models.User, errorMsg string) {
    @LayoutWithUser(if stack.ID != "" { "Edit Stack" } else { "Deploy Stack" }, user) {
        <form method="POST" action={ templ.URL(if stack.ID != "" { fmt.Sprintf("/web/stacks/%s/update", stack.ID) } else { "/web/stacks/create" }) } class="form-container">
            if errorMsg != "" {
                <div class="alert alert-error">{ errorMsg }</div>
            }
            
            <div class="form-group">
                <label for="name">Stack Name</label>
                <input type="text" id="name" name="name" value={ stack.Name } required class="form-input"/>
            </div>
            
            <div class="form-group">
                <label for="datacenter">Datacenter</label>
                <input type="text" id="datacenter" name="datacenter" value={ stack.Datacenter } class="form-input"/>
            </div>
            
            <div class="form-group">
                <label for="compose">Docker Compose YAML</label>
                <textarea id="compose" name="compose" required class="form-textarea compose-editor">{ stack.Compose }</textarea>
            </div>
            
            <div class="form-actions">
                <button type="submit" class="btn btn-primary">{ if stack.ID != "" { "Update Stack" } else { "Deploy Stack" } }</button>
                <a href="/web/stacks" class="btn btn-secondary">Cancel</a>
            </div>
        </form>
        
        <script>
        // Add YAML syntax highlighting or validation if needed
        </script>
    }
}
```

## 5. Add Web Handlers

**File**: `internal/web/handlers_stacks.go` (new file)

```go
package web

import (
    "net/http"
    "strconv"
    "github.com/labstack/echo/v4"
    "evalgo.org/graphium/models"
)

func (h *Handler) StacksList(c echo.Context) error {
    var user *models.User
    if claims, ok := c.Get("claims").(*auth.Claims); ok {
        user, _ = h.storage.GetUser(claims.UserID)
    }
    
    stacks, err := h.storage.ListStacks(nil)
    if err != nil {
        return c.String(http.StatusInternalServerError, "Failed to load stacks")
    }
    
    page := 1
    if p := c.QueryParam("page"); p != "" {
        if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
            page = parsed
        }
    }
    
    pageSize := 10
    pagination := calculatePagination(len(stacks), page, pageSize)
    paginated := paginateStacks(stacks, page, pageSize)
    
    return Render(c, StacksListWithUser(paginated, pagination, user))
}

func (h *Handler) StacksTable(c echo.Context) error {
    // HTMX table fragment
    stacks, err := h.storage.ListStacks(nil)
    if err != nil {
        return c.String(http.StatusInternalServerError, "Failed to load stacks")
    }
    
    page := 1
    if p := c.QueryParam("page"); p != "" {
        if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
            page = parsed
        }
    }
    
    pageSize := 10
    pagination := calculatePagination(len(stacks), page, pageSize)
    paginated := paginateStacks(stacks, page, pageSize)
    
    return Render(c, StacksTableWithPagination(paginated, pagination, ""))
}

func (h *Handler) StackDetail(c echo.Context) error {
    var user *models.User
    if claims, ok := c.Get("claims").(*auth.Claims); ok {
        user, _ = h.storage.GetUser(claims.UserID)
    }
    
    id := c.Param("id")
    stack, err := h.storage.GetStack(id)
    if err != nil {
        return c.String(http.StatusNotFound, "Stack not found")
    }
    
    // Get services (containers) for this stack
    services, err := h.storage.GetContainersByHost(stack.HostID)
    if err != nil {
        services = []*models.Container{}
    }
    
    return Render(c, StackDetailWithUser(stack, services, user))
}

func (h *Handler) DeployStackForm(c echo.Context) error {
    var user *models.User
    if claims, ok := c.Get("claims").(*auth.Claims); ok {
        user, _ = h.storage.GetUser(claims.UserID)
    }
    
    return Render(c, StackDeployForm(&models.Stack{}, user, ""))
}

func (h *Handler) DeployStack(c echo.Context) error {
    name := c.FormValue("name")
    compose := c.FormValue("compose")
    datacenter := c.FormValue("datacenter")
    
    if name == "" || compose == "" {
        errorMsg := "Stack name and compose file are required"
        return Render(c, StackDeployForm(&models.Stack{
            Name: name,
            Compose: compose,
            Datacenter: datacenter,
        }, nil, errorMsg))
    }
    
    // Save stack
    stack := &models.Stack{
        ID: generateID("stack"),
        Name: name,
        Compose: compose,
        Datacenter: datacenter,
        Status: "deploying",
        CreatedAt: time.Now(),
    }
    
    if err := h.storage.SaveStack(stack); err != nil {
        return c.Redirect(http.StatusFound, "/web/stacks?error=Failed+to+deploy+stack")
    }
    
    return c.Redirect(http.StatusFound, "/web/stacks")
}

func (h *Handler) DeleteStack(c echo.Context) error {
    id := c.Param("id")
    if err := h.storage.DeleteStack(id); err != nil {
        return c.Redirect(http.StatusFound, "/web/stacks?error=Failed+to+delete+stack")
    }
    
    return c.Redirect(http.StatusFound, "/web/stacks")
}

// Helper functions
func paginateStacks(stacks []*models.Stack, page, pageSize int) []*models.Stack {
    start := (page - 1) * pageSize
    if start >= len(stacks) {
        return []*models.Stack{}
    }
    end := start + pageSize
    if end > len(stacks) {
        end = len(stacks)
    }
    return stacks[start:end]
}
```

## 6. Update Navigation

In `templates.templ`, add to the `LayoutWithUser` navigation:

```html
<li><a href="/web/stacks" class="nav-link">Stacks</a></li>
```

## 7. Register Routes

In `internal/api/server.go` `setupRoutes()`:

```go
// Stacks management
webStacks := s.echo.Group("/web/stacks")
webStacks.Use(webHandler.WebAuthMiddleware)
webStacks.GET("", webHandler.StacksList)
webStacks.GET("/table", webHandler.StacksTable)
webStacks.GET("/new", webHandler.DeployStackForm)
webStacks.POST("/create", webHandler.DeployStack)
webStacks.GET("/:id", webHandler.StackDetail)
webStacks.GET("/:id/edit", webHandler.EditStackForm)
webStacks.POST("/:id/update", webHandler.UpdateStack)
webStacks.POST("/:id/delete", webHandler.DeleteStack)
```

## 8. Update Graph Visualization (Optional)

Add stack nodes to the graph visualization in `GraphView`:

```javascript
// In graph data processing
if (node.data.type === 'stack') {
    cy.add({
        data: {
            id: nodeId,
            label: node.data.name,
            type: 'stack',
            status: node.data.status
        }
    });
}

// Update styling
{
    selector: 'node[type="stack"]',
    style: {
        'background-color': '#a78bfa',  // Purple
        'shape': 'rectangle',
        'width': 100,
        'height': 60
    }
}
```

## Key Patterns to Follow

1. **Use existing pagination** - Reuse `PaginationInfo` and pagination helpers
2. **Follow HTMX pattern** - Separate table fragment handlers for live filtering
3. **Error handling** - Use query params for error messages (e.g., `?error=...`)
4. **Form submission** - POST to handlers that validate and redirect
5. **Authentication** - Check user context with `c.Get("claims")`
6. **Styling** - Use existing CSS classes: `btn`, `badge`, `table-container`, etc.
7. **Model structure** - Follow JSON-LD pattern like Container/Host models

## Testing Checklist

- [ ] List stacks with pagination
- [ ] Search/filter stacks with HTMX
- [ ] Deploy new stack (create)
- [ ] View stack details
- [ ] Update stack (edit compose)
- [ ] Delete stack
- [ ] Scale service (if implemented)
- [ ] WebSocket updates for stack status changes
- [ ] Graph visualization includes stacks
- [ ] API endpoints match REST pattern
- [ ] Admin-only access control (if needed)

## Related Files Reference

- Templ docs: https://templ.guide
- HTMX docs: https://htmx.org
- Container example: `/home/opunix/graphium/internal/web/handlers.go` (ContainersList)
- Storage example: `/home/opunix/graphium/internal/storage/containers.go`
- API example: `/home/opunix/graphium/internal/api/handlers_containers.go`
