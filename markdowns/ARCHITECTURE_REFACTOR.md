# Graphium Architecture Refactor: Agent-Based Deployment

## Problem Statement

### Current Architecture Issues

#### Issue 1: Stack Deletion Doesn't Remove Remote Containers
**Current Behavior:**
- When deleting a stack, the server tries to connect directly to Docker on all hosts
- Server uses `WebDockerClientFactory` which creates Docker clients via:
  - `tcp://<host-ip>:2375` for remote hosts (vm1, vm2)
  - `unix:///var/run/docker.sock` for localhost

**Why It Fails:**
```
handlers_stacks.go:112 → dockerSocket := fmt.Sprintf("tcp://%s:2375", host.IPAddress)
```
- vm1 and vm2 don't expose Docker on TCP port 2375 (security risk)
- Server cannot reach Docker daemons on remote hosts
- Containers remain orphaned after stack deletion

#### Issue 2: Stack Deployment Cannot Reach Remote Hosts
**Current Behavior:**
- Server tries to deploy containers directly to remote hosts
- Uses same flawed connection method
- Only localhost-docker deployments work

**Root Cause:**
The system uses a **PUSH model** where the server pushes commands to all hosts:
```
Web UI → Server → Direct Docker API → All Hosts
```

This requires:
- Network access from server to all Docker daemons
- Exposed Docker ports (security risk)
- Complex networking/firewall rules

### Current Agent Capabilities
Agents currently only:
- ✅ Monitor/sync containers from local Docker
- ✅ Report container states to server
- ❌ Cannot create containers
- ❌ Cannot delete containers
- ❌ Cannot execute deployments

## Proposed Solution: Agent-Based Deployment

### New Architecture: Pull Model

Switch to a **PULL model** where agents pull work from the server:

```
Web UI → Server → Store Deployment Task in DB
                         ↓
Agent Polls Server → Gets Assigned Tasks → Executes Locally → Reports Status
```

### Architecture Comparison

#### Current (Push - Broken)
```
┌─────────────┐
│   Server    │
│             │──── Direct Docker API (tcp://vm1:2375) ✗ FAILS
│  Deployment │──── Direct Docker API (tcp://vm2:2375) ✗ FAILS
│   Engine    │──── Direct Docker API (unix://localhost) ✓ WORKS
└─────────────┘

┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  localhost  │     │     vm1     │     │     vm2     │
│   Agent     │     │   Agent     │     │   Agent     │
│ (read-only) │     │ (read-only) │     │ (read-only) │
└─────────────┘     └─────────────┘     └─────────────┘
```

#### Proposed (Pull - Robust)
```
┌─────────────┐
│   Server    │
│             │
│  Deployment │──── Stores Tasks in Database
│   Engine    │──── Assigns Tasks to Agents
└─────────────┘
       ↑ ↑ ↑
       │ │ │ (Agents poll for tasks via HTTPS API)
       │ │ │
┌──────┘ │ └──────┐
│        │        │
▼        ▼        ▼
┌─────────────┐ ┌─────────────┐ ┌─────────────┐
│  localhost  │ │     vm1     │ │     vm2     │
│   Agent     │ │   Agent     │ │   Agent     │
│   (R/W)     │ │   (R/W)     │ │   (R/W)     │
│             │ │             │ │             │
│ - Monitor   │ │ - Monitor   │ │ - Monitor   │
│ - Deploy    │ │ - Deploy    │ │ - Deploy    │
│ - Delete    │ │ - Delete    │ │ - Delete    │
└─────────────┘ └─────────────┘ └─────────────┘
     ↓               ↓               ↓
  Docker          Docker          Docker
```

## Implementation Plan

### Phase 1: Task Queue System

#### 1.1 Create Task Model
**File:** `models/agent_task.go`

```go
type AgentTask struct {
    ID          string    `json:"@id" couchdb:"_id"`
    Rev         string    `json:"_rev,omitempty" couchdb:"_rev"`
    Type        string    `json:"@type"`          // "AgentTask"
    TaskType    string    `json:"taskType"`       // "deploy", "delete", "stop", "start"
    Status      string    `json:"status"`         // "pending", "assigned", "running", "completed", "failed"
    AgentID     string    `json:"agentId"`        // Target agent
    HostID      string    `json:"hostId"`         // Target host
    StackID     string    `json:"stackId,omitempty"`
    ContainerID string    `json:"containerId,omitempty"`
    Payload     json.RawMessage `json:"payload"`  // Task-specific data
    CreatedAt   time.Time `json:"dateCreated"`
    AssignedAt  *time.Time `json:"assignedAt,omitempty"`
    CompletedAt *time.Time `json:"completedAt,omitempty"`
    Error       string    `json:"error,omitempty"`
}

// Payload examples:
type DeployContainerPayload struct {
    ContainerSpec models.ContainerSpec `json:"containerSpec"`
    NetworkConfig *models.NetworkSpec  `json:"networkConfig,omitempty"`
}

type DeleteContainerPayload struct {
    ContainerID string `json:"containerId"`
    Force       bool   `json:"force"`
    RemoveVolumes bool `json:"removeVolumes"`
}
```

#### 1.2 Task Queue Storage
**File:** `internal/storage/tasks.go`

```go
// CreateTask creates a new agent task
func (s *Storage) CreateTask(task *models.AgentTask) error

// GetTasksByAgent retrieves pending tasks for an agent
func (s *Storage) GetTasksByAgent(agentID string, status string) ([]*models.AgentTask, error)

// UpdateTaskStatus updates task status
func (s *Storage) UpdateTaskStatus(taskID string, status string, errorMsg string) error

// GetTasksByStack retrieves all tasks for a stack
func (s *Storage) GetTasksByStack(stackID string) ([]*models.AgentTask, error)
```

### Phase 2: Agent Deployment Capabilities

#### 2.1 Enhance Agent with Deployment Functions
**File:** `agent/deployer.go` (new)

```go
type AgentDeployer struct {
    dockerClient *docker.Client
    hostID       string
    agentID      string
}

// DeployContainer creates and starts a container from spec
func (d *AgentDeployer) DeployContainer(ctx context.Context, spec *models.ContainerSpec) error

// DeleteContainer removes a container
func (d *AgentDeployer) DeleteContainer(ctx context.Context, containerID string, force bool) error

// StopContainer stops a running container
func (d *AgentDeployer) StopContainer(ctx context.Context, containerID string) error

// StartContainer starts a stopped container
func (d *AgentDeployer) StartContainer(ctx context.Context, containerID string) error
```

#### 2.2 Agent Task Polling
**File:** `agent/task_executor.go` (new)

```go
type TaskExecutor struct {
    client       *client.AgentClient
    deployer     *AgentDeployer
    pollInterval time.Duration
}

// Start begins polling for tasks
func (e *TaskExecutor) Start(ctx context.Context) error {
    ticker := time.NewTicker(e.pollInterval)
    for {
        select {
        case <-ticker.C:
            e.pollAndExecuteTasks(ctx)
        case <-ctx.Done():
            return nil
        }
    }
}

// pollAndExecuteTasks fetches and executes pending tasks
func (e *TaskExecutor) pollAndExecuteTasks(ctx context.Context) {
    // 1. Call server API: GET /api/v1/agents/{agentID}/tasks?status=pending
    // 2. For each task, execute appropriate action
    // 3. Report status back to server
}
```

### Phase 3: Server API Endpoints

#### 3.1 Agent Task API
**File:** `internal/api/handlers_agent_tasks.go` (new)

```go
// GetAgentTasks returns pending tasks for an agent
// GET /api/v1/agents/:id/tasks
func (s *Server) getAgentTasks(c echo.Context) error

// UpdateTaskStatus updates task status (called by agent)
// PUT /api/v1/tasks/:id/status
func (s *Server) updateTaskStatus(c echo.Context) error
```

#### 3.2 Refactor Stack Deployment
**File:** `internal/web/handlers_stacks.go`

```go
// DeployStack - NEW VERSION
func (h *Handler) DeployStack(c echo.Context) error {
    // 1. Parse stack definition
    // 2. Create deployment state
    // 3. For each container, create AgentTask with type="deploy"
    // 4. Assign tasks to appropriate agents based on host
    // 5. Return immediately (async deployment)
    // 6. Agents will poll and execute
}

// DeleteStack - NEW VERSION
func (h *Handler) DeleteStack(c echo.Context) error {
    // 1. Get deployment state
    // 2. For each container, create AgentTask with type="delete"
    // 3. Assign tasks to appropriate agents
    // 4. Delete stack metadata
    // 5. Agents will clean up containers
}
```

### Phase 4: Deployment Status Tracking

#### 4.1 Real-time Status Updates
- Agents report task progress via API
- Server updates deployment state in database
- WebSocket broadcasts deployment events to UI
- Dashboard shows live deployment progress

#### 4.2 UI Enhancements
- Show deployment progress (0-100%)
- Display per-container deployment status
- Show which agent is handling which task
- Error reporting per container

## Benefits of Agent-Based Architecture

### 1. Security
- ✅ No exposed Docker ports on remote hosts
- ✅ Agents authenticate to server, not vice versa
- ✅ Docker access stays local to each host
- ✅ TLS/JWT authentication for agent API calls

### 2. Reliability
- ✅ Agents can retry failed operations
- ✅ Tasks persist in database if agent is offline
- ✅ Agents resume work after restart
- ✅ No network connectivity issues from server to hosts

### 3. Scalability
- ✅ Easy to add new hosts (just start an agent)
- ✅ No server-side connection pool management
- ✅ Agents work independently
- ✅ Horizontal scaling with multiple agents per datacenter

### 4. Observability
- ✅ Task history in database
- ✅ Clear audit trail of who deployed what
- ✅ Agent health monitoring
- ✅ Deployment error tracking

### 5. Consistency with Industry Standards
This matches how modern orchestrators work:
- **Kubernetes**: kubelet pulls workloads, doesn't get pushed
- **Nomad**: client pulls allocations from server
- **Docker Swarm**: agents pull service definitions

## Migration Strategy

### Phase 1: Parallel Systems (Safe)
1. Keep existing direct Docker deployment for localhost
2. Add agent-based deployment for remote hosts only
3. Test with non-production stacks

### Phase 2: Full Migration
1. All deployments go through agent system
2. Deprecate direct Docker access
3. Remove `WebDockerClientFactory` direct connection code

### Phase 3: Advanced Features
1. Add deployment rollback capability
2. Add blue-green deployments
3. Add canary deployments
4. Add health checks during deployment

## Implementation Timeline

### Week 1: Foundation
- [ ] Create AgentTask model
- [ ] Implement task storage layer
- [ ] Add agent task API endpoints

### Week 2: Agent Enhancement
- [ ] Implement agent deployer
- [ ] Implement task executor and polling
- [ ] Add task reporting

### Week 3: Server Refactor
- [ ] Refactor stack deployment to use tasks
- [ ] Refactor stack deletion to use tasks
- [ ] Update deployment state tracking

### Week 4: Testing & UI
- [ ] End-to-end testing with 3-host setup
- [ ] UI progress indicators
- [ ] Error handling and retry logic
- [ ] Documentation

## Alternative: Quick Fix (Not Recommended)

### Option: SSH Tunneling
Instead of full refactor, use SSH tunneling:
```go
// Use EVE's SSH tunnel support
dockerSocket := fmt.Sprintf("ssh://user@%s", host.IPAddress)
```

**Pros:**
- Faster to implement
- Uses existing code

**Cons:**
- ❌ Requires SSH access from server to all hosts
- ❌ Requires SSH key management
- ❌ Still server-initiated (push model)
- ❌ Doesn't scale well
- ❌ Complex error handling

## Recommendation

**Implement the agent-based deployment architecture.**

**Rationale:**
1. Fixes both deployment and deletion issues
2. More secure (no exposed Docker ports or SSH)
3. More scalable (agents are independent)
4. Industry-standard approach
5. Better foundation for future features

**Estimated Effort:** 2-3 weeks for core functionality

**Risk:** Low - can be implemented incrementally alongside existing system

## Next Steps

1. **Review & Approve:** Discuss this architecture with team
2. **Prototype:** Build minimal task system with one agent
3. **Test:** Deploy single container via agent task
4. **Iterate:** Expand to full stack deployment
5. **Migrate:** Switch all deployments to agent-based system

## Questions for Discussion

1. **Polling Interval:** How often should agents poll for tasks? (Suggested: 5-10 seconds)
2. **Task Timeout:** How long before a task is considered failed? (Suggested: 5 minutes)
3. **Retry Logic:** How many times should agents retry failed tasks? (Suggested: 3)
4. **WebSocket Option:** Should we add WebSocket push instead of polling?
5. **Priority:** Should some tasks (e.g., delete) have higher priority?

---

**Author:** Claude + User
**Date:** 2025-10-31
**Status:** Proposal - Awaiting Review
