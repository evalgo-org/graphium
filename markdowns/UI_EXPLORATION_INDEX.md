# Graphium UI Architecture - Exploration Documentation Index

This directory contains comprehensive documentation of the Graphium UI architecture, API structure, and integration points for stack management features.

## Documents Overview

### 1. GRAPHIUM_UI_ARCHITECTURE.md (Primary Document)
**Length**: 25 KB | **Sections**: 11

Comprehensive architectural overview covering:

1. **UI Framework & Templating**
   - Technology stack (Templ, HTMX, Cytoscape.js)
   - Framework choices and design rationale
   - Key files and organization

2. **Existing UI Pages & Structure**
   - All implemented pages (10+)
   - Dashboard, containers, hosts, topology, graph
   - User management and authentication
   - Page structure patterns

3. **Current API Endpoints**
   - Complete endpoint listing (30+)
   - Container, host, query, stats, graph endpoints
   - WebSocket and authentication routes
   - Web form submission endpoints

4. **UI-Backend Interaction Pattern**
   - Traditional form submission flow
   - HTMX dynamic table updates
   - Pagination implementation
   - WebSocket real-time updates

5. **Styling Approach**
   - CSS variable system for theming
   - Dark (default) and light mode
   - Color palette and component classes
   - Responsive design principles

6. **Data Models & Structure**
   - JSON-LD format with Schema.org types
   - Container model (SoftwareApplication)
   - Host model (ComputerSystem)
   - User model and relationships
   - Pagination info structure

7. **Handler Architecture**
   - Web handler structure (handlers.go)
   - Authentication handlers (handlers_auth.go)
   - User management handlers (handlers_users.go)
   - Middleware components

8. **Key Technical Characteristics**
   - Strengths and current limitations
   - Architecture patterns used
   - Type safety and validation approach

9. **Integration Points for Stack Management**
   - Where stack data fits in the model
   - New API endpoints needed
   - New web pages required
   - Templ components to create
   - Handler methods needed
   - Graph visualization integration

10. **File Organization Summary**
    - Complete directory structure
    - File descriptions and purposes
    - Related file locations

11. **Development Notes**
    - Building and running instructions
    - Testing UI changes
    - Adding new pages
    - HTMX table update patterns

**Start here for**: Understanding the overall architecture, learning what exists, and seeing integration points.

---

### 2. STACK_INTEGRATION_GUIDE.md (Implementation Guide)
**Length**: 19 KB | **Focus**: Code-ready implementation

Step-by-step implementation guide with working code examples for adding stack management:

**Sections**:

1. **Create Stack Model**
   - Complete Stack struct definition
   - JSON-LD annotations
   - Field descriptions
   - StackService helper struct

2. **Add Storage Layer Methods**
   - Method signatures for CRUD operations
   - Query methods by host/datacenter
   - File location (internal/storage/stacks.go)

3. **Add REST API Endpoints**
   - Route definitions with middleware
   - Handler method signatures
   - Detailed descriptions of each endpoint

4. **Add Web UI Templates**
   - Complete Templ components with working code
   - StacksListWithUser (main page)
   - StacksTableWithPagination (HTMX fragment)
   - StackDetailWithUser (detail page)
   - StackDeployForm (create/edit form)

5. **Add Web Handlers**
   - Complete handler methods with code
   - List handler with pagination
   - HTMX table fragment handler
   - Detail and CRUD handlers
   - Helper functions

6. **Update Navigation**
   - Where to add "Stacks" link
   - Exact code snippet

7. **Register Routes**
   - Where to add routes in server.go
   - Complete route group setup

8. **Update Graph Visualization**
   - Add stack node type to Cytoscape
   - Styling for stack nodes
   - Edge definitions

**Key Patterns to Follow**
- Using existing pagination system
- Following HTMX conventions
- Error handling patterns
- Authentication checks
- Styling consistency

**Testing Checklist**
- 10-item verification checklist
- All features to validate

**Start here for**: Getting specific code to implement, copy-paste ready examples, and detailed implementation steps.

---

### 3. ARCHITECTURE_QUICK_REFERENCE.md (Cheat Sheet)
**Length**: 18 KB | **Focus**: Quick lookup and visualization

Visual quick-reference guide with diagrams and code snippets:

**Sections**:

1. **Technology Stack at a Glance**
   - Frontend components list
   - Backend components list
   - Data format and storage
   - Version numbers

2. **Request/Response Flow**
   - Visual diagram (ASCII)
   - Steps from browser to database
   - Rendering and response patterns

3. **File Organization Map**
   - Directory tree with descriptions
   - Purpose of each directory
   - Key files highlighted

4. **Key Concepts & Patterns**
   - Templ components with code
   - HTMX patterns with examples
   - Pagination implementation
   - Authentication flow
   - Form submission pattern

5. **Database & Models**
   - JSON-LD structure example
   - Graph relationships diagram
   - Schema.org type mappings

6. **Styling**
   - CSS variable system
   - Light/dark theme definition
   - Component classes reference

7. **Real-Time Updates**
   - WebSocket flow
   - Message format
   - JavaScript handlers

8. **Integration Points for Stack Management**
   - What needs to be added
   - Implementation order
   - Component list

9. **Performance Considerations**
   - Pagination size
   - HTMX debounce settings
   - Database optimization
   - WebSocket considerations

10. **Debugging Tips**
    - Development commands
    - URL references
    - Browser tools guidance
    - Log format

11. **Common Tasks**
    - How to add a new list page (5 steps)
    - How to add HTMX filtering (5 steps)
    - How to implement theme switching (3 steps)

**Start here for**: Quick lookups, visual understanding, debugging, and common task reference.

---

## Quick Navigation

### By User Role

**Software Architect**:
1. Read ARCHITECTURE_QUICK_REFERENCE.md first (overview)
2. Study GRAPHIUM_UI_ARCHITECTURE.md (comprehensive)
3. Review integration points in section 9

**Backend Developer**:
1. Study STACK_INTEGRATION_GUIDE.md sections 1-3 (model, storage, API)
2. Reference GRAPHIUM_UI_ARCHITECTURE.md section 6-7 (handlers)
3. Check ARCHITECTURE_QUICK_REFERENCE.md for patterns

**Frontend Developer**:
1. Study STACK_INTEGRATION_GUIDE.md sections 4-5 (templates, web handlers)
2. Reference GRAPHIUM_UI_ARCHITECTURE.md section 2, 4-5 (pages, styling)
3. Check ARCHITECTURE_QUICK_REFERENCE.md for HTMX/Templ patterns

**Full-Stack Developer**:
1. Start with ARCHITECTURE_QUICK_REFERENCE.md (overview)
2. Reference GRAPHIUM_UI_ARCHITECTURE.md for complete picture
3. Use STACK_INTEGRATION_GUIDE.md for implementation

### By Question

**Q: How does the UI work?**
A: GRAPHIUM_UI_ARCHITECTURE.md sections 1, 2, 4

**Q: What are all the API endpoints?**
A: GRAPHIUM_UI_ARCHITECTURE.md section 3

**Q: How do I add a new feature?**
A: ARCHITECTURE_QUICK_REFERENCE.md "Common Tasks" + STACK_INTEGRATION_GUIDE.md

**Q: How do I implement stack management?**
A: STACK_INTEGRATION_GUIDE.md (complete step-by-step)

**Q: What are the styling conventions?**
A: GRAPHIUM_UI_ARCHITECTURE.md section 5 + ARCHITECTURE_QUICK_REFERENCE.md "Styling"

**Q: How is data stored and related?**
A: GRAPHIUM_UI_ARCHITECTURE.md section 6 + ARCHITECTURE_QUICK_REFERENCE.md "Database & Models"

**Q: How are real-time updates handled?**
A: GRAPHIUM_UI_ARCHITECTURE.md section 4 + ARCHITECTURE_QUICK_REFERENCE.md "Real-Time Updates"

---

## Key Findings Summary

### Technology Stack
- **Frontend**: Templ (type-safe templates) + HTMX (AJAX) + Cytoscape.js (graphs)
- **Backend**: Go + Echo (web framework)
- **Database**: CouchDB (semantic graph)
- **Styling**: Custom CSS with variables (no framework)

### Current Features
- Dashboard with infrastructure overview
- Container management (list, detail, logs)
- Host management (list, detail, topology)
- Interactive graph visualization
- User authentication and management
- Real-time WebSocket updates

### Architecture Patterns
- Server-side template rendering (Templ)
- Lightweight AJAX (HTMX)
- Pagination with query parameters
- Form-based error handling
- Semantic data (JSON-LD)
- Role-based access control

### For Stack Management
- Create new model with JSON-LD format
- Implement storage layer (CRUDs)
- Create API handlers (REST endpoints)
- Create web handlers (form handlers)
- Add Templ components (UI pages)
- Register routes and add navigation

---

## Implementation Roadmap

### Phase 1: Core Stack Features (6-9 hours)
1. Create Stack model (models/stack.go)
2. Implement storage layer (internal/storage/stacks.go)
3. Create API handlers (internal/api/handlers_stacks.go)
4. Create web handlers (internal/web/handlers_stacks.go)
5. Add Templ components (templates.templ)
6. Register routes and navigation

### Phase 2: Enhanced Features (2-3 hours each, optional)
- Graph visualization integration
- WebSocket real-time updates
- YAML validation and syntax highlighting
- Service scaling interface
- Docker integration/deployment

---

## Reference Files

### Core Codebase
- `/home/opunix/graphium/internal/web/handlers.go` (600+ lines)
- `/home/opunix/graphium/internal/web/templates.templ` (2,492 lines)
- `/home/opunix/graphium/internal/api/server.go` (route setup)
- `/home/opunix/graphium/static/css/styles.css` (500+ lines)

### Data Models
- `/home/opunix/graphium/models/container.go`
- `/home/opunix/graphium/models/host.go`
- `/home/opunix/graphium/models/user.go`

### Storage
- `/home/opunix/graphium/internal/storage/containers.go`
- `/home/opunix/graphium/internal/storage/hosts.go`

### Handlers
- `/home/opunix/graphium/internal/api/handlers_containers.go`
- `/home/opunix/graphium/internal/api/handlers_hosts.go`
- `/home/opunix/graphium/internal/web/handlers_auth.go`
- `/home/opunix/graphium/internal/web/handlers_users.go`

---

## External Resources

- Templ Documentation: https://templ.guide
- HTMX Documentation: https://htmx.org
- Echo Framework: https://echo.labstack.com
- JSON-LD Specification: https://json-ld.org
- Schema.org: https://schema.org
- CouchDB: https://couchdb.apache.org
- Cytoscape.js: https://cytoscape.org

---

## Questions or Issues?

Refer to the appropriate document:
- Architecture questions → GRAPHIUM_UI_ARCHITECTURE.md
- Implementation questions → STACK_INTEGRATION_GUIDE.md
- Quick lookups → ARCHITECTURE_QUICK_REFERENCE.md

All three documents are cross-referenced and cover different aspects of the same system.

---

**Last Updated**: 2024-10-29
**Exploration Status**: Complete
**Thoroughness Level**: Medium (comprehensive coverage)
