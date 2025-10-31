# Graph Visualization Refactor: Executive Summary

**Goal**: Transform the graph visualization from host-centric to stack-centric, making stacks the primary visual elements with hosts and containers nested within them.

---

## Quick Links

- **Detailed Proposal**: [GRAPH_VISUALIZATION_REFACTOR_PROPOSAL.md](./GRAPH_VISUALIZATION_REFACTOR_PROPOSAL.md)
- **Visual Diagrams**: [GRAPH_VISUALIZATION_DIAGRAMS.md](./GRAPH_VISUALIZATION_DIAGRAMS.md)

---

## The Problem

Currently, the graph shows **hosts as primary nodes** with containers attached:

```
Host → Container 1
    → Container 2
    → Container 3
```

**Issues:**
- ✗ No visual representation of stacks/applications
- ✗ Hard to see which containers belong together
- ✗ Multi-host deployments not obvious
- ✗ Focus on infrastructure instead of applications

---

## The Solution

Transform to **stack-centric view** using Cytoscape.js compound nodes:

```
Stack (Application)
  ├── Host 1
  │   ├── Container A
  │   └── Container B
  └── Host 2
      └── Container C
```

**Benefits:**
- ✓ Application-centric view
- ✓ Multi-host topology immediately visible
- ✓ Logical grouping by stack
- ✓ Stack health at a glance

---

## View Modes

| Mode | Description | Use Case |
|------|-------------|----------|
| **Stack View** | Stacks as compound nodes with nested hosts/containers | Default - Application management |
| **Host View** | Legacy host-centric view | Infrastructure monitoring |
| **Hybrid View** | Stacks + orphaned containers | Migration, inventory |
| **Stack-Only** | Stack nodes only (collapsed) | High-level overview |

---

## Implementation Approach

### 1. Backend Changes

**New API Endpoint**: `/api/v1/graph/stack-view`

```go
// Returns graph data with compound node structure
{
  "nodes": [
    {"type": "stack", "id": "nginx-multihost", "label": "nginx-multihost"},
    {"type": "host", "id": "vm1", "parent": "nginx-multihost"},
    {"type": "container", "id": "nginx-1", "parent": "vm1"}
  ],
  "edges": [...]
}
```

**Key Methods:**
- `GetStackTopology(stackID)` - Get stack's complete deployment topology
- `FindOrphanedContainers()` - Find containers not in any stack
- `groupPlacementsByHost()` - Group container placements by host

### 2. Frontend Changes

**Cytoscape.js Configuration:**
- Enable compound nodes (parent-child relationships)
- Use `cose-bilkent` layout (better for hierarchical graphs)
- Add expand/collapse functionality for stack nodes
- Style stack nodes differently (compound appearance)

**UI Controls:**
- View mode selector dropdown
- "Show orphans" checkbox
- Expand/collapse all button
- Export graph button

### 3. Data Flow

```
1. User selects view mode
   ↓
2. API fetches stacks → deployments → hosts → containers
   ↓
3. Backend builds hierarchical graph structure
   ↓
4. Frontend renders with Cytoscape.js compound layout
   ↓
5. User can expand/collapse stacks interactively
```

---

## Key Features

### Expand/Collapse Stacks

```
Collapsed:                    Expanded:
╔═══════════════╗            ╔═══════════════════════════╗
║ nginx-multi   ║            ║ nginx-multi               ║
║ 3 containers  ║  [Click]   ║  ┌──────┐  ┌──────┐     ║
║ 3 hosts       ║  ──────►   ║  │Host1 │  │Host2 │     ║
╚═══════════════╝            ║  │• C1  │  │• C2  │     ║
                             ╚═══════════════════════════╝
```

### Interactive Tooltips

```
Hover over stack → Show:
  - Status
  - Container count
  - Host distribution
  - Deployment mode
  - Creation date
  - Quick actions (View, Stop, Restart)
```

### Real-time Updates

```
WebSocket events:
  - stack_deployed → Add stack node
  - stack_deleted → Remove stack node
  - container_added → Update stack container count
  - host_added → Update graph structure
```

---

## Timeline & Effort

| Phase | Duration | Tasks |
|-------|----------|-------|
| **Phase 1: Backend** | 3 days | API endpoints, storage methods, graph builder |
| **Phase 2: Frontend** | 4 days | Cytoscape config, UI controls, styling |
| **Phase 3: Testing** | 2 days | Unit tests, integration tests, UI tests |
| **Phase 4: UAT** | 3 days | Staging deployment, user feedback |
| **Phase 5: Production** | 1 day | Rollout, monitoring |

**Total: ~2 weeks** (10 working days)

---

## Migration Strategy

### Week 1: Add New View (Backward Compatible)
- Keep existing host-centric view as default
- Add new stack-centric view as "beta feature"
- Both views available via selector

### Week 2: Feature Parity & Testing
- Implement all view modes
- Add expand/collapse functionality
- Comprehensive testing
- Gather user feedback

### Week 3: Switch Default
- Make stack-centric view the default
- Host-centric view still available
- Monitor usage analytics

### Week 4+: Deprecation (Optional)
- If no users use host-only view, deprecate
- Otherwise, keep both views

---

## Quick Start (Development)

### 1. Create Feature Branch

```bash
git checkout -b feature/stack-centric-graph
```

### 2. Start with Backend

```bash
# Create new API endpoint
vim internal/api/handlers_graph_stack.go

# Add storage methods
vim internal/storage/graph.go

# Add tests
vim internal/api/handlers_graph_stack_test.go
```

### 3. Update Frontend

```bash
# Update Cytoscape configuration
vim internal/web/templates.templ

# Add CSS for compound nodes
vim static/css/graph.css
```

### 4. Test Locally

```bash
# Build and run
go build -o graphium-dev ./cmd/graphium
./graphium-dev server

# Open browser
open http://localhost:8095/web/graph
```

---

## Success Metrics

### Functionality
- ✅ All 4 view modes working
- ✅ Expand/collapse functional
- ✅ Real-time WebSocket updates
- ✅ Performance: <2s load time for 100 stacks

### User Experience
- ✅ 80% of users prefer stack view (via analytics)
- ✅ Positive user feedback (UAT survey)
- ✅ Reduced time to understand multi-host deployments
- ✅ Mobile responsive (tested on 3 screen sizes)

### Technical
- ✅ API response time: <500ms for 50 stacks
- ✅ No regressions in existing functionality
- ✅ 90%+ test coverage
- ✅ Zero production errors first week

---

## Files to Modify

### Backend
```
internal/api/handlers_graph.go             (add new endpoint)
internal/storage/graph.go                   (add helper methods)
internal/api/server.go                      (register new route)
internal/api/handlers_graph_test.go         (add tests)
```

### Frontend
```
internal/web/templates.templ                (update GraphPage)
static/css/graph.css                        (new file - styling)
internal/web/handlers.go                    (if needed)
```

### Models (if needed)
```
models/graph.go                             (new file - graph types)
```

---

## Risk Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| Performance degradation with large datasets | High | Lazy loading, caching, pagination |
| User confusion with new UI | Medium | Keep both views, add tooltips/help |
| Breaking existing integrations | High | New endpoint, keep old endpoint |
| Compound node layout issues | Medium | Extensive testing, fallback layouts |
| Mobile usability | Medium | Force simple view on small screens |

---

## Questions to Resolve

### Before Starting
1. **Default collapsed or expanded?**
   - Recommendation: Collapsed with localStorage preference

2. **Show stack dependencies?**
   - Recommendation: Yes, as dotted edges between stacks

3. **Mobile support?**
   - Recommendation: Force stack-only view on mobile

4. **Export functionality?**
   - Recommendation: Yes, add PNG/SVG export button

### During Development
5. **Legend needed?**
   - Recommendation: Yes, collapsible legend panel

6. **Filtering/search?**
   - Recommendation: Add stack name filter (Phase 2)

---

## Next Steps

### 1. Review & Approve (1 day)
- [ ] Read full proposal document
- [ ] Review visual diagrams
- [ ] Discuss with team
- [ ] Approve/modify approach

### 2. Create Tickets (1 day)
- [ ] Break down into Jira/GitHub issues
- [ ] Assign story points
- [ ] Set sprint goals

### 3. Setup Development (1 day)
- [ ] Create feature branch
- [ ] Set up local testing environment
- [ ] Create UI mockups (Figma/Sketch)

### 4. Begin Implementation (Week 1)
- [ ] Backend API endpoint
- [ ] Storage layer methods
- [ ] Unit tests

### 5. Continue... (Week 2+)
- [ ] Frontend implementation
- [ ] Integration tests
- [ ] UAT & feedback
- [ ] Production deployment

---

## Resources

- **Cytoscape.js Docs**: https://js.cytoscape.org/
- **Compound Nodes Guide**: https://js.cytoscape.org/#notation/compound-nodes
- **Layout Options**: https://js.cytoscape.org/#layouts/cose-bilkent
- **Current Implementation**: `/home/opunix/graphium/internal/api/handlers_graph.go`

---

## Contact

For questions about this proposal, contact the development team or review the detailed documentation in this repository.

**Last Updated**: 2025-10-31
