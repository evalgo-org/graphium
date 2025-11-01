# Phase 3: Stack Deployment Integration Guide

## Overview

This document describes how to integrate the new task-based deployment system with the existing stack deployment handlers.

## Status

✅ Task queue system complete
✅ Agent deployment capabilities complete
✅ Compilation successful
✅ Helper functions created (`internal/web/stack_tasks.go`)
📝 Handlers need refactoring (see below)

## Integration Steps

### Step 1: Refactor DeployStack Handler

**File:** `internal/web/handlers_stacks.go`

**Current Flow:**
```go
func (h *Handler) DeployStack(c echo.Context) error {
    // 1. Parse JSON-LD stack definition
    // 2. Create stack entity
    // 3. Call deployer.Deploy() - DIRECT DOCKER CALLS
    // 4. Return result
}
```

**New Flow:**
```go
func (h *Handler) DeployStack(c echo.Context) error {
    // 1. Parse JSON-LD stack definition
    // 2. Create stack entity with status="deploying"
    // 3. Create agent tasks for each container
    // 4. Return immediately with stack ID
    // 5. Agents poll and execute tasks asynchronously
    // 6. WebSocket updates notify UI of progress
}
```

**Example Implementation:**

```go
func (h *Handler) DeployStack(c echo.Context) error {
    ctx := c.Request().Context()

    // Get user
    var username string = "web-user"
    if claims, ok := c.Get("claims").(*auth.Claims); ok {
        username = claims.Username
    }

    // Parse stack JSON
    stackJSON := c.FormValue("definition")
    var definition models.StackDefinition
    if err := json.Unmarshal([]byte(stackJSON), &definition); err != nil {
        return Render(c, DeployStackFormWithUser(..., "Invalid JSON", user))
    }

    // Parse stack definition
    parsed, err := ParseStackDefinition(&definition)
    if err != nil {
        return Render(c, DeployStackFormWithUser(..., "Parse error", user))
    }

    // Create stack entity
    stack := &models.Stack{
        ID:          fmt.Sprintf("stack-%s-%d", parsed.StackNode.Name, time.Now().Unix()),
        Name:        parsed.StackNode.Name,
        Description: parsed.StackNode.Description,
        Status:      "deploying",
        Deployment: models.DeploymentConfig{
            Mode:              "multi-host",
            PlacementStrategy: "manual",
            NetworkMode:       "host-port",
        },
        CreatedAt: time.Now(),
        Owner:     username,
    }

    // Save stack
    if err := h.storage.SaveStack(stack); err != nil {
        return c.String(http.StatusInternalServerError, "Failed to create stack")
    }

    // Create deployment state
    deploymentState := &models.DeploymentState{
        ID:         stack.ID,
        StackID:    stack.ID,
        Status:     "deploying",
        Phase:      "creating-tasks",
        Progress:   0,
        Placements: make(map[string]*models.ContainerPlacement),
        StartedAt:  time.Now(),
    }

    if err := h.storage.SaveDeploymentState(deploymentState); err != nil {
        return c.String(http.StatusInternalServerError, "Failed to create deployment state")
    }

    // Create agent tasks for each container
    tasks, err := h.CreateDeploymentTasksForStack(stack.ID, parsed.ContainerSpecs, username)
    if err != nil {
        stack.Status = "error"
        stack.ErrorMessage = fmt.Sprintf("Failed to create deployment tasks: %v", err)
        h.storage.UpdateStack(stack)
        return c.String(http.StatusInternalServerError, stack.ErrorMessage)
    }

    // Update deployment state
    deploymentState.Phase = "waiting-for-agents"
    deploymentState.Progress = 10
    h.storage.UpdateDeploymentState(deploymentState)

    // Broadcast WebSocket event
    h.broadcaster.BroadcastGraphEvent("stack_deploying", map[string]interface{}{
        "stackId":    stack.ID,
        "totalTasks": len(tasks),
    })

    // Redirect to stack detail page (which will show deployment progress)
    return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/web/stacks/%s", stack.ID))
}
```

### Step 2: Refactor DeleteStack Handler

**Current Flow:**
```go
func (h *Handler) DeleteStack(c echo.Context) error {
    // 1. Get stack
    // 2. Call deployer.Remove() - DIRECT DOCKER CALLS
    // 3. Delete from database
}
```

**New Flow:**
```go
func (h *Handler) DeleteStack(c echo.Context) error {
    // 1. Get stack and deployment state
    // 2. Create deletion tasks for each container
    // 3. Update stack status to "deleting"
    // 4. Return immediately
    // 5. Agents execute deletion tasks
    // 6. When all tasks complete, cleanup stack metadata
}
```

**Example Implementation:**

```go
func (h *Handler) DeleteStack(c echo.Context) error {
    ctx := c.Request().Context()
    id := c.Param("id")

    // Get user
    var username string = "web-user"
    if claims, ok := c.Get("claims").(*auth.Claims); ok {
        username = claims.Username
    }

    // Get stack
    stack, err := h.storage.GetStack(id)
    if err != nil {
        return c.String(http.StatusNotFound, "Stack not found")
    }

    // Get deployment state
    deploymentState, err := h.storage.GetDeploymentState(id)
    if err != nil {
        // Stack not deployed, just delete metadata
        if err := h.storage.DeleteStack(id, stack.Rev); err != nil {
            return c.String(http.StatusInternalServerError, "Failed to delete stack")
        }
        return c.Redirect(http.StatusSeeOther, "/web/stacks")
    }

    // Update stack status
    stack.Status = "deleting"
    h.storage.UpdateStack(stack)

    // Create deletion tasks
    tasks, err := h.CreateDeletionTasksForStack(id, deploymentState, username)
    if err != nil {
        return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to create deletion tasks: %v", err))
    }

    // Broadcast WebSocket event
    h.broadcaster.BroadcastGraphEvent("stack_deleting", map[string]interface{}{
        "stackId":    id,
        "totalTasks": len(tasks),
    })

    // Redirect to stacks list
    return c.Redirect(http.StatusSeeOther, "/web/stacks")
}
```

### Step 3: Add Background Task Monitor

Create a goroutine that watches for completed deletion tasks and cleans up stack metadata:

```go
// File: internal/api/server.go

func (s *Server) startTaskMonitor(ctx context.Context) {
    go func() {
        ticker := time.NewTicker(10 * time.Second)
        defer ticker.Stop()

        for {
            select {
            case <-ticker.C:
                s.checkCompletedStackDeletions()
            case <-ctx.Done():
                return
            }
        }
    }()
}

func (s *Server) checkCompletedStackDeletions() {
    // Get all stacks with status="deleting"
    stacks, err := s.storage.ListStacks(map[string]interface{}{
        "status": "deleting",
    })
    if err != nil {
        return
    }

    for _, stack := range stacks {
        // Get tasks for this stack
        tasks, err := s.storage.GetTasksByStack(stack.ID)
        if err != nil {
            continue
        }

        // Check if all tasks are complete
        allComplete := true
        for _, task := range tasks {
            if task.Status != "completed" && task.Status != "failed" && task.Status != "cancelled" {
                allComplete = false
                break
            }
        }

        if allComplete {
            // Delete stack metadata
            s.storage.DeleteStack(stack.ID, stack.Rev)
            s.storage.DeleteDeploymentState(stack.ID)

            // Broadcast event
            s.BroadcastGraphEvent("stack_deleted", map[string]interface{}{
                "stackId": stack.ID,
            })
        }
    }
}
```

## Testing Plan

### Manual Testing

1. **Start the server:**
   ```bash
   ./graphium-dev server
   ```

2. **Start agents on all hosts:**
   ```bash
   # localhost
   TOKEN="your-agent-token" ./graphium-dev agent --host-id localhost-docker

   # vm1 (SSH to vm1 first)
   TOKEN="your-agent-token" ./graphium-dev agent --host-id vm1

   # vm2 (SSH to vm2 first)
   TOKEN="your-agent-token" ./graphium-dev agent --host-id vm2
   ```

3. **Deploy the nginx stack:**
   - Open http://localhost:8095/web/stacks
   - Click "Deploy Stack"
   - Paste contents of `nginx-multihost-stack.json`
   - Click "Deploy"

4. **Watch the deployment:**
   - Check server logs for task creation
   - Check agent logs for task execution
   - Open browser console to see WebSocket events
   - Refresh stack detail page to see progress

5. **Verify containers:**
   ```bash
   # On localhost
   docker ps | grep nginx
   curl http://localhost:8081

   # On vm1
   docker ps | grep nginx
   curl http://localhost:8082

   # On vm2
   docker ps | grep nginx
   curl http://localhost:8083
   ```

6. **Delete the stack:**
   - Click "Delete Stack" button
   - Watch agents remove containers
   - Verify all containers are removed

### API Testing

```bash
# Create a deployment task manually
curl -X POST http://localhost:8095/api/v1/tasks \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d @- << 'EOF'
{
  "@type": "AgentTask",
  "taskType": "deploy",
  "status": "pending",
  "agentId": "localhost-docker",
  "hostId": "localhost-docker",
  "priority": 5,
  "payload": {
    "containerSpec": {
      "name": "test-nginx",
      "image": "nginx:alpine",
      "ports": [{"containerPort": 80, "hostPort": 8888}],
      "restartPolicy": "unless-stopped"
    },
    "pullPolicy": "if-not-present"
  }
}
EOF

# List tasks for agent
curl http://localhost:8095/api/v1/agents/localhost-docker/tasks \
  -H "Authorization: Bearer $AGENT_TOKEN"

# Get task statistics
curl http://localhost:8095/api/v1/tasks/stats \
  -H "Authorization: Bearer $TOKEN"
```

## Known Limitations

1. **No automatic cleanup** - Stack metadata isn't automatically deleted when deletion tasks complete (need Step 3)
2. **No rollback** - If deployment fails, no automatic rollback (future feature)
3. **No health checks** - Tasks don't verify container health after deployment
4. **In-memory sorting** - Task priority sorting done in memory (consider CouchDB views)
5. **No dependency orchestration** - Container dependencies not enforced yet

## Next Steps

1. Implement handlers with new task-based flow
2. Add task monitor for automatic cleanup
3. Test with real nginx stack
4. Add UI progress indicators
5. Document deployment workflow

## Files Modified

- ✅ `models/agent_task.go` - Task models
- ✅ `internal/storage/tasks.go` - Task CRUD
- ✅ `internal/api/handlers_agent_tasks.go` - Task API
- ✅ `agent/deployer.go` - Container operations
- ✅ `agent/task_executor.go` - Task execution
- ✅ `internal/web/stack_tasks.go` - Helper functions
- 📝 `internal/web/handlers_stacks.go` - Needs refactoring
- 📝 `internal/api/server.go` - Needs task monitor

## Estimated Time to Complete

- Handler refactoring: 1-2 hours
- Task monitor: 30 minutes
- Testing: 1-2 hours
- Bug fixes: 1 hour
- **Total: 3-5 hours**

## Success Criteria

- [ ] Stack deployment creates tasks instead of direct Docker calls
- [ ] Agents pick up and execute tasks
- [ ] Containers deploy to correct hosts
- [ ] Stack deletion removes containers from all hosts
- [ ] WebSocket events update UI in real-time
- [ ] Task statistics visible in API
- [ ] nginx-multihost-stack.json deploys successfully
