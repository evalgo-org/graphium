# Graphium UI Architecture Overview

## Executive Summary

Graphium is a semantic container orchestration platform with a modern, full-stack Go web application. It uses **Templ** for server-side template rendering and **HTMX** for lightweight, dynamic UI interactions without building a separate SPA. The platform manages containers and hosts using JSON-LD semantic data models and serves a REST API with real-time WebSocket updates.

---

## 1. UI Framework & Templating

### Framework Stack
- **Web Framework**: Echo v4 (Go)
- **Templating Engine**: Templ v0.3.960 (https://templ.guide)
  - Type-safe, composable server-side templates
  - Compiles to pure Go code
  - No string-based template syntax
  - Direct integration with Echo's response writer

- **Frontend Interactivity**: HTMX v1.9.10 (https://htmx.org)
  - Attribute-based AJAX requests
  - Dynamic table updates without page reloads
  - Real-time WebSocket support via `hx-ws` extension
  - Request/response driven architecture

- **Client-Side Visualization**: Cytoscape.js v3.26.0
  - Interactive graph visualization
  - Node filtering and layout algorithms
  - Real-time node/edge updates via WebSocket

- **Styling**: Custom CSS with CSS Variables (No framework like Tailwind)
  - Dark mode (default) + Light mode theme
  - Zen color palette (teals, lavenders, earth tones)
  - CSS variables for theming: `--primary-color`, `--surface-color`, etc.
  - Responsive design with flexbox/grid layouts

### Key Files
- `/home/opunix/graphium/internal/web/templates.templ` (2,492 lines)
  - All UI templates defined as Templ components
  - Compiles to `/home/opunix/graphium/internal/web/templates_templ.go`
- `/home/opunix/graphium/static/css/styles.css` (~500+ lines)
  - Dark/light theme CSS variables
  - Component styling (cards, tables, badges, buttons)
- `/home/opunix/graphium/internal/web/render.go` (12 lines)
  - Simple render helper: `Render(c echo.Context, component templ.Component)`

---

## 2. Existing UI Pages & Structure

### Current Pages Implemented

#### Authentication Pages
- **Login Page** (`/web/auth/login`)
  - Form-based authentication
  - Session + JWT token cookies
  - Error message display
  - Redirect to dashboard on successful login

- **Profile Page** (`/web/profile`)
  - View user info (username, email, role)
  - Change password form
  - API key management (generate/revoke)

#### Main Dashboard
- **Dashboard** (`/`)
  - Infrastructure overview stats grid
  - Stats: Host count, Container count, Running containers, Hosts by status
  - Quick action buttons to navigate to detailed views
  - Real-time stats via WebSocket (not yet fully implemented)

#### Container Management
- **Containers List** (`/web/containers`)
  - Paginated table (10 items per page)
  - Search by name/ID with HTMX live filtering
  - Status filter dropdown (running, stopped, paused, exited)
  - View details and logs links per container
  - HTMX triggers: `keyup` for search (300ms debounce), `change` for status filter

- **Container Details** (`/web/containers/:id`)
  - Full container information display
  - Host relationship (linked to host page)
  - Port mappings table
  - Environment variables display
  - Created date and other metadata

- **Container Logs** (`/web/containers/:id/logs`)
  - Live logs viewer with HTMX polling
  - Auto-refreshes every 5 seconds
  - Configurable tail lines
  - Raw container log output

#### Host Management
- **Hosts List** (`/web/hosts`)
  - Paginated table (10 items per page)
  - Search by name/ID with HTMX live filtering
  - Status filter (active, inactive, maintenance)
  - Datacenter filter selector
  - View details and containers links per host
  - Displays: Name, IP, CPU cores, Memory (GB), Status, Datacenter

- **Host Details** (`/web/hosts/:id`)
  - Full host information
  - Hardware specs (CPU, Memory)
  - Network configuration (IP address)
  - List of running containers on that host
  - Status and datacenter location

#### Visualization Pages
- **Topology View** (`/web/topology`)
  - Grid-based visualization of hosts and containers
  - Per-datacenter grouping
  - Host cards with container counts
  - Filterable by datacenter
  - Links to filtered container lists

- **Graph Visualization** (`/web/graph`)
  - Cytoscape.js powered interactive graph
  - Filters: Node type (hosts/containers), Status, Datacenter, Layout algorithm
  - Real-time updates via WebSocket (`/api/v1/ws/graph`)
  - Export functions: PNG, SVG, JSON
  - Zoom, pan, fit-to-screen controls
  - Animated node addition/removal/updates

#### User Management (Admin Only)
- **Users List** (`/web/users`) - Admin only
  - Table of all users
  - View, edit, delete per user
  - Create new user button

- **User Detail** (`/web/users/:id`)
  - User profile information
  - Role and permissions display
  - API key management

- **User Forms** (`/web/users/new`, `/web/users/:id/edit`)
  - Create/edit user forms
  - Username, email, password fields
  - Role selection dropdown
  - Form validation feedback

### Page Structure Pattern

All pages follow this template hierarchy:

```
LayoutWithUser("Page Title", user)
├── Navigation Bar (with theme toggle, user menu)
├── Page Content
│   ├── Page Header (title, filters)
│   ├── Main Content
│   └── Pagination (if list view)
└── Footer
```

---

## 3. Current API Endpoints

### API Base URL
`/api/v1`

### Container Endpoints
```
GET    /containers              - List containers (paginated, filterable)
GET    /containers/:id          - Get container details
GET    /containers/:id/logs     - Get container logs (supports tail parameter)
GET    /containers/:id/logs/download - Download container logs
POST   /containers              - Create container (agent auth)
PUT    /containers/:id          - Update container (agent auth)
DELETE /containers/:id          - Delete container (agent auth)
POST   /containers/bulk         - Bulk create containers
```

### Host Endpoints
```
GET    /hosts                   - List hosts (paginated, filterable)
GET    /hosts/:id               - Get host details
POST   /hosts                   - Create host (agent auth)
PUT    /hosts/:id               - Update host (agent auth)
DELETE /hosts/:id               - Delete host (agent auth)
POST   /hosts/bulk              - Bulk create hosts
```

### Query Endpoints
```
GET    /query/containers/by-host/:hostId        - Get containers on specific host
GET    /query/containers/by-status/:status      - Filter containers by status
GET    /query/hosts/by-datacenter/:datacenter   - Filter hosts by datacenter
GET    /query/traverse/:id                      - Graph traversal from node
GET    /query/dependents/:id                    - Get dependent nodes
GET    /query/topology/:datacenter              - Get datacenter topology
```

### Statistics Endpoints
```
GET    /stats                   - Get overall statistics
GET    /stats/containers/count  - Container count
GET    /stats/hosts/count       - Host count
GET    /stats/distribution      - Host-container distribution
```

### Graph Endpoints
```
GET    /graph                   - Get graph data (nodes + edges JSON)
GET    /graph/stats             - Graph statistics
GET    /graph/layout            - Graph layout data
```

### WebSocket Endpoints
```
WS     /ws/graph                - Real-time graph updates
WS     /ws/stats                - Real-time statistics updates
```

### Authentication Endpoints (API)
```
POST   /auth/login              - Get access + refresh tokens
POST   /auth/register           - Register new user (admin only)
POST   /auth/refresh            - Refresh access token
POST   /auth/logout             - Logout (revoke tokens)
GET    /auth/me                 - Get current user info
```

### User Management Endpoints (API)
```
GET    /users                   - List users (admin only)
GET    /users/:id               - Get user details (admin only)
PUT    /users/:id               - Update user (admin only)
DELETE /users/:id               - Delete user (admin only)
POST   /users/password          - Change own password
POST   /users/api-keys          - Generate API key
DELETE /users/api-keys/:index   - Revoke API key
```

### Web Form Endpoints (Templ + Form submissions)
```
GET    /web/auth/login          - Login page
POST   /web/auth/login          - Form submission (sets cookies)
GET    /web/auth/logout         - Logout

GET    /web/containers          - Containers page
GET    /web/containers/table    - HTMX table update
GET    /web/containers/:id      - Container detail page
GET    /web/containers/:id/logs - Container logs page

GET    /web/hosts               - Hosts page
GET    /web/hosts/table         - HTMX table update
GET    /web/hosts/:id           - Host detail page

GET    /web/topology            - Topology view
GET    /web/graph               - Graph visualization page

GET    /web/profile             - User profile page
POST   /web/profile/password    - Change password

GET    /web/users               - Users list (admin)
GET    /web/users/new           - New user form
POST   /web/users/create        - Create user form submission
GET    /web/users/:id           - User detail
GET    /web/users/:id/edit      - Edit user form
POST   /web/users/:id/update    - Update user form submission
POST   /web/users/:id/delete    - Delete user form submission
POST   /web/users/:id/api-keys/generate - Generate API key
POST   /web/users/:id/api-keys/:index/revoke - Revoke API key
```

---

## 4. UI-Backend Interaction Pattern

### Form Submission Flow (Traditional)
1. User fills form in Templ template
2. Form POST to `/web/*/create` or `/web/*/update` handler
3. Handler validates and calls storage layer
4. Handler redirects to appropriate page (with error message in URL query if needed)
5. Templ template renders response page

### HTMX Dynamic Table Updates
1. User types in search input or changes filter dropdown
2. HTMX attributes trigger: `hx-get="/web/containers/table"` + `hx-target="#containers-table"`
3. Query params automatically included: `hx-include="[name='search']"` + `hx-include="[name='status']"`
4. Backend handler (`ContainersTable` or `ContainersTableWithPagination`) returns HTML fragment
5. HTMX swaps response into `#containers-table` div
6. No page reload, instant filtering

**Example HTMX Usage:**
```html
<input type="text" name="search" 
  hx-get="/web/containers/table"
  hx-target="#containers-table"
  hx-trigger="keyup changed delay:300ms"
  hx-include="[name='status']"
/>
<select name="status"
  hx-get="/web/containers/table"
  hx-target="#containers-table"
  hx-trigger="change"
  hx-include="[name='search']"
>
```

### Pagination Flow
1. Pagination component generates page links
2. Links include query params: `?page=2&status=running&search=web`
3. Handler extracts page number and filters
4. Returns paginated HTML fragment for HTMX or full page for direct navigation

### WebSocket Real-Time Updates
1. Graph page connects: `ws://localhost:8080/api/v1/ws/graph`
2. HTMX manages connection with `hx-ws="connect:/api/v1/ws/graph"`
3. Backend broadcasts node/edge add/remove/update messages
4. Client-side JavaScript handles events: `handleContainerAdded()`, `handleHostUpdated()`, etc.
5. Cytoscape.js updates animated with smooth transitions

---

## 5. Styling Approach

### CSS Architecture
- **Location**: `/home/opunix/graphium/static/css/styles.css`
- **Approach**: Custom CSS with CSS Variables (no Tailwind, Bootstrap, or utility-first framework)
- **Theme Support**: Dark (default) + Light mode via `data-theme="light"` attribute on `<html>`

### Color Palette (Dark Theme - Default)
```css
--primary-color: #5F9EA0;        /* Muted Teal */
--secondary-color: #B19CD9;      /* Soft Lavender */
--success-color: #9CAF88;        /* Soft Sage */
--warning-color: #D4A574;        /* Warm Earth Tone */
--danger-color: #C78283;         /* Muted Coral */
--bg-color: #2F4F4F;             /* Deep Forest Green */
--surface-color: #36454F;        /* Charcoal Gray */
--text-color: #F7F3E9;           /* Warm Cream */
--border-color: #4A5A5F;         /* Lighter Charcoal */
```

### Light Theme
```css
[data-theme="light"] {
  --primary-color: #4A8A8C;
  --bg-color: #F7F3E9;           /* Warm Cream */
  --surface-color: #FFFFFF;      /* Pure white */
  --text-color: #2F4F4F;         /* Deep Forest Green */
  --border-color: #D4CFC0;
}
```

### Key Components
- **Navigation Bar**: Fixed top, flex layout, user menu on right
- **Stat Cards**: Grid layout, hover elevation effect, left border accent
- **Data Tables**: Bordered rows, badge status indicators, action buttons
- **Badges**: Color coded by status (running=sage, stopped=gray, danger=coral)
- **Buttons**: Primary (teal), Secondary (gray), Info (teal)
- **Forms**: Label-input pairs, error message display, submit buttons
- **Theme Toggle**: Button in footer area, toggles `data-theme` attribute, stores preference in localStorage

### Responsive Design
- Mobile-first approach
- Grid layouts with `auto-fit, minmax(250px, 1fr)`
- Flexbox for navigation and button groups
- Max-width container: 1400px

---

## 6. Data Models & Structure

### Container Model (JSON-LD: SoftwareApplication)
```go
type Container struct {
    Context     string            `json:"@context"`       // "https://schema.org"
    Type        string            `json:"@type"`          // "SoftwareApplication"
    ID          string            `json:"@id"`            // Container ID (CouchDB _id)
    Rev         string            `json:"_rev,omitempty"` // CouchDB revision
    Name        string            `json:"name"`           // Container name
    Image       string            `json:"executableName"` // Container image
    Status      string            `json:"status"`         // running/stopped/paused/exited
    HostedOn    string            `json:"hostedOn"`       // Host ID relationship
    Ports       []Port            `json:"ports"`          // Port mappings
    Env         map[string]string `json:"environment"`    // Environment variables
    Created     string            `json:"dateCreated"`    // ISO 8601 timestamp
}
```

### Host Model (JSON-LD: ComputerSystem)
```go
type Host struct {
    Context    string `json:"@context"`  // "https://schema.org"
    Type       string `json:"@type"`     // "ComputerSystem"
    ID         string `json:"@id"`       // Host ID (CouchDB _id)
    Rev        string `json:"_rev,omitempty"` // CouchDB revision
    Name       string `json:"name"`      // Host name
    IPAddress  string `json:"ipAddress"` // IP address
    CPU        int    `json:"cpu"`       // CPU cores (Schema.org: processorCount)
    Memory     int64  `json:"memory"`    // Memory in bytes (Schema.org: memorySize)
    Status     string `json:"status"`    // active/inactive/maintenance
    Datacenter string `json:"location"` // Datacenter (Schema.org: location)
}
```

### User Model
```go
type User struct {
    ID              string    `json:"@id"`
    Type            string    `json:"@type"`     // "Person"
    Username        string    `json:"name"`
    Email           string    `json:"email"`
    PasswordHash    string    `json:"_password"` // Not exposed in JSON
    Role            string    `json:"role"`      // admin/user/agent
    Enabled         bool      `json:"enabled"`
    CreatedAt       time.Time `json:"dateCreated"`
    UpdatedAt       time.Time `json:"dateModified"`
    LastLoginAt     *time.Time
}
```

### Pagination Info (Used in Web Templates)
```go
type PaginationInfo struct {
    Page       int  // Current page (1-indexed)
    PageSize   int  // Items per page (default 10)
    TotalItems int  // Total items in collection
    TotalPages int  // Calculated: ceil(TotalItems / PageSize)
    HasPrev    bool // Can go to previous page
    HasNext    bool // Can go to next page
}
```

---

## 7. Handler Architecture

### Web Handler Structure
- **Location**: `/home/opunix/graphium/internal/web/handlers.go` (~600+ lines)
- **Type**: `Handler` struct containing `storage` and `config`

**Key Methods:**
```go
func (h *Handler) Dashboard(c echo.Context) error
func (h *Handler) ContainersList(c echo.Context) error
func (h *Handler) ContainersTable(c echo.Context) error        // HTMX fragment
func (h *Handler) ContainerDetail(c echo.Context) error
func (h *Handler) HostsList(c echo.Context) error
func (h *Handler) HostsTable(c echo.Context) error             // HTMX fragment
func (h *Handler) HostDetail(c echo.Context) error
func (h *Handler) TopologyView(c echo.Context) error
func (h *Handler) GraphView(c echo.Context) error
func (h *Handler) ContainerLogs(c echo.Context) error
```

### Authentication Handlers
- **Location**: `/home/opunix/graphium/internal/web/handlers_auth.go`

**Methods:**
```go
func (h *Handler) LoginPage(c echo.Context) error
func (h *Handler) Login(c echo.Context) error                  // POST form handler
func (h *Handler) Logout(c echo.Context) error
func (h *Handler) Profile(c echo.Context) error
func (h *Handler) ChangePassword(c echo.Context) error
```

### User Management Handlers
- **Location**: `/home/opunix/graphium/internal/web/handlers_users.go`

**Methods:**
```go
func (h *Handler) ListUsers(c echo.Context) error
func (h *Handler) ViewUser(c echo.Context) error
func (h *Handler) NewUserForm(c echo.Context) error
func (h *Handler) CreateUser(c echo.Context) error             // POST form handler
func (h *Handler) EditUserForm(c echo.Context) error
func (h *Handler) UpdateUser(c echo.Context) error             // POST form handler
func (h *Handler) DeleteUser(c echo.Context) error
func (h *Handler) GenerateAPIKey(c echo.Context) error
func (h *Handler) RevokeAPIKey(c echo.Context) error
```

### Middleware
- **Web Auth Middleware**: `WebAuthMiddleware` - Validates session cookies/JWT
- **Admin Middleware**: `WebAdminMiddleware` - Checks user has admin role
- **API Auth Middleware**: Token-based JWT validation

---

## 8. Key Technical Characteristics

### Strengths
1. **Type-Safe Templates**: Templ compiles to Go code - no runtime template syntax errors
2. **Lightweight AJAX**: HTMX eliminates need for JavaScript framework (React, Vue, Angular)
3. **Real-Time Support**: WebSocket integration for live graph updates
4. **Semantic Data**: JSON-LD provides structured, queryable data
5. **Clean Separation**: Web UI separate from REST API (can be used independently)
6. **Pagination**: Built-in pagination for list views with query param support
7. **Theming**: CSS variables enable instant dark/light mode switching
8. **Authentication**: Session-based (web UI) + JWT-based (API)

### Current Limitations
1. **No JavaScript SPA**: Graph page uses vanilla JS + Cytoscape, not a framework
2. **Limited API Documentation**: Swagger docs exist but may need expansion
3. **No Stack Management**: No dedicated UI/API for Docker Compose stacks
4. **Form Validation**: Limited client-side validation (mostly server-side)
5. **Real-Time Stats**: WebSocket stats endpoint not fully implemented in UI

### Architecture Patterns
1. **Handler-Based Routes**: Echo routes map to handler methods
2. **Template Components**: Templ allows composable, reusable UI components
3. **Fragment Responses**: HTMX targets specific DOM fragments for updates
4. **Pagination**: Separate list handlers and table fragment handlers
5. **Error Handling**: Query params for error messages (e.g., `/web/auth/login?error=...`)
6. **User Context**: Handlers extract user from JWT claims for personalization

---

## 9. Integration Points for Stack Management

### Where Stack Data Would Fit
1. **New Model**: `Stack` (Docker Compose)
   ```go
   type Stack struct {
       Context     string            `json:"@context"`
       Type        string            `json:"@type"`     // "BundleOffer" or custom
       ID          string            `json:"@id"`
       Name        string            `json:"name"`
       Status      string            `json:"status"`    // deploying/running/stopped/error
       Services    []string          `json:"services"` // Service IDs
       Compose     string            `json:"compose"`   // YAML content
       Datacenter  string            `json:"location"`
       CreatedAt   time.Time         `json:"dateCreated"`
   }
   ```

2. **New API Endpoints** (follow existing pattern):
   - `GET /api/v1/stacks` - List all stacks
   - `GET /api/v1/stacks/:id` - Get stack details
   - `POST /api/v1/stacks` - Deploy stack
   - `PUT /api/v1/stacks/:id` - Update stack
   - `DELETE /api/v1/stacks/:id` - Remove stack
   - `GET /api/v1/stacks/:id/services` - Get stack services (containers)
   - `POST /api/v1/stacks/:id/scale` - Scale stack services

3. **New Web Pages**:
   - `/web/stacks` - List stacks with status, action buttons
   - `/web/stacks/:id` - Stack detail, services, logs
   - `/web/stacks/new` - Deploy form (YAML editor)
   - `/web/stacks/:id/edit` - Edit compose file
   - `/web/stacks/:id/logs` - Aggregate service logs

4. **Templ Components to Create**:
   - `StacksListWithUser()` - Stacks list page
   - `StacksTable()` / `StacksTableWithPagination()` - HTMX table updates
   - `StackDetailWithUser()` - Stack detail page with services
   - `StackForm()` - Deploy/edit form with YAML editor
   - `ServicesList()` - Table of services in stack

5. **Web Handlers** (new file: `handlers_stacks.go`):
   - `StacksList()`, `StacksTable()`
   - `StackDetail()`
   - `DeployForm()`, `Deploy()`, `UpdateStack()`, `DeleteStack()`
   - `ScaleService()`

6. **Graph Updates**:
   - New node type: "stack" in Cytoscape
   - New edge: Stack -> Services (containers)
   - Service nodes could be children of stack in hierarchy view

---

## 10. File Organization Summary

```
/home/opunix/graphium/
├── internal/
│   ├── web/
│   │   ├── handlers.go              # Main page handlers
│   │   ├── handlers_auth.go         # Auth form handlers
│   │   ├── handlers_users.go        # User management handlers
│   │   ├── handlers_stacks.go       # [TODO] Stack handlers
│   │   ├── middleware.go            # Auth middleware
│   │   ├── templates.templ          # All Templ components (2,492 lines)
│   │   ├── templates_templ.go       # [Generated] Compiled templates
│   │   └── render.go                # Render helper
│   │
│   ├── api/
│   │   ├── server.go                # Server setup, routes
│   │   ├── handlers_containers.go   # Container API
│   │   ├── handlers_hosts.go        # Host API
│   │   ├── handlers_stacks.go       # [TODO] Stack API
│   │   ├── middleware.go            # API middleware
│   │   ├── errors.go                # Error responses
│   │   ├── websocket_hub.go         # WebSocket management
│   │   └── logs.go                  # Container logs
│   │
│   ├── storage/
│   │   └── *.go                     # CouchDB storage layer
│   │
│   └── auth/
│       └── *.go                     # JWT/session auth
│
├── models/
│   ├── container.go
│   ├── host.go
│   ├── user.go
│   └── stack.go                     # [TODO] Stack model
│
├── static/
│   └── css/
│       └── styles.css               # All styling
│
└── cmd/
    └── server.go                    # Entry point
```

---

## 11. Development Notes

### Building & Running
```bash
# Generate Templ code (auto on save with task dev)
task templ:generate

# Build web UI + API
task build

# Run dev server with hot reload
task dev
```

### Testing UI Changes
1. Edit `.templ` file
2. Templ auto-compiles to `*_templ.go`
3. No restart needed for handler changes
4. Refresh browser to see template changes

### Adding New Pages
1. Create Templ component in `templates.templ`
2. Create handler in `handlers.go` (calls `Render(c, Component(...))`)
3. Register route in `/internal/api/server.go` `setupRoutes()`
4. Add navigation link in `LayoutWithUser` template

### Adding HTMX Table Updates
1. Create table fragment Templ component (no layout, just table)
2. Create handler that returns fragment
3. Add HTMX attributes to input/select in list page
4. Set `hx-target="#table-id"` and `hx-get="/endpoint"`

---

## Summary

Graphium's UI is a modern, server-side rendered web application using Templ for type-safe templates and HTMX for dynamic interactions. It provides a dashboard for viewing containers and hosts, with features like real-time graph visualization, pagination, filtering, and user management. The architecture cleanly separates the REST API from the web UI while sharing the same backend storage and models. Integration of stack management features would follow the established patterns: new model, new API endpoints, new web pages, and new Templ components.
