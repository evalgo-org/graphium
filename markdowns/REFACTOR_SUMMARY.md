# Agent-Based Deployment: Quick Summary

## The Problem (in 2 sentences)

**Current:** Server tries to deploy/delete containers by connecting directly to Docker on all hosts via `tcp://host:2375`, which fails for vm1 and vm2 because Docker isn't exposed.

**Result:** Stack deployments only work on localhost, and deleting stacks leaves orphaned containers on remote hosts.

## The Solution (in 2 sentences)

**Proposed:** Agents pull deployment tasks from the server and execute them locally using their own Docker socket access.

**Result:** Secure, scalable, and works across all hosts without exposing Docker ports.

## Architecture Change

### BEFORE (Broken)
```
┌─────────────────────────────┐
│   Server (Graphium)         │
│                             │
│  handlers_stacks.go:112     │
│  tcp://vm1:2375  ────────X  │  ❌ Connection refused
│  tcp://vm2:2375  ────────X  │  ❌ Connection refused
│  unix://localhost  ──────✓  │  ✅ Works (local only)
└─────────────────────────────┘
```

### AFTER (Fixed)
```
┌─────────────────────────────┐
│   Server (Graphium)         │
│                             │
│  Creates AgentTask:         │
│  - deploy nginx-1 → vm1     │
│  - deploy nginx-2 → vm2     │
│  - deploy nginx-3 → local   │
│  (stores in database)       │
└─────────────────────────────┘
          ↑    ↑    ↑
          │    │    │  Agents poll via HTTPS
          │    │    │
    ┌─────┘    │    └─────┐
    │          │          │
    ▼          ▼          ▼
┌────────┐ ┌────────┐ ┌────────┐
│ Agent  │ │ Agent  │ │ Agent  │
│  vm1   │ │  vm2   │ │ local  │
│        │ │        │ │        │
│ Gets   │ │ Gets   │ │ Gets   │
│ task   │ │ task   │ │ task   │
│   ↓    │ │   ↓    │ │   ↓    │
│ Docker │ │ Docker │ │ Docker │
│ local  │ │ local  │ │ local  │
└────────┘ └────────┘ └────────┘
```

## Code Changes Needed

### 1. New Model (models/agent_task.go)
```go
type AgentTask struct {
    ID       string
    TaskType string  // "deploy", "delete", "stop", "start"
    Status   string  // "pending", "running", "completed", "failed"
    AgentID  string  // Which agent should handle this
    Payload  json.RawMessage  // Container spec, etc.
}
```

### 2. Agent Enhancement (agent/task_executor.go)
```go
// Agent polls server every 5 seconds
func (e *TaskExecutor) Start(ctx context.Context) {
    for {
        tasks := e.fetchPendingTasks()  // GET /api/v1/agents/{id}/tasks
        for _, task := range tasks {
            e.executeTask(task)  // Create/delete container locally
            e.reportStatus(task) // PUT /api/v1/tasks/{id}/status
        }
        time.Sleep(5 * time.Second)
    }
}
```

### 3. Server Refactor (internal/web/handlers_stacks.go)
```go
// BEFORE
func (h *Handler) DeployStack(c echo.Context) error {
    // Try to connect to Docker on all hosts
    cli := client.NewClientWithOpts(client.WithHost("tcp://vm1:2375")) // FAILS
    cli.ContainerCreate(...)  // Never executes
}

// AFTER
func (h *Handler) DeployStack(c echo.Context) error {
    // Create tasks for agents to execute
    for _, container := range stack.Containers {
        task := &models.AgentTask{
            TaskType: "deploy",
            AgentID:  container.HostID,
            Payload:  container.Spec,
        }
        h.storage.CreateTask(task)  // Agents will pick it up
    }
}
```

## Implementation Order

### Phase 1 (Week 1): Task System
- [ ] Create `models/agent_task.go`
- [ ] Create `internal/storage/tasks.go` (CRUD for tasks)
- [ ] Add API endpoint: `GET /api/v1/agents/{id}/tasks`
- [ ] Add API endpoint: `PUT /api/v1/tasks/{id}/status`

### Phase 2 (Week 2): Agent Deployment
- [ ] Create `agent/deployer.go` (container create/delete functions)
- [ ] Create `agent/task_executor.go` (polling logic)
- [ ] Update agent command to start task executor
- [ ] Test: Deploy single container via agent task

### Phase 3 (Week 3): Stack Refactor
- [ ] Update `DeployStack()` to create tasks
- [ ] Update `DeleteStack()` to create delete tasks
- [ ] Update `StopStack()` to create stop tasks
- [ ] Remove direct Docker connection code

### Phase 4 (Week 4): UI & Polish
- [ ] Show deployment progress in UI
- [ ] Display task status per container
- [ ] Add error reporting
- [ ] End-to-end testing with 3 hosts

## Why This Is Better

| Aspect | Current (Push) | Proposed (Pull) |
|--------|---------------|-----------------|
| **Security** | ❌ Requires exposed Docker ports | ✅ No exposed ports needed |
| **Networking** | ❌ Server must reach all hosts | ✅ Agents reach server (easier) |
| **Scalability** | ❌ Server manages all connections | ✅ Agents work independently |
| **Reliability** | ❌ Network issues break deployment | ✅ Tasks persist, agents retry |
| **Adding Hosts** | ❌ Complex networking setup | ✅ Just start an agent |
| **Industry Standard** | ❌ Old push model | ✅ Matches Kubernetes/Nomad |

## Quick Win: Try It Now

### Step 1: Verify the problem
```bash
# Try to deploy your nginx stack
# Watch it fail on vm1 and vm2
./graphium-dev server &
# Upload nginx-multihost-stack.json via UI
# Result: Only nginx-1 deploys to localhost
```

### Step 2: Check logs
```bash
# You'll see errors like:
# ERROR: Failed to connect to tcp://192.168.122.11:2375
# ERROR: Failed to connect to tcp://192.168.122.12:2375
```

### Step 3: Check current code
```bash
# See the problematic line:
grep -n "tcp://" internal/web/handlers_stacks.go
# Line 112: dockerSocket := fmt.Sprintf("tcp://%s:2375", host.IPAddress)
```

## Decision Time

### Option A: Full Refactor (Recommended)
- **Time:** 2-3 weeks
- **Effort:** Medium
- **Benefit:** Fixes everything, future-proof
- **Risk:** Low (can test incrementally)

### Option B: Quick SSH Fix (Not Recommended)
- **Time:** 1-2 days
- **Effort:** Low
- **Benefit:** Might work short-term
- **Risk:** High (SSH keys, security, doesn't scale)

### Option C: Do Nothing
- **Time:** 0
- **Effort:** 0
- **Benefit:** Multi-host remains broken
- **Risk:** Technical debt accumulates

## Recommendation

**Choose Option A: Full Refactor**

This is the right architectural pattern that will:
1. Fix deployment issues
2. Fix deletion issues
3. Enable future features (rollbacks, health checks, etc.)
4. Match industry best practices
5. Make the system production-ready

---

**Ready to implement?** See `ARCHITECTURE_REFACTOR.md` for detailed implementation plan.

**Questions?** Ask about:
- Polling vs WebSocket
- Task priorities
- Retry strategies
- Rollback capabilities
