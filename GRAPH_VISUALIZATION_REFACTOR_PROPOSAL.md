# Graph Visualization Refactoring Proposal: Stack-Centric View

**Date:** 2025-10-31
**Status:** Proposal
**Author:** Claude Code

## Executive Summary

Refactor the graph visualization from **host-centric** to **stack-centric**, making stacks the primary nodes in the graph. Each stack node will display the hosts it's deployed to and the containers within it, providing an application-centric view of the infrastructure.

---

## Current Architecture (Host-Centric)

### Current Visualization Structure

```
Host (Node)
  ├── Container 1 (Node) --[hosted_on]--> Host
  ├── Container 2 (Node) --[hosted_on]--> Host
  └── Container 3 (Node) --[hosted_on]--> Host

Container --[depends_on]--> Container
```

**Current Graph Data Model:**
```javascript
{
  "nodes": [
    {"type": "host", "id": "localhost-docker", "label": "localhost"},
    {"type": "container", "id": "nginx-1", "label": "nginx-1"},
    {"type": "container", "id": "nginx-2", "label": "nginx-2"}
  ],
  "edges": [
    {"source": "nginx-1", "target": "localhost-docker", "type": "hosted_on"},
    {"source": "nginx-2", "target": "localhost-docker", "type": "hosted_on"}
  ]
}
```

**Issues with Current Approach:**
- Focus is on infrastructure (hosts) rather than applications (stacks)
- Difficult to understand multi-host stack deployments at a glance
- No visibility into which containers belong to which application/stack
- Orphaned containers not associated with stacks are hidden in the noise

---

## Proposed Architecture (Stack-Centric)

### New Visualization Structure

```
Stack (Compound Node)
  ├── Host 1 (Child Node)
  │     ├── Container A (Grandchild Node)
  │     └── Container B (Grandchild Node)
  │
  └── Host 2 (Child Node)
        └── Container C (Grandchild Node)

Stack --[depends_on]--> Stack (if stack-level dependencies exist)
Container --[depends_on]--> Container (cross-container dependencies)
```

**New Graph Data Model:**
```javascript
{
  "nodes": [
    {
      "type": "stack",
      "id": "nginx-multihost",
      "label": "nginx-multihost",
      "status": "running",
      "containerCount": 3,
      "hostCount": 3,
      "metadata": {
        "deploymentMode": "multi-host",
        "placementStrategy": "manual"
      }
    },
    {
      "type": "host",
      "id": "localhost-docker",
      "label": "localhost",
      "parent": "nginx-multihost",  // Stack parent for compound layout
      "status": "active"
    },
    {
      "type": "container",
      "id": "nginx-1",
      "label": "nginx-1",
      "parent": "localhost-docker",  // Host parent for nesting
      "stack": "nginx-multihost",
      "status": "running"
    }
  ],
  "edges": [
    {"source": "nginx-1", "target": "nginx-2", "type": "depends_on"},
    // hosted_on edges implicit via parent relationship
  ]
}
```

### View Modes

**1. Stack View (Default):**
- Stacks as compound nodes (expandable/collapsible)
- Hosts nested within stacks
- Containers nested within hosts
- Shows multi-host deployment topology clearly

**2. Host View (Legacy):**
- Hosts as primary nodes
- Containers attached to hosts
- Backward compatibility with current view

**3. Hybrid View:**
- Show stacks AND orphaned containers
- Stacks collapsed by default
- Orphaned containers (not in stacks) shown separately

**4. Stack-Only View:**
- Only show stack nodes
- Hide container details
- High-level application overview

---

## Implementation Plan

### Phase 1: Data Model Changes

#### 1.1 Update GraphNode Structure

**File:** `internal/api/handlers_graph.go`

```go
type GraphNodeData struct {
    ID             string            `json:"id"`
    Label          string            `json:"label"`
    Type           string            `json:"type"` // "stack", "host", "container"
    Status         string            `json:"status,omitempty"`

    // Stack-specific fields
    ContainerCount int               `json:"containerCount,omitempty"`
    HostCount      int               `json:"hostCount,omitempty"`
    DeploymentMode string            `json:"deploymentMode,omitempty"`

    // Host-specific fields
    IP             string            `json:"ip,omitempty"`
    CPU            int               `json:"cpu,omitempty"`
    Memory         int64             `json:"memory,omitempty"`
    Location       string            `json:"location,omitempty"`

    // Container-specific fields
    Image          string            `json:"image,omitempty"`
    Stack          string            `json:"stack,omitempty"` // Stack ID this container belongs to

    // Cytoscape.js compound node support
    Parent         string            `json:"parent,omitempty"` // Parent node ID for nesting

    Metadata       map[string]string `json:"metadata,omitempty"`
}
```

#### 1.2 Create New API Endpoint

**File:** `internal/api/handlers_graph.go`

```go
// GetGraphDataStackView returns the stack-centric graph visualization
// @Summary Get stack-centric graph data
// @Description Get graph with stacks as primary nodes, hosts nested within
// @Tags Graph
// @Accept json
// @Produce json
// @Param view query string false "View mode" Enums(stack, host, hybrid, stack-only)
// @Param expanded query string false "Comma-separated stack IDs to show expanded"
// @Success 200 {object} GraphData
// @Router /graph/stack-view [get]
func (s *Server) GetGraphDataStackView(c echo.Context) error {
    viewMode := c.QueryParam("view")
    if viewMode == "" {
        viewMode = "stack" // Default to stack view
    }

    expandedStacks := parseExpandedStacks(c.QueryParam("expanded"))

    switch viewMode {
    case "stack":
        return s.getStackCentricGraph(c, expandedStacks, false)
    case "host":
        return s.GetGraphData(c) // Use existing host-centric view
    case "hybrid":
        return s.getHybridGraph(c, expandedStacks)
    case "stack-only":
        return s.getStackOnlyGraph(c)
    default:
        return BadRequestError("Invalid view mode", "")
    }
}

// getStackCentricGraph builds stack-centric graph data
func (s *Server) getStackCentricGraph(c echo.Context, expandedStacks map[string]bool, includeOrphans bool) error {
    graphData := GraphData{
        Nodes: make([]GraphNode, 0),
        Edges: make([]GraphEdge, 0),
    }

    // 1. Get all stacks
    stacks, err := s.storage.ListStacks(nil)
    if err != nil {
        return InternalError("Failed to list stacks", err.Error())
    }

    // 2. For each stack, get deployment state
    for _, stack := range stacks {
        deployments, err := s.storage.GetDeploymentsByStackID(stack.ID)
        if err != nil || len(deployments) == 0 {
            continue // Skip stacks without deployments
        }

        deployment := deployments[len(deployments)-1] // Get latest deployment

        // Add stack node
        stackNode := GraphNode{
            Data: GraphNodeData{
                ID:             stack.ID,
                Label:          stack.Name,
                Type:           "stack",
                Status:         stack.Status,
                ContainerCount: len(deployment.Placements),
                HostCount:      countUniqueHosts(deployment.Placements),
                DeploymentMode: stack.Deployment.Mode,
                Metadata: map[string]string{
                    "description": stack.Description,
                },
            },
        }
        graphData.Nodes = append(graphData.Nodes, stackNode)

        // Group placements by host
        hostContainers := groupPlacementsByHost(deployment.Placements)

        // Add host nodes (as children of stack)
        for hostID, containers := range hostContainers {
            host, err := s.storage.GetHost(hostID)
            if err != nil {
                continue // Skip missing hosts
            }

            hostNode := GraphNode{
                Data: GraphNodeData{
                    ID:       hostID,
                    Label:    host.Name,
                    Type:     "host",
                    Status:   host.Status,
                    IP:       host.IPAddress,
                    Location: host.Datacenter,
                    Parent:   stack.ID, // Nest within stack
                },
            }
            graphData.Nodes = append(graphData.Nodes, hostNode)

            // Add container nodes (as children of host)
            for containerName, placement := range containers {
                container, err := s.storage.GetContainer(placement.ContainerID)
                if err != nil {
                    continue // Skip missing containers
                }

                containerNode := GraphNode{
                    Data: GraphNodeData{
                        ID:     container.ID,
                        Label:  container.Name,
                        Type:   "container",
                        Status: container.Status,
                        Image:  container.Image,
                        Stack:  stack.ID,
                        Parent: hostID, // Nest within host
                    },
                }
                graphData.Nodes = append(graphData.Nodes, containerNode)

                // Add dependency edges
                for _, depID := range container.DependsOn {
                    edge := GraphEdge{
                        Data: GraphEdgeData{
                            ID:     container.ID + "-depends-" + depID,
                            Source: container.ID,
                            Target: depID,
                            Label:  "depends on",
                            Type:   "depends_on",
                        },
                    }
                    graphData.Edges = append(graphData.Edges, edge)
                }
            }
        }
    }

    // 3. Optionally include orphaned containers (not in any stack)
    if includeOrphans {
        orphans := s.findOrphanedContainers(stacks)
        // ... add orphan nodes with no parent
    }

    return c.JSON(http.StatusOK, graphData)
}

// Helper functions
func countUniqueHosts(placements map[string]*models.ContainerPlacement) int {
    hosts := make(map[string]bool)
    for _, p := range placements {
        hosts[p.HostID] = true
    }
    return len(hosts)
}

func groupPlacementsByHost(placements map[string]*models.ContainerPlacement) map[string]map[string]*models.ContainerPlacement {
    result := make(map[string]map[string]*models.ContainerPlacement)
    for containerName, placement := range placements {
        if result[placement.HostID] == nil {
            result[placement.HostID] = make(map[string]*models.ContainerPlacement)
        }
        result[placement.HostID][containerName] = placement
    }
    return result
}
```

---

### Phase 2: Frontend Changes

#### 2.1 Update Cytoscape.js Configuration

**File:** `internal/web/templates.templ` (GraphPage template)

Add support for compound nodes (parent-child nesting):

```javascript
const cy = cytoscape({
    container: document.getElementById('cy'),

    style: [
        // Stack nodes (compound)
        {
            selector: 'node[type="stack"]',
            style: {
                'background-color': '#3b82f6',
                'background-opacity': 0.1,
                'border-width': 3,
                'border-color': '#3b82f6',
                'border-style': 'solid',
                'shape': 'roundrectangle',
                'label': 'data(label)',
                'text-valign': 'top',
                'text-halign': 'center',
                'padding': '20px',
                'font-size': '18px',
                'font-weight': 'bold',
                'color': '#3b82f6'
            }
        },

        // Host nodes (nested in stacks)
        {
            selector: 'node[type="host"]',
            style: {
                'background-color': '#8b5cf6',
                'background-opacity': 0.2,
                'border-width': 2,
                'border-color': '#8b5cf6',
                'shape': 'roundrectangle',
                'label': 'data(label)',
                'padding': '15px',
                'font-size': '14px',
                'color': '#8b5cf6'
            }
        },

        // Container nodes (nested in hosts)
        {
            selector: 'node[type="container"]',
            style: {
                'background-color': '#10b981',
                'border-width': 2,
                'border-color': '#059669',
                'shape': 'ellipse',
                'label': 'data(label)',
                'width': '60px',
                'height': '60px',
                'font-size': '12px'
            }
        },

        // Running status
        {
            selector: 'node[status="running"]',
            style: {
                'border-color': '#10b981',
                'background-color': '#10b981'
            }
        },

        // Stopped status
        {
            selector: 'node[status="stopped"]',
            style: {
                'border-color': '#6b7280',
                'background-color': '#6b7280',
                'opacity': 0.6
            }
        },

        // Edges
        {
            selector: 'edge[type="depends_on"]',
            style: {
                'width': 2,
                'line-color': '#f59e0b',
                'target-arrow-color': '#f59e0b',
                'target-arrow-shape': 'triangle',
                'curve-style': 'bezier',
                'label': 'data(label)',
                'font-size': '10px',
                'text-rotation': 'autorotate'
            }
        }
    ],

    layout: {
        name: 'cose-bilkent', // Better for hierarchical compound graphs
        animate: true,
        animationDuration: 500,
        nodeDimensionsIncludeLabels: true,
        // Compound node layout options
        idealEdgeLength: 100,
        nodeRepulsion: 8000,
        nestingFactor: 0.1,
        gravity: 0.25,
        numIter: 2500,
        tile: true,
        tilingPaddingVertical: 10,
        tilingPaddingHorizontal: 10
    }
});
```

#### 2.2 Add View Mode Selector

Add UI controls to switch between view modes:

```html
<div class="graph-controls">
    <div class="graph-control-group">
        <label for="view-mode">View Mode:</label>
        <select id="view-mode" name="view-mode" onchange="switchViewMode(this.value)">
            <option value="stack" selected>Stack View</option>
            <option value="host">Host View</option>
            <option value="hybrid">Hybrid View</option>
            <option value="stack-only">Stack Overview</option>
        </select>
    </div>

    <div class="graph-control-group">
        <label>
            <input type="checkbox" id="show-orphans" onchange="toggleOrphans(this.checked)">
            Show containers without stacks
        </label>
    </div>
</div>

<script>
function switchViewMode(mode) {
    const showOrphans = document.getElementById('show-orphans').checked;
    const expanded = getExpandedStackIds(); // Track which stacks user expanded

    fetch(`/api/v1/graph/stack-view?view=${mode}&orphans=${showOrphans}&expanded=${expanded}`)
        .then(response => response.json())
        .then(data => {
            updateGraph(data);
        });
}

function toggleOrphans(show) {
    const mode = document.getElementById('view-mode').value;
    switchViewMode(mode);
}

// Allow users to expand/collapse stack nodes
cy.on('tap', 'node[type="stack"]', function(evt) {
    const node = evt.target;
    const stackId = node.id();

    if (node.hasClass('collapsed')) {
        // Expand: show children
        node.removeClass('collapsed');
        cy.nodes(`[parent="${stackId}"]`).style('display', 'element');
        trackExpanded(stackId, true);
    } else {
        // Collapse: hide children
        node.addClass('collapsed');
        cy.nodes(`[parent="${stackId}"]`).style('display', 'none');
        trackExpanded(stackId, false);
    }

    cy.layout({name: 'cose-bilkent'}).run();
});
</script>
```

---

### Phase 3: Backend Storage Methods

#### 3.1 Add Helper Methods

**File:** `internal/storage/graph.go`

```go
// GetStackTopology returns the complete topology of a stack including all hosts and containers.
func (s *Storage) GetStackTopology(stackID string) (*StackTopology, error) {
    // Get stack
    stack, err := s.GetStack(stackID)
    if err != nil {
        return nil, err
    }

    // Get deployment state
    deployments, err := s.GetDeploymentsByStackID(stackID)
    if err != nil || len(deployments) == 0 {
        return &StackTopology{Stack: stack, Hosts: make(map[string]*StackHostTopology)}, nil
    }

    deployment := deployments[len(deployments)-1]

    // Build topology
    topology := &StackTopology{
        Stack:  stack,
        Hosts:  make(map[string]*StackHostTopology),
    }

    // Group by host
    for containerName, placement := range deployment.Placements {
        if topology.Hosts[placement.HostID] == nil {
            host, _ := s.GetHost(placement.HostID)
            topology.Hosts[placement.HostID] = &StackHostTopology{
                Host:       host,
                Containers: make([]*models.Container, 0),
            }
        }

        container, err := s.GetContainer(placement.ContainerID)
        if err == nil {
            topology.Hosts[placement.HostID].Containers = append(
                topology.Hosts[placement.HostID].Containers,
                container,
            )
        }
    }

    return topology, nil
}

// FindOrphanedContainers returns containers that don't belong to any stack.
func (s *Storage) FindOrphanedContainers() ([]*models.Container, error) {
    // Get all containers
    allContainers, err := s.ListContainers(nil)
    if err != nil {
        return nil, err
    }

    // Get all stacks and their containers
    stacks, err := s.ListStacks(nil)
    if err != nil {
        return nil, err
    }

    stackContainers := make(map[string]bool)
    for _, stack := range stacks {
        deployments, err := s.GetDeploymentsByStackID(stack.ID)
        if err != nil || len(deployments) == 0 {
            continue
        }

        deployment := deployments[len(deployments)-1]
        for _, placement := range deployment.Placements {
            stackContainers[placement.ContainerID] = true
        }
    }

    // Find containers not in any stack
    orphans := make([]*models.Container, 0)
    for _, container := range allContainers {
        if !stackContainers[container.ID] {
            orphans = append(orphans, container)
        }
    }

    return orphans, nil
}

// StackTopology represents the complete view of a stack's deployment.
type StackTopology struct {
    Stack *models.Stack
    Hosts map[string]*StackHostTopology
}

// StackHostTopology represents a host and its containers within a stack.
type StackHostTopology struct {
    Host       *models.Host
    Containers []*models.Container
}
```

---

### Phase 4: CSS Styling

**File:** `static/css/graph.css` (new file)

```css
/* Stack View Specific Styles */
.graph-view-stack .stack-node {
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    border-radius: 12px;
}

.graph-view-stack .host-node {
    background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%);
}

.graph-view-stack .container-node {
    background: linear-gradient(135deg, #4facfe 0%, #00f2fe 100%);
}

/* Collapsed stack indicator */
.stack-node.collapsed::after {
    content: '[+]';
    position: absolute;
    top: 5px;
    right: 10px;
    font-size: 18px;
    color: #fff;
}

.stack-node:not(.collapsed)::after {
    content: '[-]';
    position: absolute;
    top: 5px;
    right: 10px;
    font-size: 18px;
    color: #fff;
}

/* Tooltips */
.graph-tooltip {
    position: absolute;
    background: rgba(0, 0, 0, 0.9);
    color: white;
    padding: 10px;
    border-radius: 6px;
    font-size: 12px;
    pointer-events: none;
    z-index: 1000;
}

.graph-tooltip h4 {
    margin: 0 0 8px 0;
    font-size: 14px;
    border-bottom: 1px solid #444;
    padding-bottom: 4px;
}

.graph-tooltip .stat {
    display: flex;
    justify-content: space-between;
    margin: 4px 0;
}

.graph-tooltip .stat-label {
    font-weight: bold;
    margin-right: 10px;
}
```

---

## Migration Strategy

### Step 1: Backward Compatibility (Week 1)
- Keep existing `/api/v1/graph` endpoint (host-centric)
- Add new `/api/v1/graph/stack-view` endpoint (stack-centric)
- Add view mode selector in UI (defaults to host view)

### Step 2: Feature Parity (Week 2)
- Implement all view modes (stack, host, hybrid, stack-only)
- Add expand/collapse functionality
- Add tooltips and interactive features
- Comprehensive testing

### Step 3: Switch Default (Week 3)
- Change default view mode to "hybrid" (shows stacks + orphans)
- Gather user feedback
- Performance optimization for large deployments

### Step 4: Deprecation (Week 4+)
- Mark host-only view as "legacy"
- Eventually deprecate if no users need it

---

## Benefits

### For Operators
- **Application-centric view**: See deployments as applications, not infrastructure
- **Multi-host clarity**: Instantly see which hosts a stack spans
- **Deployment topology**: Visual representation matches deployment architecture
- **Easier troubleshooting**: Quickly identify which containers are in which stack

### For Developers
- **Logical grouping**: Containers grouped by application/stack
- **Dependency visibility**: See inter-container dependencies within context
- **Stack health**: At-a-glance stack status and container distribution

### For Management
- **Resource allocation**: See how applications are distributed across infrastructure
- **Capacity planning**: Understand multi-host resource usage per application
- **Cost attribution**: Map infrastructure costs to applications/stacks

---

## Performance Considerations

### Optimization Strategies

1. **Lazy Loading**: Only load detailed container data when stack is expanded
2. **Caching**: Cache deployment topologies with 30-second TTL
3. **Progressive Rendering**: Render stack nodes first, then expand children
4. **Virtualization**: For 100+ stacks, implement viewport-based rendering
5. **WebSocket Updates**: Real-time updates for stack/container state changes

### Scalability Limits

| Scale | Nodes | Performance | Recommendation |
|-------|-------|-------------|----------------|
| Small | <50 | Excellent | All view modes |
| Medium | 50-200 | Good | Use stack-only view for overview |
| Large | 200-500 | Acceptable | Collapse stacks by default |
| Enterprise | 500+ | Requires optimization | Pagination + filtering |

---

## Alternative Approaches Considered

### Option A: Separate Stack and Infrastructure Views
- **Pros**: Cleaner separation, simpler implementation
- **Cons**: Users need to switch between pages, lose context

### Option B: Stack List with Drill-Down
- **Pros**: Scales better for large deployments
- **Cons**: Loses graph visualization benefits

### Option C: 3D Visualization
- **Pros**: More information density, "cool factor"
- **Cons**: Complexity, accessibility issues, steep learning curve

**Decision**: Implement the proposed compound node approach (Option D) because it:
- Maintains graph visualization paradigm users are familiar with
- Supports both views with minimal code changes
- Leverages Cytoscape.js compound node features
- Provides progressive disclosure (expand/collapse)

---

## Testing Plan

### Unit Tests
- Helper functions (countUniqueHosts, groupPlacementsByHost)
- Orphan detection logic
- Topology builders

### Integration Tests
- API endpoint responses for different view modes
- Graph data structure validation
- Edge cases (empty stacks, missing hosts, orphaned containers)

### UI Tests
- View mode switching
- Expand/collapse functionality
- Layout correctness
- Performance with large datasets

### User Acceptance Testing
- Deploy to staging environment
- Gather feedback from 5-10 operators
- Iterate based on feedback

---

## Timeline

| Phase | Duration | Deliverables |
|-------|----------|--------------|
| Phase 1: Backend | 3 days | New API endpoints, storage methods |
| Phase 2: Frontend | 4 days | UI controls, Cytoscape config, styling |
| Phase 3: Testing | 2 days | Unit + integration tests |
| Phase 4: UAT | 3 days | Staging deployment, feedback |
| Phase 5: Production | 1 day | Rollout, monitoring |

**Total: ~2 weeks**

---

## Open Questions

1. **Collapsed by Default?**: Should stacks be collapsed or expanded by default?
   - Recommendation: Collapsed, with user preference saved in localStorage

2. **Stack Dependencies?**: Should we show dependencies between stacks?
   - Recommendation: Yes, as dotted edges between stack nodes

3. **Legend?**: Need visual legend for node types and statuses?
   - Recommendation: Yes, add collapsible legend panel

4. **Mobile Support?**: How should this render on mobile devices?
   - Recommendation: Force stack-only view on small screens, disable nesting

5. **Export Feature?**: Should users be able to export graph as PNG/SVG?
   - Recommendation: Yes, add export button (Cytoscape.js supports this)

---

## Next Steps

1. **Review this proposal** with the team
2. **Prioritize features** (MVP vs nice-to-have)
3. **Create detailed tickets** for each phase
4. **Set up feature branch** for development
5. **Design UI mockups** for review

---

## References

- [Cytoscape.js Compound Nodes](https://js.cytoscape.org/#notation/compound-nodes)
- [Cytoscape.js Layouts](https://js.cytoscape.org/#layouts)
- [Current Graph Implementation](/home/opunix/graphium/internal/api/handlers_graph.go)
- [Stack Model](/home/opunix/graphium/models/stack.go)
- [Deployment State Model](/home/opunix/graphium/models/stack_deployment.go)
