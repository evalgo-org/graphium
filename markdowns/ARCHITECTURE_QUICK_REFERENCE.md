# Graphium UI Architecture - Quick Reference

## Technology Stack at a Glance

```
Frontend Layer:
  HTML Templates: Templ v0.3.960 (server-side, type-safe, compiles to Go)
  Interactivity:  HTMX v1.9.10 (attribute-based AJAX, no JS framework)
  Visualization:  Cytoscape.js v3.26.0 (interactive graph)
  Styling:        Custom CSS + CSS Variables (no Tailwind/Bootstrap)
  Theme:          Dark (default) + Light mode via data-theme attribute

Backend Layer:
  Web Framework:  Echo v4.13 (Go HTTP server)
  REST API:       30+ endpoints, JSON-LD semantic data
  Real-time:      WebSocket via Echo + Gorilla
  Auth:           JWT tokens + Session cookies
  Database:       CouchDB via eve library (semantic graph DB)

Data Models:
  Semantic Format: JSON-LD (https://json-ld.org)
  Schema.org Types: SoftwareApplication (Container), ComputerSystem (Host)
  Storage:        CouchDB documents with graph relationships
```

## Request/Response Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                        User Browser                             │
│                                                                 │
│  [Click Link] → [HTMX Trigger] → [Form Submit]                │
└────────────┬──────────────────────┬────────────────────────────┘
             │                      │
             ▼                      ▼
┌──────────────────────────────────────────────────────────────┐
│  HTTP Request (GET/POST)                                     │
│  - URL: /web/containers (page) or /web/containers/table (HTMX)│
│  - Cookies: access_token, refresh_token                      │
│  - FormData: search, status, page (query params)             │
└──────────────────────────────────────────────────────────────┘
             │
             ▼
┌──────────────────────────────────────────────────────────────┐
│  Echo Router & Middleware                                    │
│  - WebAuthMiddleware: Validates JWT in cookies               │
│  - WebAdminMiddleware: Checks admin role                     │
│  - Extracts user context: c.Get("claims")                    │
└──────────────────────────────────────────────────────────────┘
             │
             ▼
┌──────────────────────────────────────────────────────────────┐
│  Handler (internal/web/handlers.go)                          │
│  - Gets user from context                                    │
│  - Retrieves data from storage layer                         │
│  - Applies filters/pagination                                │
│  - Calls Render(c, TemplComponent(...))                      │
└──────────────────────────────────────────────────────────────┘
             │
             ▼
┌──────────────────────────────────────────────────────────────┐
│  Templ Component (templates.templ)                           │
│  Type-safe Go function with HTML output                      │
│  - Full page: LayoutWithUser() wrapper                       │
│  - Fragment: Just table/section (for HTMX)                   │
│  - No string concatenation, compiler checks types            │
└──────────────────────────────────────────────────────────────┘
             │
             ▼
┌──────────────────────────────────────────────────────────────┐
│  HTTP Response                                               │
│  - Full Page HTML (initial navigation)                       │
│  - Fragment HTML (HTMX requests swap into target selector)   │
│  - Status: 200 OK or 302 Redirect (form submissions)        │
└──────────────────────────────────────────────────────────────┘
             │
             ▼
┌──────────────────────────────────────────────────────────────┐
│  Browser Rendering                                           │
│  - Full page load (traditional flow)                         │
│  - HTMX swap: innerHTML/outerHTML into target                │
│  - CSS styling applied (dark/light theme via data-theme)    │
│  - JavaScript event handlers bound (for Cytoscape, etc)      │
└──────────────────────────────────────────────────────────────┘
```

## File Organization Map

```
graphium/
├── internal/
│   ├── web/                          # Web UI (Templ + HTMX + Auth)
│   │   ├── handlers.go               # 600+ lines: main page handlers
│   │   ├── handlers_auth.go          # Login/logout/profile forms
│   │   ├── handlers_users.go         # User management forms
│   │   ├── middleware.go             # Auth middleware
│   │   ├── templates.templ           # 2,492 lines: ALL UI components
│   │   ├── templates_templ.go        # Generated: compiled to Go
│   │   └── render.go                 # Helper: Render(c, Component)
│   │
│   ├── api/                          # REST API (JSON endpoints)
│   │   ├── server.go                 # Echo setup, route registration
│   │   ├── handlers_containers.go    # Container CRUD + logs
│   │   ├── handlers_hosts.go         # Host CRUD
│   │   ├── handlers_graph.go         # Graph data + visualization
│   │   ├── handlers_auth.go          # JWT login/refresh/logout
│   │   ├── handlers_users.go         # User management API
│   │   ├── middleware.go             # JWT validation, rate limiting
│   │   ├── websocket_hub.go          # Real-time updates
│   │   ├── logs.go                   # Container log retrieval
│   │   └── errors.go                 # Error response formatting
│   │
│   ├── storage/                      # CouchDB data layer
│   │   ├── storage.go                # Connection, methods
│   │   ├── containers.go             # GetContainer, ListContainers, etc
│   │   ├── hosts.go                  # GetHost, ListHosts, etc
│   │   └── graph.go                  # Traversal, topology queries
│   │
│   └── auth/                         # Authentication & authorization
│       ├── middleware.go             # JWT parsing, role checks
│       ├── claims.go                 # JWT claims structure
│       └── password.go               # Hashing, comparison
│
├── models/                           # Data models (JSON-LD)
│   ├── container.go                  # SoftwareApplication type
│   ├── host.go                       # ComputerSystem type
│   └── user.go                       # Person type + roles
│
├── static/
│   └── css/
│       └── styles.css                # 500+ lines: all styling
│                                     # CSS variables, responsive design
│
├── cmd/
│   ├── server.go                     # CLI entry point
│   └── main.go                       # Initialize & start
│
└── docs/                             # Swagger/OpenAPI specs
    └── swagger.* (auto-generated)
```

## Key Concepts & Patterns

### 1. Templ Components (Type-Safe Templates)
```go
// Component signature - returns templ.Component
templ ComponentName(data *Type, user *models.User) {
    @LayoutWithUser("Title", user) {
        <!-- HTML content here -->
        { data.Field }
    }
}

// Usage in handler
return Render(c, ComponentName(data, user))

// Benefits:
// - Compiler checks all variables, no string errors
// - Composable: components call other components with @
// - No template syntax to learn, just Go inside HTML
```

### 2. HTMX Patterns (Dynamic Updates)
```html
<!-- Live filter on input change (with debounce) -->
<input name="search" 
  hx-get="/web/containers/table"      <!-- Endpoint -->
  hx-target="#containers-table"       <!-- Where to put response -->
  hx-trigger="keyup changed delay:300ms"  <!-- When to trigger -->
  hx-include="[name='status']"        <!-- Include other form values -->
/>

<!-- Result: Typing triggers HTTP request, response replaces #containers-table div -->
```

### 3. Pagination Pattern
```go
// In handler:
page := 1
if p := c.QueryParam("page"); p != "" {
    page, _ = strconv.Atoi(p)
}
pageSize := 10
pagination := calculatePagination(totalItems, page, pageSize)
items := paginateItems(items, page, pageSize)

// In template:
@Pagination(pagination, "/web/items", queryParams)

// Output: Links like /web/items?page=2&status=running&search=web
```

### 4. Authentication Flow
```go
// Web UI: Cookie-based
1. User logs in at /web/auth/login
2. Handler validates password
3. Sets cookies: access_token (short), refresh_token (long)
4. Middleware checks cookies on each request
5. Extracts JWT claims: c.Get("claims").(*auth.Claims)

// REST API: Token-based
1. POST /api/v1/auth/login with credentials
2. Returns: { "accessToken": "...", "refreshToken": "..." }
3. Client includes in Authorization header: Bearer {token}
4. Middleware validates JWT signature
```

### 5. Form Submission Pattern
```html
<!-- Template: Form with error display -->
<form method="POST" action="/web/resource/create">
    if errorMsg != "" {
        <div class="alert alert-error">{ errorMsg }</div>
    }
    <input type="text" name="name" value={ obj.Name }/>
    <button type="submit">Create</button>
</form>

// Handler: Validate, save, redirect
func (h *Handler) Create(c echo.Context) error {
    name := c.FormValue("name")
    if name == "" {
        return Render(c, Form(&obj, "Name is required"))
    }
    if err := h.storage.Save(...); err != nil {
        return c.Redirect(http.StatusFound, "/path?error=Failed")
    }
    return c.Redirect(http.StatusFound, "/list")
}
```

## Database & Models

### JSON-LD Structure
```json
{
  "@context": "https://schema.org",
  "@type": "SoftwareApplication",    // Schema.org type
  "@id": "container-123",            // Unique identifier
  "_id": "container-123",            // CouchDB _id
  "_rev": "3-xyz...",                // CouchDB revision
  "name": "web-server",              // Human readable
  "executableName": "nginx:latest",  // Container image
  "status": "running",               // Runtime state
  "hostedOn": "host-01",             // Relationship to Host
  "dateCreated": "2024-10-29T..."    // ISO 8601 timestamp
}
```

### Graph Relationships
```
Host (ComputerSystem)
  ├── hosts many → Container (SoftwareApplication)
  │     └── has hostedOn → Host
  ├── has CPU, Memory, IP
  └── has location (Datacenter)

Stack (new, BundleOffer) - proposed
  ├── contains Services → Containers
  ├── deployed on → Host or Datacenter
  └── has Compose file (YAML)
```

## Styling

### CSS Variable System
```css
/* Dark theme (default) */
:root {
  --primary-color: #5F9EA0;        /* Teal - primary actions */
  --secondary-color: #B19CD9;      /* Lavender - accents */
  --success-color: #9CAF88;        /* Sage - success/running */
  --danger-color: #C78283;         /* Coral - errors/stopped */
  --bg-color: #2F4F4F;             /* Deep forest - background */
  --surface-color: #36454F;        /* Charcoal - cards/panels */
  --text-color: #F7F3E9;           /* Cream - text */
  --border-color: #4A5A5F;         /* Gray-blue - borders */
}

/* Light theme */
[data-theme="light"] {
  --primary-color: #4A8A8C;
  --bg-color: #F7F3E9;
  --surface-color: #FFFFFF;
  --text-color: #2F4F4F;
}
```

### Component Classes
```
.navbar            - Top navigation bar
.nav-links         - Navigation menu items
.container         - Main content wrapper (max-width: 1400px)
.page-header       - Page title + filters section
.filters           - Filter controls (search, select)
.stats-grid        - Dashboard stats cards
.stat-card         - Individual stat
.badge badge-*     - Status indicator (badge-running, badge-stopped, etc)
.table-container   - Wrapper for data tables
.data-table        - Actual table styling
.btn btn-*         - Button styles (btn-primary, btn-secondary, btn-danger)
.form-*            - Form elements (form-input, form-textarea, form-group)
.pagination        - Pagination controls
.detail-container  - Detail page wrapper
.empty-state       - Empty result message
```

## Real-Time Updates (WebSocket)

### Graph Visualization Updates
```
1. Client connects: ws://localhost:8080/api/v1/ws/graph
2. Backend broadcasts events:
   {
     "type": "container_added",
     "data": {
       "@id": "container-xyz",
       "name": "service-1",
       "status": "running"
     }
   }
3. JavaScript handler updates Cytoscape.js:
   - handleContainerAdded(data)
   - handleContainerUpdated(data)
   - handleContainerRemoved(data)
4. Animated transitions for smooth UX
```

## Integration Points for Stack Management

### What Needs to Be Added
1. **Model** (models/stack.go): Define Stack struct with JSON-LD annotations
2. **Storage** (internal/storage/stacks.go): CRUD methods for stacks
3. **API Handlers** (internal/api/handlers_stacks.go): REST endpoints
4. **Web Handlers** (internal/web/handlers_stacks.go): Form handlers
5. **Templates** (templates.templ): UI components for stack management
6. **Routes** (api/server.go setupRoutes): Register all endpoints
7. **Navigation**: Add "Stacks" link to navbar
8. **Graph**: Optional - add stack nodes to visualization

### Implementation Order
1. Create Stack model with JSON-LD structure
2. Implement storage layer (CRUDs + queries)
3. Create API handlers (REST endpoints)
4. Create web handlers (form handlers)
5. Create Templ components (UI pages)
6. Register routes
7. Add navigation
8. Test pagination, filtering, HTMX updates
9. Add graph visualization (optional)
10. Add WebSocket real-time updates (optional)

## Performance Considerations

- **Pagination**: 10 items per page (adjustable)
- **HTMX Debounce**: 300ms for search inputs (prevents spam)
- **Database Queries**: Use filters efficiently (indexed fields)
- **WebSocket**: Broadcast only to subscribed connections
- **Graph Rendering**: Cytoscape.js can handle 1000s of nodes
- **CSS Variables**: Instant theme switching without reload

## Debugging Tips

```bash
# Watch Templ compilation
task templ:watch

# View Swagger API docs
http://localhost:8080/docs

# Check database
http://localhost:5984/_utils

# Browser dev tools
- Network tab: See HTMX requests
- Console: WebSocket messages
- Elements: Check data-theme attribute for theme

# Server logs
# Show full request/response in Echo logger format
[2024-10-29T10:15:30Z] 200 GET /web/containers/table (42ms)
```

## Common Tasks

### Add a new list page
1. Create handler: `func (h *Handler) ItemsList(c echo.Context) error`
2. Create Templ component: `templ ItemsListWithUser(...) { @LayoutWithUser(...) }`
3. Create table fragment handler for HTMX updates
4. Create Templ table component: `templ ItemsTableWithPagination(...)`
5. Register routes in `server.go`
6. Add navigation link in `LayoutWithUser`

### Add HTMX filtering
1. Add `hx-get="/web/items/table"` to search/filter inputs
2. Add `hx-target="#items-table"` to match table wrapper ID
3. Add `hx-trigger="keyup delay:300ms"` for search, `change` for select
4. Add `hx-include="[name='other-filter']"` to include other form values
5. Handler extracts query params and applies filters

### Theme switching
1. User clicks theme toggle button
2. JavaScript toggles `html.setAttribute('data-theme', 'light')`
3. Saves preference: `localStorage.setItem('theme', 'light')`
4. CSS variables automatically switch via `[data-theme="light"]` selector

---

**For detailed information, see GRAPHIUM_UI_ARCHITECTURE.md**
**For stack integration, see STACK_INTEGRATION_GUIDE.md**
