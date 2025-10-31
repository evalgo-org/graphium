# Graph Visualization: Before & After

## Current Architecture (Host-Centric)

```
┌─────────────────────────────────────────────────────────────┐
│                   GRAPH VISUALIZATION                        │
│                    (Current: Host View)                      │
└─────────────────────────────────────────────────────────────┘

          ┌──────────────────┐
          │  localhost       │ ◄────────────┐
          │  (Host)          │              │
          │  192.168.1.10    │              │
          └──────────────────┘              │
                   ▲                         │
                   │                         │ hosted_on
          ┌────────┼─────────┐              │
          │        │         │              │
   ┌──────┴───┐ ┌─┴──────┐ ┌┴───────────┐  │
   │ nginx-1  │ │ nginx-2│ │  redis-1   │  │
   │ (running)│ │(running)│ │ (running)  │  │
   └──────────┘ └────────┘ └────────────┘  │
                                             │
          ┌──────────────────┐              │
          │  vm1             │ ◄────────────┤
          │  (Host)          │              │
          │  192.168.122.11  │              │
          └──────────────────┘              │
                   ▲                         │
                   │                         │
          ┌────────┘                         │
          │                                  │
   ┌──────┴───┐                              │
   │ nginx-3  │                              │
   │ (running)│                              │
   └──────────┘                              │
                                             │
          ┌──────────────────┐              │
          │  vm2             │ ◄────────────┘
          │  (Host)          │
          │  192.168.122.167 │
          └──────────────────┘
                   ▲
                   │
          ┌────────┘
          │
   ┌──────┴───┐
   │ nginx-4  │
   │ (running)│
   └──────────┘

PROBLEMS:
✗ No visual grouping by application
✗ Hard to see which containers belong together
✗ Multi-host deployment not obvious
✗ Stack concept not visible
```

---

## Proposed Architecture (Stack-Centric)

```
┌─────────────────────────────────────────────────────────────┐
│                   GRAPH VISUALIZATION                        │
│                    (New: Stack View)                         │
└─────────────────────────────────────────────────────────────┘

┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
┃  nginx-multihost (Stack)                                    ┃
┃  Status: running | Containers: 3 | Hosts: 3                 ┃
┃  Mode: multi-host | Strategy: manual                        ┃
┃                                                              ┃
┃  ┌──────────────────────┐  ┌──────────────────────┐        ┃
┃  │  localhost           │  │  vm1                 │        ┃
┃  │  192.168.1.10        │  │  192.168.122.11      │        ┃
┃  │                      │  │                      │        ┃
┃  │  ┌────────────────┐  │  │  ┌────────────────┐ │        ┃
┃  │  │  nginx-1       │  │  │  │  nginx-2       │ │        ┃
┃  │  │  :8081         │  │  │  │  :8082         │ │        ┃
┃  │  │  (running)     │  │  │  │  (running)     │ │        ┃
┃  │  └────────────────┘  │  │  └────────────────┘ │        ┃
┃  └──────────────────────┘  └──────────────────────┘        ┃
┃                                                              ┃
┃  ┌──────────────────────┐                                   ┃
┃  │  vm2                 │                                   ┃
┃  │  192.168.122.167     │                                   ┃
┃  │                      │                                   ┃
┃  │  ┌────────────────┐  │                                   ┃
┃  │  │  nginx-3       │  │                                   ┃
┃  │  │  :8083         │  │                                   ┃
┃  │  │  (running)     │  │                                   ┃
┃  │  └────────────────┘  │                                   ┃
┃  └──────────────────────┘                                   ┃
┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛

┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
┃  redis-cache (Stack)                                        ┃
┃  Status: running | Containers: 1 | Hosts: 1                 ┃
┃  Mode: single-host                                          ┃
┃                                                              ┃
┃  ┌──────────────────────┐                                   ┃
┃  │  localhost           │                                   ┃
┃  │  192.168.1.10        │                                   ┃
┃  │                      │                                   ┃
┃  │  ┌────────────────┐  │                                   ┃
┃  │  │  redis-1       │  │                                   ┃
┃  │  │  :6379         │  │                                   ┃
┃  │  │  (running)     │  │                                   ┃
┃  │  └────────────────┘  │                                   ┃
┃  └──────────────────────┘                                   ┃
┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛

BENEFITS:
✓ Clear application grouping
✓ Multi-host deployment immediately visible
✓ Stack status at a glance
✓ Logical hierarchy: Stack → Host → Container
✓ Deployment strategy visible
```

---

## View Mode Comparison

### 1. Stack View (Default)

```
╔══════════════╗    ╔══════════════╗
║   Stack A    ║    ║   Stack B    ║
║              ║    ║              ║
║ ┌──────────┐ ║    ║ ┌──────────┐ ║
║ │  Host 1  │ ║    ║ │  Host 2  │ ║
║ │ ┌──────┐ │ ║    ║ │ ┌──────┐ │ ║
║ │ │Cont1 │ │ ║    ║ │ │Cont3 │ │ ║
║ │ └──────┘ │ ║    ║ │ └──────┘ │ ║
║ └──────────┘ ║    ║ └──────────┘ ║
╚══════════════╝    ╚══════════════╝

Focus: Applications
Use Case: Application management
```

### 2. Host View (Legacy)

```
┌────────────┐    ┌────────────┐
│   Host 1   │    │   Host 2   │
└─────┬──────┘    └─────┬──────┘
      │                 │
  ┌───┴───┐         ┌───┴───┐
  │       │         │       │
┌─┴─┐   ┌─┴─┐     ┌─┴─┐   ┌─┴─┐
│C1 │   │C2 │     │C3 │   │C4 │
└───┘   └───┘     └───┘   └───┘

Focus: Infrastructure
Use Case: Host resource monitoring
```

### 3. Hybrid View

```
╔══════════════╗    ┌────────────┐
║   Stack A    ║    │   Host 3   │
║              ║    └─────┬──────┘
║ ┌──────────┐ ║          │
║ │  Host 1  │ ║      ┌───┴───┐
║ │ ┌──────┐ │ ║      │Orphan │
║ │ │Cont1 │ │ ║      │Cont5  │
║ │ └──────┘ │ ║      └───────┘
║ └──────────┘ ║
╚══════════════╝

Focus: Both stacks + orphans
Use Case: Migration/inventory
```

### 4. Stack-Only View

```
╔═══════╗    ╔═══════╗    ╔═══════╗
║Stack A║    ║Stack B║    ║Stack C║
║ 3c 2h ║    ║ 1c 1h ║    ║ 5c 3h ║
╚═══════╝    ╚═══════╝    ╚═══════╝
     │            │            │
     └────────────┴────────────┘
          depends_on

Focus: Stack relationships
Use Case: High-level overview
```

---

## Interactive Features

### Expand/Collapse

```
COLLAPSED STATE:
╔══════════════════════════════╗
║ nginx-multihost [+]          ║
║ 3 containers, 3 hosts        ║
╚══════════════════════════════╝

                ⬇ Click to expand

EXPANDED STATE:
╔══════════════════════════════════════════╗
║ nginx-multihost [-]                      ║
║                                          ║
║  ┌──────────┐  ┌──────────┐            ║
║  │ Host 1   │  │ Host 2   │            ║
║  │ • Cont1  │  │ • Cont2  │            ║
║  └──────────┘  └──────────┘            ║
╚══════════════════════════════════════════╝
```

### Tooltip on Hover

```
         ┌─────────────────────────┐
         │ nginx-multihost         │
         ├─────────────────────────┤
         │ Status: running         │
         │ Containers: 3           │
         │ Hosts: 3                │
         │ Mode: multi-host        │
         │ Created: 2 days ago     │
         │                         │
         │ [View Details] [Stop]   │
         └─────────────────────────┘
              ▲
              │
    ╔═══════════════╗
    ║ Stack Node    ║ ◄── Hover here
    ╚═══════════════╝
```

---

## Data Flow

```
┌─────────────┐
│   Browser   │
└──────┬──────┘
       │ GET /api/v1/graph/stack-view?view=stack
       ▼
┌─────────────────────┐
│   API Handler       │
│  (GetGraphData      │
│   StackView)        │
└──────┬──────────────┘
       │
       ├─► ListStacks()
       │     └─► Stacks: [stack1, stack2]
       │
       ├─► GetDeploymentsByStackID(stack1)
       │     └─► DeploymentState with Placements
       │
       ├─► GetHost(host1), GetHost(host2)
       │     └─► Host metadata
       │
       └─► GetContainer(cont1), GetContainer(cont2)
             └─► Container details

       │ Assemble GraphData
       ▼
┌─────────────────────┐
│  {                  │
│    nodes: [         │
│      {type:"stack"} │ ◄── Stack nodes
│      {type:"host",  │ ◄── Host nodes (parent: stackID)
│       parent:"..."} │
│      {type:"cont",  │ ◄── Container nodes (parent: hostID)
│       parent:"..."} │
│    ],              │
│    edges: [...]    │
│  }                 │
└──────┬──────────────┘
       │
       ▼
┌─────────────────────┐
│  Cytoscape.js       │
│  (Compound Layout)  │
│                     │
│  - Render nodes     │
│  - Nest children    │
│  - Apply styling    │
│  - Calculate layout │
└─────────────────────┘
```

---

## Layout Algorithm Comparison

### Current: Force-Directed (cose)

```
   Host1         Host2
     │             │
   ┌─┴─┐         ┌─┴─┐
  C1  C2        C3  C4

✓ Good for showing relationships
✗ Poor for hierarchical data
✗ Containers can overlap
```

### Proposed: Hierarchical Compound (cose-bilkent)

```
╔═══════════════╗  ╔═══════════════╗
║ Stack1        ║  ║ Stack2        ║
║ ┌───────────┐ ║  ║ ┌───────────┐ ║
║ │ Host1     │ ║  ║ │ Host2     │ ║
║ │ C1 C2     │ ║  ║ │ C3 C4     │ ║
║ └───────────┘ ║  ║ └───────────┘ ║
╚═══════════════╝  ╚═══════════════╝

✓ Clear hierarchy
✓ No overlapping
✓ Natural grouping
✓ Better use of space
```

---

## Mobile/Responsive View

### Desktop (>1200px)

```
┌─────────────────────────────────────────────┐
│  [Stack View] [Host View] [Hybrid] [Stack-] │
│  ☐ Show orphans                             │
├─────────────────────────────────────────────┤
│ ╔═══════╗  ╔═══════╗  ╔═══════╗           │
│ ║Stack A║  ║Stack B║  ║Stack C║           │
│ ║       ║  ║       ║  ║       ║           │
│ ╚═══════╝  ╚═══════╝  ╚═══════╝           │
│                                             │
│ ╔═══════╗  ╔═══════╗                      │
│ ║Stack D║  ║Stack E║                      │
│ ╚═══════╝  ╚═══════╝                      │
└─────────────────────────────────────────────┘
```

### Tablet (768px-1200px)

```
┌─────────────────────────┐
│  View: [Stack ▼]        │
├─────────────────────────┤
│ ╔═══════╗  ╔═══════╗   │
│ ║Stack A║  ║Stack B║   │
│ ╚═══════╝  ╚═══════╝   │
│                         │
│ ╔═══════╗  ╔═══════╗   │
│ ║Stack C║  ║Stack D║   │
│ ╚═══════╝  ╚═══════╝   │
└─────────────────────────┘
```

### Mobile (<768px)

```
┌──────────────┐
│ Stacks       │
├──────────────┤
│ ╔══════════╗ │
│ ║ Stack A  ║ │
│ ║ 3c 2h    ║ │
│ ╚══════════╝ │
│              │
│ ╔══════════╗ │
│ ║ Stack B  ║ │
│ ║ 1c 1h    ║ │
│ ╚══════════╝ │
│              │
│ ╔══════════╗ │
│ ║ Stack C  ║ │
│ ║ 5c 3h    ║ │
│ ╚══════════╝ │
└──────────────┘
  (List view,
   tap to drill)
```

---

## Performance Optimization

### Lazy Loading

```
┌──────────────────────┐
│ Initial Load         │
└──────────────────────┘
         │
         ▼
  Load Stack Nodes Only
         │
         ▼
┌──────────────────────┐
│ ╔═════╗ ╔═════╗     │
│ ║Stk A║ ║Stk B║     │
│ ╚═════╝ ╚═════╝     │
└──────────────────────┘
         │
    User Expands Stack A
         │
         ▼
  Load Hosts + Containers
    for Stack A only
         │
         ▼
┌──────────────────────┐
│ ╔═══════════╗ ╔═══╗ │
│ ║ Stk A     ║ ║StB║ │
│ ║ ┌───────┐ ║ ╚═══╝ │
│ ║ │Host1  │ ║       │
│ ║ │C1 C2  │ ║       │
│ ╚═══════════╝       │
└──────────────────────┘
```

### Caching Strategy

```
┌──────────────────────────────────┐
│     Cache Layer (Redis)          │
├──────────────────────────────────┤
│ Key: graph:stack-view:v1         │
│ TTL: 30 seconds                  │
│ Data: {nodes: [...], edges:[...]}│
└──────────────────────────────────┘
         ▲           │
         │           ▼
    Write Cache   Read Cache
         │           │
┌────────┴───────────┴────────┐
│   Graph Data Generator       │
└──────────────────────────────┘
```
